package haloce

import "testing"

// TestRuntimeVerifiedOffsets locks the Halo: CE offsets that the 2026-06-21 xemu
// runtime pass confirmed against live guest memory (halo-offset-mapper
// docs/RUNTIME-PASS-2026-06-21-CE.md). These came from the corrected
// export_cartographer.py export (build ce-h1og-default; all CE builds are
// byte-layout-identical, so the values hold for retail-NTSC too).
//
// This is a pure value-lock guard — no xemu needed. If a future edit drifts one
// of these constants, this test fails and points back to the verified source,
// so the live read path can't silently regress to a wrong address. The scenario
// tag chain in particular underpins the map-detection fix (see map_detect.go /
// TestResolveMapName); the rest are the active-read-path gameplay offsets the
// scraper surfaces (health/shield/ammo/grenades/score/clock).
func TestRuntimeVerifiedOffsets(t *testing.T) {
	cases := []struct {
		name string
		got  uint32
		want uint32
	}{
		// Map identity — cache tag header chain (the map-detection fix).
		{"AddrTagHeaderPtr", AddrTagHeaderPtr, 0x2DF1C4},
		{"OffTagHeaderTagArray", OffTagHeaderTagArray, 0x00},
		{"OffTagHeaderScenarioTagID", OffTagHeaderScenarioTagID, 0x04},
		{"OffTagHeaderTagCount", OffTagHeaderTagCount, 0x0C},
		{"ConstTagEntrySize", ConstTagEntrySize, 0x20},
		{"OffTagNamePtr", OffTagNamePtr, 0x10},
		{"AddrGlobalTagInstancesPtr", AddrGlobalTagInstancesPtr, 0x39CE24},

		// State gates.
		{"AddrMainMenuActive", AddrMainMenuActive, 0x2E4068},
		{"AddrGameConnection", AddrGameConnection, 0x2E3684},
		// MP-host-only hint (empty in menu/SP) — see resolveMapName.
		{"AddrGlobalStageName", AddrGlobalStageName, 0x2FAC20},
		// Game-globals map-loaded/active.
		{"AddrGameGlobalsPtr", AddrGameGlobalsPtr, 0x27629C},
		{"OffGGMapLoaded", OffGGMapLoaded, 0x00},
		{"OffGGActive", OffGGActive, 0x01},

		// Biped health/shield — normalized fraction + absolute caps.
		{"OffDynHealth", OffDynHealth, 0x90},
		{"OffDynShields", OffDynShields, 0x94},
		{"OffDynMaxHealth", OffDynMaxHealth, 0x88},
		{"OffDynMaxShields", OffDynMaxShields, 0x8C},
		{"OffDynFrags", OffDynFrags, 0x2CE},

		// Weapon ammo.
		{"OffWepAmmoMag", OffWepAmmoMag, 0x260},
		{"OffWepAmmoPack", OffWepAmmoPack, 0x25E},

		// Player record counters + handle.
		{"OffPlrKills", OffPlrKills, 0x98},
		{"OffPlrDeaths", OffPlrDeaths, 0xAA},
		{"OffPlrObjectHandle", OffPlrObjectHandle, 0x34},
		{"AddrPlayerDatumArrayPtr", AddrPlayerDatumArrayPtr, 0x2FAD28},

		// Match clock + scores.
		{"AddrGameTimeGlobalsPtr", AddrGameTimeGlobalsPtr, 0x2F8CA0},
		{"OffGTGGameTime", OffGTGGameTime, 0x0C},
		{"AddrScoreSlayer", AddrScoreSlayer, 0x276710},
		{"AddrScoreLimitSlayer", AddrScoreLimitSlayer, 0x2F90E8},
		{"AddrScoreCTF", AddrScoreCTF, 0x2762B4},
		{"RefAddrCTFFlag0Ptr", RefAddrCTFFlag0Ptr, 0x2762A4},

		// Projectile — the resolved +0x1C overlap is arming_time (f32), at the
		// same offset the old s32 target_object_index used.
		{"OffProjArmingTime", OffProjArmingTime, 0x1C},
		{"OffProjDetonationTimer", OffProjDetonationTimer, 0x14},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = 0x%X, want runtime-verified 0x%X", tc.name, tc.got, tc.want)
		}
	}
}

// TestArmingTimeReplacesTargetObjectIndex guards the projectile +0x1C semantic
// fix: the field is arming_time (f32), so OffProjArmingTime must sit at 0x1C and
// pair with OffProjArmingTimeDelta at 0x20 (mirroring detonation_timer /
// detonation_timer_delta at 0x14 / 0x18). RUNTIME-VERIFIED 2026-06-21: the s32
// reading at this offset was garbage (-1096152152), the f32 reading plausible
// (-0.3321).
func TestArmingTimeReplacesTargetObjectIndex(t *testing.T) {
	if OffProjArmingTime != 0x1C {
		t.Fatalf("OffProjArmingTime = 0x%X, want 0x1C", OffProjArmingTime)
	}
	if OffProjArmingTimeDelta != OffProjArmingTime+0x04 {
		t.Errorf("OffProjArmingTimeDelta = 0x%X, want value+delta pair at 0x%X",
			OffProjArmingTimeDelta, OffProjArmingTime+0x04)
	}
	if OffProjDetonationTimerDelta != OffProjDetonationTimer+0x04 {
		t.Errorf("OffProjDetonationTimerDelta = 0x%X, want value+delta pair at 0x%X",
			OffProjDetonationTimerDelta, OffProjDetonationTimer+0x04)
	}
}
