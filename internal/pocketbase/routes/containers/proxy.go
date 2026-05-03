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
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerKioskProxy)
}

func registerKioskProxy() {
	// Mounted directly on se.Router (NOT on Group) so we can authenticate
	// via ?token= rather than the Authorization header — iframes cannot
	// set headers on their own requests.
	if Router == nil {
		return
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
	if !authorizeAdminQueryToken(e) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	name := e.Request.PathValue("name")
	info, ok := Manager.Get(name)
	if !ok {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "container not found"})
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
			DialContext: dialWithRetry,
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
			return injectBaseHref(resp, prefix+"/")
		},
	}

	proxy.ServeHTTP(e.Response, e.Request)
	return nil
}

// dialWithRetry retries a TCP dial for up to ~10s when the upstream refuses
// the connection — covers the gap between `podman start` returning and the
// browser container's HTTP listener actually accepting connections. Once the
// listener is up, the first attempt succeeds with no measurable overhead.
func dialWithRetry(ctx context.Context, network, addr string) (net.Conn, error) {
	deadline := time.Now().Add(10 * time.Second)
	var d net.Dialer
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

// injectBaseHref rewrites an HTML response's <head> to include a <base href>
// pointing at the proxied prefix, so the kiosk's relative asset paths resolve.
// Handles gzip transparently and rewrites Content-Length on success.
func injectBaseHref(resp *http.Response, base string) error {
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
		gr.Close()
		if err != nil {
			return fmt.Errorf("kiosk proxy: gzip read: %w", err)
		}
	} else {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("kiosk proxy: read body: %w", err)
		}
	}
	resp.Body.Close()

	tag := []byte(`<base href="` + base + `">`)
	if i := bytes.Index(bytes.ToLower(body), []byte("<head>")); i != -1 {
		insert := i + len("<head>")
		body = append(body[:insert], append(tag, body[insert:]...)...)
	} else if i := bytes.Index(bytes.ToLower(body), []byte("<html")); i != -1 {
		// No <head> — splice after the <html ...> opening tag's `>`.
		if end := bytes.IndexByte(body[i:], '>'); end != -1 {
			insert := i + end + 1
			body = append(body[:insert], append([]byte("<head>"+string(tag)+"</head>"), body[insert:]...)...)
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
