package xcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xemu-cartographer/xc-scraper/daemon"
	"github.com/xemu-cartographer/xc-scraper/hosthealth"
	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/runner"
)

// ErrUpstreamDown is returned by every Ctl call while the stream client is
// not connected to the daemon (DESIGN-STEP8 §8.5 fail-fast): the answer comes
// back in well under a millisecond and no HTTP request is made, so a
// downstream request never hangs on a daemon that is known to be away.
var ErrUpstreamDown = errors.New("xcclient: upstream daemon not connected")

// DefaultCtlTimeout is the per-call HTTP timeout (§8.5: 2 s).
const DefaultCtlTimeout = 2 * time.Second

// ctlMaxBody bounds a daemon response body (inspect snapshots are the
// largest at a few hundred KiB).
const ctlMaxBody = 8 << 20

// Error is a daemon error reply {"error":{"code","message"}} together with
// its HTTP status. Bodies that are not in the daemon's shape keep Code ""
// and carry the trimmed body as Message.
type Error struct {
	Status  int
	Code    string
	Message string
	Method  string
	Path    string
}

func (e *Error) Error() string {
	code := e.Code
	if code == "" {
		code = "http_" + strconv.Itoa(e.Status)
	}
	return fmt.Sprintf("xcclient: %s %s: %d %s: %s", e.Method, e.Path, e.Status, code, e.Message)
}

// Is lets errors.Is(err, &Error{Status: 404}) / {Code: "x"} match on the
// fields the target sets (zero fields are wildcards).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return (t.Status == 0 || t.Status == e.Status) && (t.Code == "" || t.Code == e.Code)
}

// Ctl is the daemon's HTTP client (§6.1 read API + §6.2 control API). Base
// is XC_SCRAPER_URL verbatim; Token authenticates the read API (Bearer),
// ControlToken the /api/ctl/* routes. Connected is the fail-fast gate: when
// set and false, every call returns ErrUpstreamDown without touching the
// network. Build one with NewCtl to bind it to a Client.
type Ctl struct {
	Base         string
	Token        string
	ControlToken string
	// Timeout bounds each call (0 = DefaultCtlTimeout).
	Timeout time.Duration
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Connected reports whether the upstream stream is up; nil = no gate.
	Connected func() bool
}

// NewCtl binds a control client to c: same base URL, feed token and HTTP
// client, gated on c.Status().Connected.
func NewCtl(c *Client, controlToken string) *Ctl {
	return &Ctl{
		Base:         c.cfg.URL,
		Token:        c.cfg.Token,
		ControlToken: controlToken,
		HTTPClient:   c.http,
		Connected:    func() bool { return c.Status().Connected },
	}
}

// Do performs one request and returns the status and body verbatim (no
// error for non-2xx) — the primitive the league's pass-through proxies use.
// control selects the bearer: feed token (false) or control token (true).
func (c *Ctl) Do(ctx context.Context, method, path string, body []byte, control bool) (int, []byte, error) {
	if c.Connected != nil && !c.Connected() {
		return 0, nil, ErrUpstreamDown
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultCtlTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.Base, "/")+path, rdr)
	if err != nil {
		return 0, nil, fmt.Errorf("xcclient: %s %s: %w", method, path, err)
	}
	token := c.Token
	if control {
		token = c.ControlToken
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("xcclient: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, ctlMaxBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("xcclient: %s %s: read body: %w", method, path, err)
	}
	return resp.StatusCode, out, nil
}

// call is Do plus the daemon error mapping: non-2xx → *Error; a non-nil
// out is JSON-decoded from a 2xx body.
func (c *Ctl) call(ctx context.Context, method, path string, in, out any) ([]byte, error) {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return nil, fmt.Errorf("xcclient: %s %s: encode body: %w", method, path, err)
		}
	}
	status, raw, err := c.Do(ctx, method, path, body, strings.HasPrefix(path, "/api/ctl/"))
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, decodeError(method, path, status, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return nil, fmt.Errorf("xcclient: %s %s: decode: %w", method, path, err)
		}
	}
	return raw, nil
}

// decodeError turns a non-2xx body into *Error.
func decodeError(method, path string, status int, raw []byte) *Error {
	e := &Error{Status: status, Method: method, Path: path}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &body) == nil && (body.Error.Code != "" || body.Error.Message != "") {
		e.Code, e.Message = body.Error.Code, body.Error.Message
		return e
	}
	e.Message = trimPayload(raw)
	return e
}

// Get decodes a read-API JSON reply into out.
func (c *Ctl) Get(ctx context.Context, path string, out any) error {
	_, err := c.call(ctx, http.MethodGet, path, nil, out)
	return err
}

// GetRaw returns a read-API 2xx body verbatim.
func (c *Ctl) GetRaw(ctx context.Context, path string) ([]byte, error) {
	return c.call(ctx, http.MethodGet, path, nil, nil)
}

// Control sends in (JSON, may be nil) with the control token and decodes
// the 2xx reply into out (may be nil). Non-2xx → *Error.
func (c *Ctl) Control(ctx context.Context, method, path string, in, out any) error {
	_, err := c.call(ctx, method, path, in, out)
	return err
}

func instancePath(name, suffix string) string {
	return "/api/instances/" + url.PathEscape(name) + suffix
}

func ctlInstancePath(name, suffix string) string {
	return "/api/ctl/instances/" + url.PathEscape(name) + suffix
}

// InstanceRow is one GET /api/instances entry (runner.Info + phase, running,
// attach{kind,addr}, error). Aliased here so route packages name the row
// through xcclient only; at R2 (D-15) this is the one place to re-point
// when the daemon's body types move to a leaf package.
type InstanceRow = daemon.InstanceRow

// Instances is GET /api/instances.
func (c *Ctl) Instances(ctx context.Context) ([]InstanceRow, error) {
	var rows []InstanceRow
	err := c.Get(ctx, "/api/instances", &rows)
	return rows, err
}

// Inspect is GET /api/instances/{n}/inspect.
func (c *Ctl) Inspect(ctx context.Context, name string) (runner.InspectState, error) {
	var st runner.InspectState
	err := c.Get(ctx, instancePath(name, "/inspect"), &st)
	return st, err
}

// Events is GET /api/instances/{n}/events: the framed wire.Message bytes
// (room host:<n>) verbatim.
func (c *Ctl) Events(ctx context.Context, name string, sinceTick uint32, types []string) ([]byte, error) {
	q := url.Values{}
	if sinceTick != 0 {
		q.Set("since_tick", strconv.FormatUint(uint64(sinceTick), 10))
	}
	if len(types) != 0 {
		q.Set("types", strings.Join(types, ","))
	}
	path := instancePath(name, "/events")
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return c.GetRaw(ctx, path)
}

// Probe is GET /api/instances/{n}/probe: framed bytes verbatim (504
// probe_timeout when the runner did not answer).
func (c *Ctl) Probe(ctx context.Context, name string) ([]byte, error) {
	return c.GetRaw(ctx, instancePath(name, "/probe"))
}

// Maps is GET /api/instances/{n}/maps.
func (c *Ctl) Maps(ctx context.Context, name string) (runner.MapList, error) {
	var ml runner.MapList
	err := c.Get(ctx, instancePath(name, "/maps"), &ml)
	return ml, err
}

// Readout is GET /api/instances/{n}/readout (404 no_readout before the
// first tick).
func (c *Ctl) Readout(ctx context.Context, name string) (hostrunner.ScraperReadout, error) {
	var ro hostrunner.ScraperReadout
	err := c.Get(ctx, instancePath(name, "/readout"), &ro)
	return ro, err
}

// HostHealth is GET /api/instances/{n}/health (404 no_health before the
// first sample).
func (c *Ctl) HostHealth(ctx context.Context, name string) (hosthealth.Health, error) {
	var hh hosthealth.Health
	err := c.Get(ctx, instancePath(name, "/health"), &hh)
	return hh, err
}

// Diagnostics is GET /api/instances/{n}/diagnostics.
func (c *Ctl) Diagnostics(ctx context.Context, name string) (daemon.Diagnostics, error) {
	var d daemon.Diagnostics
	err := c.Get(ctx, instancePath(name, "/diagnostics"), &d)
	return d, err
}

// HostStatus is GET /api/instances/{n}/host (404 hostrunner_disabled when
// the daemon runs without --hostrunner).
func (c *Ctl) HostStatus(ctx context.Context, name string) (hostrunner.Status, error) {
	var st hostrunner.Status
	err := c.Get(ctx, instancePath(name, "/host"), &st)
	return st, err
}

// Attach is POST /api/ctl/attach {name, addr}; 202 = accepted (the attach
// runs asynchronously), 409 already_running, 400 invalid_name / bad_addr.
func (c *Ctl) Attach(ctx context.Context, name, addr string) (daemon.AttachResponse, error) {
	var resp daemon.AttachResponse
	err := c.Control(ctx, http.MethodPost, "/api/ctl/attach", daemon.AttachRequest{Name: name, Addr: addr}, &resp)
	return resp, err
}

// Detach is DELETE /api/ctl/instances/{n} (Stop + hold).
func (c *Ctl) Detach(ctx context.Context, name string) error {
	return c.Control(ctx, http.MethodDelete, ctlInstancePath(name, ""), nil, nil)
}

// Restart is POST /api/ctl/instances/{n}/restart (Stop + Start, 202).
func (c *Ctl) Restart(ctx context.Context, name string) error {
	return c.Control(ctx, http.MethodPost, ctlInstancePath(name, "/restart"), nil, nil)
}

// SetAuthority is POST /api/ctl/instances/{n}/host {"authority"}.
func (c *Ctl) SetAuthority(ctx context.Context, name, authority string) (hostrunner.Status, error) {
	var st hostrunner.Status
	err := c.Control(ctx, http.MethodPost, ctlInstancePath(name, "/host"), daemon.HostAuthorityRequest{Authority: authority}, &st)
	return st, err
}

// SetSelection is PUT /api/ctl/instances/{n}/host/selection {"map",
// "gametype"} — the two names only; the daemon computes the D-pad steps
// against its live carousel (§6.2).
func (c *Ctl) SetSelection(ctx context.Context, name, mapName, gametype string) (hostrunner.Status, error) {
	var st hostrunner.Status
	err := c.Control(ctx, http.MethodPut, ctlInstancePath(name, "/host/selection"), daemon.HostSelectionRequest{Map: mapName, Gametype: gametype}, &st)
	return st, err
}

// ClearSelection is DELETE /api/ctl/instances/{n}/host/selection.
func (c *Ctl) ClearSelection(ctx context.Context, name string) (hostrunner.Status, error) {
	var st hostrunner.Status
	err := c.Control(ctx, http.MethodDelete, ctlInstancePath(name, "/host/selection"), nil, &st)
	return st, err
}

// SetReady is PUT /api/ctl/instances/{n}/host/ready {"ready"}.
func (c *Ctl) SetReady(ctx context.Context, name string, ready bool) (hostrunner.Status, error) {
	var st hostrunner.Status
	err := c.Control(ctx, http.MethodPut, ctlInstancePath(name, "/host/ready"), daemon.HostReadyRequest{Ready: ready}, &st)
	return st, err
}

// GetConfig is GET /api/ctl/config (the effective document).
func (c *Ctl) GetConfig(ctx context.Context) (daemon.Document, error) {
	var doc daemon.Document
	err := c.Control(ctx, http.MethodGet, "/api/ctl/config", nil, &doc)
	return doc, err
}

// PutConfig is PUT /api/ctl/config (full replace of the control layer);
// returns the effective document.
func (c *Ctl) PutConfig(ctx context.Context, doc daemon.Document) (daemon.Document, error) {
	var eff daemon.Document
	err := c.Control(ctx, http.MethodPut, "/api/ctl/config", doc, &eff)
	return eff, err
}

// PutInstance is PUT /api/ctl/instances/{n} (InstanceConfig merge).
func (c *Ctl) PutInstance(ctx context.Context, name string, cfg daemon.InstanceConfig) (daemon.Document, error) {
	var eff daemon.Document
	err := c.Control(ctx, http.MethodPut, ctlInstancePath(name, ""), cfg, &eff)
	return eff, err
}

// PutOffsetSet is PUT /api/ctl/offset_sets/{id} with the raw set JSON.
func (c *Ctl) PutOffsetSet(ctx context.Context, id string, raw json.RawMessage) (daemon.Document, error) {
	var eff daemon.Document
	err := c.Control(ctx, http.MethodPut, "/api/ctl/offset_sets/"+url.PathEscape(id), raw, &eff)
	return eff, err
}
