package pb

import (
	"strings"
	"time"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

// WebhookEnvVar is the shared secret the xc-scraper daemon presents on
// POST /api/xc/finished_game (step 8 §7.2): the same value is the daemon's
// --webhook-token, sent verbatim as `Authorization: Bearer <value>`. While
// set, it is accepted as an in-memory machine key with WebhookScopes.
const WebhookEnvVar = "XC_SCRAPER_WEBHOOK_TOKEN"

// WebhookKid is the kid of the in-memory machine row ImportWebhookEnv
// installs. It is never presented on the wire (the daemon sends the raw
// secret), so it needs no KidPrefix and never reaches LookupToken.
const WebhookKid = "webhook-env"

// WebhookScopes are the scopes the imported XC_SCRAPER_WEBHOOK_TOKEN row
// carries — the ingest route only.
var WebhookScopes = []string{string(authz.ActionScraperIngest)}

// webhookLabel is the label the webhook row shows in listings / logs.
const webhookLabel = "XC_SCRAPER_WEBHOOK_TOKEN (env)"

// ImportWebhookEnv reads XC_SCRAPER_WEBHOOK_TOKEN through env (os.Getenv in
// main; injectable for tests) into the adapter's in-memory row {Kid:
// webhook-env, Kind: machine, Scopes: WebhookScopes}, mirroring
// ImportLegacyEnv. The secret is kept only as its hash. An unset / blank
// value clears any previous import and reports false.
//
// The row is matched by the whole presented value on the REST carriers
// (Authorization: Bearer / X-Api-Key, see resolveEvent), so the operator
// sets one env value on both sides. It works in every build and tier;
// the boot report nags to mint a real api_tokens machine key instead:
// POST /api/admin/tokens {"kind":"machine","scopes":["scraper.ingest"]}.
func ImportWebhookEnv(d *PBDeps, env func(string) string) (imported bool) {
	if d == nil || env == nil {
		return false
	}
	secret := strings.TrimSpace(env(WebhookEnvVar))
	if secret == "" {
		d.setWebhook(nil)
		return false
	}
	d.setWebhook(&authz.TokenRow{
		Kid:     WebhookKid,
		KeyHash: authz.HashSecret(secret),
		Kind:    "machine",
		Label:   webhookLabel,
		Scopes:  authz.CanonScopes(WebhookScopes),
	})
	return true
}

// WebhookImported reports whether ImportWebhookEnv installed a row.
func (d *PBDeps) WebhookImported() bool {
	return d.webhookRow() != nil
}

// LogWebhookEnv prints the boot-report line for the webhook key beside the
// eight `authz:` lines: the nag to mint a real key while the env import is
// live, else a one-line "unset" so operators can tell the route is
// api_tokens-only.
func LogWebhookEnv(imported bool, log func(format string, args ...any)) {
	if log == nil {
		return
	}
	if imported {
		log("authz: %s imported as kid=%s scopes=[%s] — WARNING: mint an api_tokens machine key (POST /api/admin/tokens kind=machine scopes=[scraper.ingest]), pass it as the daemon's --webhook-token and unset the env var", WebhookEnvVar, WebhookKid, strings.Join(WebhookScopes, ", "))
		return
	}
	log("authz: %s unset — /api/xc/finished_game accepts api_tokens machine keys with scraper.ingest only", WebhookEnvVar)
}

// webhookRow returns a copy of the imported XC_SCRAPER_WEBHOOK_TOKEN row,
// nil when none was imported.
func (d *PBDeps) webhookRow() *authz.TokenRow {
	if d == nil {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.webhook == nil {
		return nil
	}
	row := *d.webhook
	row.Scopes = append([]string(nil), d.webhook.Scopes...)
	return &row
}

func (d *PBDeps) setWebhook(row *authz.TokenRow) {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.webhook = row
	d.mu.Unlock()
}

// resolveWebhookSecret compares a whole presented REST credential against
// the imported webhook row (constant-time via VerifyToken). ok is false
// when no row is imported or the value does not match — callers then keep
// the outcome they had, so the import never changes another credential's
// result.
func resolveWebhookSecret(d *PBDeps, value string) (authz.Principal, bool) {
	row := d.webhookRow()
	if row == nil {
		return authz.Nobody(), false
	}
	if err := authz.VerifyToken(*row, value, time.Now()); err != nil {
		return authz.Nobody(), false
	}
	return authz.PrincipalFromToken(*row), true
}
