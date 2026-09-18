package pb

import (
	"strings"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

// LegacyEnvVar is the pre-authz LAN secret (PD-12). While set, it is
// accepted as an in-memory machine key with LegacyScopes.
const LegacyEnvVar = "LAN_SAVES_TOKEN"

// LegacyScopes are the scopes the imported LAN_SAVES_TOKEN row carries.
var LegacyScopes = []string{"lan.saves.*", "lan.sync.*"}

// legacyLabel is the label the legacy row shows in listings.
const legacyLabel = "LAN_SAVES_TOKEN (legacy env)"

// ImportLegacyEnv reads LAN_SAVES_TOKEN through env (os.Getenv in main;
// injectable for tests) into the adapter's in-memory row {Kid: legacy-env,
// Kind: machine, Scopes: LegacyScopes}. The secret is kept only as its
// hash. An unset / blank value clears any previous import and reports
// false.
func ImportLegacyEnv(d *PBDeps, env func(string) string) (imported bool) {
	if d == nil || env == nil {
		return false
	}
	secret := strings.TrimSpace(env(LegacyEnvVar))
	if secret == "" {
		d.setLegacy(nil)
		return false
	}
	d.setLegacy(&authz.TokenRow{
		Kid:     authz.LegacyKid,
		KeyHash: authz.HashSecret(secret),
		Kind:    "machine",
		Label:   legacyLabel,
		Scopes:  authz.CanonScopes(LegacyScopes),
	})
	return true
}

// LegacyImported reports whether ImportLegacyEnv installed a row.
func (d *PBDeps) LegacyImported() bool {
	return d.legacyRow() != nil
}
