package containers

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

// makeResp builds a minimal *http.Response for injectBaseHref to chew on.
func makeResp(body []byte, gzipped bool) *http.Response {
	h := make(http.Header)
	h.Set("Content-Type", "text/html; charset=utf-8")
	if gzipped {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, _ = gw.Write(body)
		_ = gw.Close()
		body = buf.Bytes()
		h.Set("Content-Encoding", "gzip")
	}
	return &http.Response{
		Header: h,
		Body:   io.NopCloser(bytes.NewReader(body)),
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gr.Close()
		body, err = io.ReadAll(gr)
		if err != nil {
			t.Fatalf("gunzip: %v", err)
		}
	}
	return string(body)
}

func TestInjectBaseHrefPlain(t *testing.T) {
	resp := makeResp([]byte("<html><head><title>k</title></head><body>x</body></html>"), false)
	if err := injectBaseHref(resp, "/api/admin/containers/alpha/kiosk/"); err != nil {
		t.Fatalf("inject: %v", err)
	}
	got := readBody(t, resp)
	if !strings.Contains(got, `<base href="/api/admin/containers/alpha/kiosk/">`) {
		t.Errorf("missing <base> tag: %q", got)
	}
	if !strings.Contains(got, "<title>k</title>") {
		t.Errorf("body damaged: %q", got)
	}
}

func TestInjectBaseHrefGzipped(t *testing.T) {
	resp := makeResp([]byte("<html><head></head><body></body></html>"), true)
	if err := injectBaseHref(resp, "/p/"); err != nil {
		t.Fatalf("inject: %v", err)
	}
	got := readBody(t, resp)
	if !strings.Contains(got, `<base href="/p/">`) {
		t.Errorf("missing <base> after gzip round-trip: %q", got)
	}
}

func TestInjectBaseHrefNoHead(t *testing.T) {
	// HTML without an explicit <head> — the helper should splice one in.
	resp := makeResp([]byte("<html><body>hi</body></html>"), false)
	if err := injectBaseHref(resp, "/p/"); err != nil {
		t.Fatalf("inject: %v", err)
	}
	got := readBody(t, resp)
	if !strings.Contains(got, "<head>") || !strings.Contains(got, `<base href="/p/">`) {
		t.Errorf("missing synthesised <head>+<base>: %q", got)
	}
}
