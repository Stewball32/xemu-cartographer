package containers

// HTTP reverse-proxy that fronts the jlesage/firefox kiosk container's web UI.
//
// Why a proxy instead of direct port access:
//   - The kiosk's HTTP port (browser_web) and VNC port (browser_vnc) are bound
//     to 127.0.0.1 only (see internal/podman/podman.go:createBrowser). Direct
//     access from a browser doesn't work over the internet.
//   - All traffic flows through PocketBase's :8090, which is the single port
//     that needs to be public when deploying behind a TLS reverse-proxy.
//   - Auth is enforced via the same PocketBase JWT used everywhere else.
//
// The HTML base-href injection rewrites jlesage's noVNC entry-point so that
// its relative asset paths (`app/`, `core/`, `vendor/`) resolve under the
// proxied prefix. The /websockify upgrade is handled transparently by
// httputil.ReverseProxy — its built-in switching protocols path supports
// WebSockets as long as the upstream URL is preserved.

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerKioskProxy)
}

// Kiosk dial-retry tuning, read once at register time (OnServe, after any .env
// is in the process environment). kioskDialBudget bounds the total time the
// reverse proxy spends retrying the TCP dial to a *running-but-booting* browser
// container (nginx/s6-overlay take a few seconds to accept connections after
// `podman start`). kioskPerDialTimeout bounds each individual attempt so a
// filtered/silently-dropped port can't stall a single dial past the budget.
// Defaults preserve the historical ~10s boot-race window.
var (
	kioskDialBudget     = 10 * time.Second
	kioskPerDialTimeout = 3 * time.Second
)

func registerKioskProxy() {
	// Mounted directly on se.Router (NOT on Group) so we can authenticate
	// via ?token= rather than the Authorization header — iframes cannot
	// set headers on their own requests.
	if Router == nil {
		return
	}

	if d := envDurationMS("CONTAINERS_KIOSK_DIAL_TIMEOUT_MS"); d > 0 {
		kioskDialBudget = d
	}
	if d := envDurationMS("CONTAINERS_KIOSK_PER_DIAL_TIMEOUT_MS"); d > 0 {
		kioskPerDialTimeout = d
	}

	Router.GET("/api/admin/containers/{name}/kiosk/{path...}", handleKioskProxy)
	Router.HEAD("/api/admin/containers/{name}/kiosk/{path...}", handleKioskProxy)
	Router.POST("/api/admin/containers/{name}/kiosk/{path...}", handleKioskProxy)

	// Bare `/kiosk` (no trailing slash) — redirect to `/kiosk/` so the
	// browser sees a stable origin path and the base-href works.
	Router.GET("/api/admin/containers/{name}/kiosk", func(e *core.RequestEvent) error {
		name := e.Request.PathValue("name")
		http.Redirect(e.Response, e.Request,
			"/api/admin/containers/"+url.PathEscape(name)+"/kiosk/",
			http.StatusFound)
		return nil
	})
}

func handleKioskProxy(e *core.RequestEvent) error {
	name := e.Request.PathValue("name")
	// M09: admins reach any container's kiosk; a non-admin reaches only the
	// container their gamertag is currently rostered in.
	if !authorizeKioskAccess(e, name) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	info, ok := Manager.Get(name)
	if !ok {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "container not found"})
	}

	// Gate on live podman status before entering the dial-retry loop. A
	// container that is *recorded* but *not running* passes the Get() existence
	// check (existence ≠ liveness); without this gate it would get dialed on a
	// dead port and hang the full ~10s dial-retry budget before surfacing as a
	// 502. KioskLive fast-fails that case with a clean 503 the browser can
	// render immediately; a running-but-still-booting container reads as live
	// and the dial retry below covers its nginx warm-up.
	if !Manager.KioskLive(name) {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "container not running"})
	}

	// When the request brought a fresh ?token=, persist it as a path-scoped
	// HttpOnly cookie so the iframe's sub-resource fetches (CSS/JS/images/
	// /websockify) authenticate without anyone rewriting URLs.
	if t := e.Request.URL.Query().Get("token"); t != "" {
		setKioskTokenCookie(e, "/api/admin/containers/"+url.PathEscape(name)+"/kiosk/", t)
	}

	// PB's default security headers middleware sets X-Frame-Options: SAMEORIGIN
	// on every response. That blocks the iframe whenever the embedding page is
	// on a different origin (dev: Vite :5173 → PB :8090). Strip it here — auth
	// is enforced via the ?token= JWT, not by frame-ancestors. Same trick PB
	// itself uses for file routes (see apis/file.go).
	e.Response.Header().Del("X-Frame-Options")

	target := &url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + strconv.Itoa(info.Ports.BrowserWeb),
	}
	prefix := "/api/admin/containers/" + url.PathEscape(name) + "/kiosk"

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			// Strip the prefix so the upstream sees the original noVNC paths.
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.Host = target.Host
		},
		// The browser container's HTTP listener (s6-overlay → nginx) takes
		// several seconds to come up after `podman start`. Without a retry,
		// the user's first iframe load races that boot and gets a 502.
		Transport: &http.Transport{
			DialContext: newDialWithRetry(kioskDialBudget, kioskPerDialTimeout),
		},
		ModifyResponse: func(resp *http.Response) error {
			// Drop framing restrictions from upstream too — same reason as
			// the PB-default header strip above.
			resp.Header.Del("X-Frame-Options")
			resp.Header.Del("Content-Security-Policy")

			ct := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "text/html") {
				return nil
			}
			return rewriteKioskHTML(resp, prefix+"/")
		},
	}

	proxy.ServeHTTP(e.Response, e.Request)
	return nil
}

// newDialWithRetry builds a DialContext that retries a TCP dial for up to
// `budget` when the upstream refuses the connection — covering the gap between
// `podman start` returning and the browser container's HTTP listener actually
// accepting connections. Each individual attempt is bounded by `perDial` so a
// filtered (silently-dropped) port can't stall a single dial past the budget.
// Once the listener is up, the first attempt succeeds with no measurable
// overhead.
//
// The liveness gate in handleKioskProxy means we only reach here for a
// container podman reports as running, so this retry now only ever smooths a
// genuine boot race — never a recorded-but-dead container.
func newDialWithRetry(budget, perDial time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		deadline := time.Now().Add(budget)
		d := net.Dialer{Timeout: perDial}
		for {
			conn, err := d.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}
			if ctx.Err() != nil || time.Now().After(deadline) {
				return nil, err
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
}

// envDurationMS reads an integer-milliseconds env var into a time.Duration,
// returning 0 when unset/empty/invalid so callers keep their default.
func envDurationMS(key string) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return time.Duration(n) * time.Millisecond
}

// permissionsPolyfillScript wraps navigator.permissions.query so Firefox stops
// throwing TypeError on unknown permission names like 'clipboard-write' (a
// Chromium-only enum). noVNC's RFB constructor probes that name and the
// rejection floods the kiosk console on every load. Returning a denied
// PermissionStatus-shaped object matches Chromium's behavior. Chromium itself
// is a no-op since it doesn't throw on the same query.
const permissionsPolyfillScript = `<script>(function(){var p=navigator.permissions;if(!p||!p.query)return;var orig=p.query.bind(p);p.query=function(d){try{return orig(d).catch(function(e){if(e&&e.name==='TypeError')return{state:'denied',onchange:null};throw e;});}catch(e){if(e&&e.name==='TypeError')return Promise.resolve({state:'denied',onchange:null});return Promise.reject(e);}};})();</script>`

// rewriteKioskHTML splices a <base href> tag and the permissions polyfill into
// the kiosk's HTML <head>. The base href makes the upstream's relative asset
// paths resolve under our proxy prefix; the polyfill silences a Firefox-only
// noVNC console error. Handles gzip transparently and rewrites Content-Length.
func rewriteKioskHTML(resp *http.Response, base string) error {
	var (
		body []byte
		err  error
	)

	gzipped := strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip")
	if gzipped {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("kiosk proxy: gzip reader: %w", err)
		}
		body, err = io.ReadAll(gr)
		_ = gr.Close()
		if err != nil {
			return fmt.Errorf("kiosk proxy: gzip read: %w", err)
		}
	} else {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("kiosk proxy: read body: %w", err)
		}
	}
	_ = resp.Body.Close()

	injection := []byte(`<base href="` + base + `">` + permissionsPolyfillScript)
	if i := bytes.Index(bytes.ToLower(body), []byte("<head>")); i != -1 {
		insert := i + len("<head>")
		body = append(body[:insert], append(injection, body[insert:]...)...)
	} else if i := bytes.Index(bytes.ToLower(body), []byte("<html")); i != -1 {
		// No <head> — splice after the <html ...> opening tag's `>`.
		if end := bytes.IndexByte(body[i:], '>'); end != -1 {
			insert := i + end + 1
			body = append(body[:insert], append([]byte("<head>"+string(injection)+"</head>"), body[insert:]...)...)
		}
	}

	if gzipped {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(body); err != nil {
			return fmt.Errorf("kiosk proxy: gzip write: %w", err)
		}
		if err := gw.Close(); err != nil {
			return fmt.Errorf("kiosk proxy: gzip close: %w", err)
		}
		body = buf.Bytes()
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}
