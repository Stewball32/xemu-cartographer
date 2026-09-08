package pb_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// resolve runs ResolveRequest over a synthetic REST event and returns the
// principal + error.
func resolve(t *testing.T, app core.App, d *pb.PBDeps, hdr map[string]string) (authz.Principal, error) {
	t.Helper()
	e := newEvent(app, nil, http.MethodPost, "/api/xc/finished_game", hdr)
	return pb.ResolveRequest(app, d, e)
}

func TestImportWebhookEnv(t *testing.T) {
	app, d := pbtest.NewApp(t)
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }

	if pb.ImportWebhookEnv(d, getenv) || d.WebhookImported() {
		t.Fatalf("unset env must not import")
	}
	// Nothing imported: a raw Bearer is still no credential at all.
	if p, err := resolve(t, app, d, map[string]string{"Authorization": "Bearer hook-s3cret"}); err != nil || p.Kind != authz.KindAnonymous {
		t.Fatalf("no import: principal = %+v err = %v, want Nobody", p, err)
	}

	env[pb.WebhookEnvVar] = "  hook-s3cret  "
	if !pb.ImportWebhookEnv(d, getenv) || !d.WebhookImported() {
		t.Fatalf("set env must import")
	}
	// The kid is never presented on the wire, so LookupToken keeps ignoring it
	// and the legacy row is untouched.
	if _, ok := d.LookupToken(pb.WebhookKid); ok {
		t.Fatalf("webhook row must not be addressable by kid")
	}
	if d.LegacyImported() {
		t.Fatalf("webhook import must not touch the legacy row")
	}

	// The raw secret resolves on the REST carriers (trimmed, Bearer optional) …
	for _, hdr := range []map[string]string{
		{"Authorization": "Bearer hook-s3cret"},
		{"Authorization": "hook-s3cret"},
		{"X-Api-Key": "hook-s3cret"},
	} {
		p, err := resolve(t, app, d, hdr)
		if err != nil {
			t.Fatalf("%v: %v", hdr, err)
		}
		if p.Kind != authz.KindMachine || p.ID != pb.WebhookKid {
			t.Fatalf("%v: principal = %+v", hdr, p)
		}
		if got := strings.Join(p.Scopes, ","); got != "scraper.ingest" {
			t.Fatalf("%v: scopes = %q", hdr, got)
		}
		if !authz.Can(d, p, authz.ActionScraperIngest, authz.Global()) {
			t.Fatalf("%v: webhook principal must pass scraper.ingest", hdr)
		}
		if authz.Can(d, p, authz.ActionAdminScraper, authz.Global()) || authz.Can(d, p, authz.ActionLANSavesBuild, authz.Global()) {
			t.Fatalf("%v: webhook principal must not pass other actions", hdr)
		}
	}

	// … never on the LAN carriers (those stay LAN_SAVES_TOKEN's) …
	for _, hdr := range []map[string]string{
		{"X-LAN-Token": "hook-s3cret"},
		{"Authorization": "Bearer hook-s3cret"},
	} {
		e := newEvent(app, nil, http.MethodGet, "/api/lan/saves/build", hdr)
		if code, _ := runLAN(t, d, e, lanBuild); code == http.StatusOK {
			t.Fatalf("%v: webhook secret must not open /api/lan/*", hdr)
		}
	}

	// … and a non-match keeps today's outcome: no credential, an unknown
	// opaque kid, a bad secret on a real key.
	if p, err := resolve(t, app, d, map[string]string{"Authorization": "Bearer nope"}); err != nil || p.Kind != authz.KindAnonymous {
		t.Fatalf("wrong secret: principal = %+v err = %v, want Nobody", p, err)
	}
	if _, err := resolve(t, app, d, map[string]string{"Authorization": "Bearer mk_unknown.zzz"}); !errors.Is(err, authz.ErrUnknownKid) {
		t.Fatalf("unknown kid: err = %v, want ErrUnknownKid", err)
	}
	kid, key := pbtest.MintToken(t, app, d, "machine", []string{"scraper.ingest"})
	if _, err := resolve(t, app, d, map[string]string{"Authorization": "Bearer " + kid + ".wrong"}); !errors.Is(err, authz.ErrBadSecret) {
		t.Fatalf("bad secret on minted key: err = %v, want ErrBadSecret", err)
	}
	if p, err := resolve(t, app, d, map[string]string{"Authorization": "Bearer " + key}); err != nil || p.ID != kid {
		t.Fatalf("minted key: principal = %+v err = %v", p, err)
	}

	// Clearing the env drops the row.
	env[pb.WebhookEnvVar] = ""
	if pb.ImportWebhookEnv(d, getenv) || d.WebhookImported() {
		t.Fatalf("blank env must clear the import")
	}
	if p, _ := resolve(t, app, d, map[string]string{"Authorization": "Bearer hook-s3cret"}); p.Kind != authz.KindAnonymous {
		t.Fatalf("after clear: principal = %+v, want Nobody", p)
	}

	// Nil-safe.
	if pb.ImportWebhookEnv(nil, getenv) || pb.ImportWebhookEnv(d, nil) {
		t.Fatalf("nil deps / env must report false")
	}
	var nilDeps *pb.PBDeps
	if nilDeps.WebhookImported() {
		t.Fatalf("nil deps reports imported")
	}
}

// TestWebhookEnvWithDot: an env value containing '.' whose head is a known
// kid prefix is still matched whole (the "<prefix>." head is just how the
// secret happens to start).
func TestWebhookEnvWithDot(t *testing.T) {
	app, d := pbtest.NewApp(t)
	secret := "mk_looks.opaque"
	if !pb.ImportWebhookEnv(d, func(string) string { return secret }) {
		t.Fatalf("import failed")
	}
	p, err := resolve(t, app, d, map[string]string{"Authorization": "Bearer " + secret})
	if err != nil || p.ID != pb.WebhookKid {
		t.Fatalf("principal = %+v err = %v", p, err)
	}
}

func TestLogWebhookEnv(t *testing.T) {
	var lines []string
	log := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	pb.LogWebhookEnv(true, log)
	pb.LogWebhookEnv(false, log)
	pb.LogWebhookEnv(true, nil)
	if len(lines) != 2 {
		t.Fatalf("lines = %v", lines)
	}
	if !strings.Contains(lines[0], "kid=webhook-env") || !strings.Contains(lines[0], "scopes=[scraper.ingest]") || !strings.Contains(lines[0], "WARNING") {
		t.Errorf("nag line = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "authz: XC_SCRAPER_WEBHOOK_TOKEN unset") {
		t.Errorf("unset line = %q", lines[1])
	}
}
