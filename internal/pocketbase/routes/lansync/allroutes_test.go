package lansync

import (
	"testing"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

// TestSyncVerbMap pins the group-relative path → LAN sync verb map the
// pb.AuthorizeLAN middleware consults (R-6). /dl/game/{id} addresses the ISO
// so the rule can scope it; anything not in the table maps to a verb no rule
// grants, so a new route is denied until it is added here.
func TestSyncVerbMap(t *testing.T) {
	cases := []struct {
		path     string
		want     authz.Action
		wantKind authz.ResourceKind
		wantID   string
	}{
		{"/manifest", authz.ActionLANSyncManifest, authz.ResGlobal, ""},
		{"/manifest/", authz.ActionLANSyncManifest, authz.ResGlobal, ""},
		{"/dl/game/abc123", authz.ActionLANSyncDLGame, authz.ResISO, "abc123"},
		{"/dl/game/abc123/", authz.ActionLANSyncDLGame, authz.ResISO, "abc123"},
		{"/dl/app/xyz", authz.ActionLANSyncDLApp, authz.ResGlobal, ""},
		{"", syncActionUnmapped, authz.ResGlobal, ""},
		{"/", syncActionUnmapped, authz.ResGlobal, ""},
		{"/manifestx", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl/game", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl/game/", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl/game/a/b", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl/app/", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl/app/a/b", syncActionUnmapped, authz.ResGlobal, ""},
		{"/dl/other/x", syncActionUnmapped, authz.ResGlobal, ""},
	}
	for _, c := range cases {
		got, res := syncVerbFor(c.path)
		if got != c.want {
			t.Errorf("syncVerbFor(%q) = %q, want %q", c.path, got, c.want)
		}
		if res.Kind != c.wantKind || res.ID != c.wantID {
			t.Errorf("syncVerbFor(%q) resource = %s/%q, want %s/%q", c.path, res.Kind, res.ID, c.wantKind, c.wantID)
		}
	}
	// The unmapped sentinel must never be a granted verb.
	if syncActionUnmapped.Known() {
		t.Fatalf("%q must not be part of the action vocabulary", syncActionUnmapped)
	}
}
