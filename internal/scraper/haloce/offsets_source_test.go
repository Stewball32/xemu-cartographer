package haloce

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"testing"
)

// offsetsSourceJSON is the halo-offset-mapper export
// (offset-maps/cartographer-export/haloce_offsets.json, build ce-h1og-default)
// committed into this repo as the in-tree source of truth for the CE offsets.
// Its values are the authority the hand-reconciled constants in offsets.go /
// offsets_reference.go are copied from.
//
//go:embed offsets_source.json
var offsetsSourceJSON []byte

type offsetSourceEntry struct {
	Value string `json:"value"`
}

type offsetSource struct {
	TargetBuild string                       `json:"target_build"`
	SourceMap   string                       `json:"source_map"`
	Offsets     map[string]offsetSourceEntry `json:"offsets"`
}

// TestOffsetsMatchSourceExport guards the halo-offset-mapper → cartographer seam
// against BOTH local edits and upstream drift. The prior value-lock
// (TestRuntimeVerifiedOffsets) freezes this repo's copy in isolation; it cannot
// tell that the upstream map re-exported a corrected value. This test asserts
// each Go constant equals the value in the committed source export.
//
// Coverage is the set of constants whose Go name matches the export name
// exactly (55 as of the ce-h1og-default export). The remainder were renamed
// during hand-reconciliation (e.g. export OffPlayerName → OffPlrName); matching
// those by hand would be brittle, so they stay guarded by the runtime value-lock
// only. If upstream renames one of the mapped constants (so it disappears from
// the export), this test fails loudly rather than silently dropping coverage.
//
// When the source export legitimately changes, re-copy
// offset-maps/cartographer-export/haloce_offsets.json into offsets_source.json
// and reconcile offsets.go to match (see halo-offset-mapper
// docs/CARTOGRAPHER-IMPORT.md).
func TestOffsetsMatchSourceExport(t *testing.T) {
	var src offsetSource
	if err := json.Unmarshal(offsetsSourceJSON, &src); err != nil {
		t.Fatalf("parse offsets_source.json: %v", err)
	}
	if len(src.Offsets) == 0 {
		t.Fatal("offsets_source.json has no offsets — wrong file committed?")
	}

	// Cartographer constants whose name matches the export name exactly.
	// Generated from the intersection of offsets.go/offsets_reference.go and the
	// export; referencing the constants directly keeps them compile-checked.
	mapped := []struct {
		name string
		got  uint32
	}{
		{"AddrGameConnection", AddrGameConnection},
		{"AddrGameEngineGlobalsPtr", AddrGameEngineGlobalsPtr},
		{"AddrGameGlobalsPtr", AddrGameGlobalsPtr},
		{"AddrGlobalStageName", AddrGlobalStageName},
		{"AddrGlobalTagInstancesPtr", AddrGlobalTagInstancesPtr},
		{"AddrIsTeamGame", AddrIsTeamGame},
		{"AddrMainMenuActive", AddrMainMenuActive},
		{"AddrPlayerDatumArrayPtr", AddrPlayerDatumArrayPtr},
		{"AddrPlayersGlobalsPtr", AddrPlayersGlobalsPtr},
		{"AddrScoreCTF", AddrScoreCTF},
		{"AddrScoreLimitSlayer", AddrScoreLimitSlayer},
		{"AddrScoreSlayer", AddrScoreSlayer},
		{"AddrTagHeaderPtr", AddrTagHeaderPtr},
		{"ConstTagEntrySize", ConstTagEntrySize},
		{"OffDynAimX", OffDynAimX},
		{"OffDynAirborne", OffDynAirborne},
		{"OffDynCamo", OffDynCamo},
		{"OffDynCrouchScale", OffDynCrouchScale},
		{"OffDynCurrentAction", OffDynCurrentAction},
		{"OffDynFrags", OffDynFrags},
		{"OffDynHealth", OffDynHealth},
		{"OffDynMaxHealth", OffDynMaxHealth},
		{"OffDynMaxShields", OffDynMaxShields},
		{"OffDynParentObject", OffDynParentObject},
		{"OffDynPlasmas", OffDynPlasmas},
		{"OffDynSelectedSlot", OffDynSelectedSlot},
		{"OffDynShields", OffDynShields},
		{"OffDynWeaponSlot0", OffDynWeaponSlot0},
		{"OffDynZoomLevel", OffDynZoomLevel},
		{"OffGGActive", OffGGActive},
		{"OffGGMapLoaded", OffGGMapLoaded},
		{"OffPlrAssists", OffPlrAssists},
		{"OffPlrCTFScore", OffPlrCTFScore},
		{"OffPlrDeaths", OffPlrDeaths},
		{"OffPlrKillStreak", OffPlrKillStreak},
		{"OffPlrKills", OffPlrKills},
		{"OffPlrMultikill", OffPlrMultikill},
		{"OffPlrObjectHandle", OffPlrObjectHandle},
		{"OffProjDetonationTimer", OffProjDetonationTimer},
		{"OffScenarioItemCount", OffScenarioItemCount},
		{"OffScenarioPlayerSpawnCount", OffScenarioPlayerSpawnCount},
		{"OffScenarioPlayerSpawnFirst", OffScenarioPlayerSpawnFirst},
		{"OffTagHeaderTagArray", OffTagHeaderTagArray},
		{"OffTagHeaderTagCount", OffTagHeaderTagCount},
		{"OffTagNamePtr", OffTagNamePtr},
		{"OffWepAmmoPack", OffWepAmmoPack},
		{"RefAddrCTFFlag0Ptr", RefAddrCTFFlag0Ptr},
		{"RefAddrCTFFlag1Ptr", RefAddrCTFFlag1Ptr},
		{"RefAddrFPWeaponPtr", RefAddrFPWeaponPtr},
		{"RefAddrItemDatumSize", RefAddrItemDatumSize},
		{"RefAddrObjectDatumSize", RefAddrObjectDatumSize},
		{"RefAddrObjectTypeDefArray", RefAddrObjectTypeDefArray},
		{"RefAddrObserverCameraBase", RefAddrObserverCameraBase},
		{"RefAddrPlayerControlPtr", RefAddrPlayerControlPtr},
		{"RefAddrUnitDatumSize", RefAddrUnitDatumSize},
	}

	for _, m := range mapped {
		entry, ok := src.Offsets[m.name]
		if !ok {
			t.Errorf("%s: present in Go but missing from source export "+
				"(upstream rename/removal?) — reconcile against the export", m.name)
			continue
		}
		want, err := strconv.ParseUint(entry.Value, 0, 32)
		if err != nil {
			t.Errorf("%s: unparseable source value %q: %v", m.name, entry.Value, err)
			continue
		}
		if uint64(m.got) != want {
			t.Errorf("%s = 0x%X, but source export has %s (0x%X) — offset drift; reconcile",
				m.name, m.got, entry.Value, want)
		}
	}
}
