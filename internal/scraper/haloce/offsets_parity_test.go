package haloce

import (
	"reflect"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
)

// TestBaselineOffsetsMatchConstants is the behavior-preservation proof for the
// offset-versioning refactor: every field bound from the haloce baseline config
// must equal the previously hardcoded package constant of the same name.
func TestBaselineOffsetsMatchConstants(t *testing.T) {
	s, err := offsets.Baseline("haloce")
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	o, err := OffsetsFromSet(s)
	if err != nil {
		t.Fatalf("bind baseline: %v", err)
	}
	wantU := map[string]uint32{
		"AddrGameCanScore":               AddrGameCanScore,
		"AddrGameClientPtr":              AddrGameClientPtr,
		"AddrGameConnection":             AddrGameConnection,
		"AddrGameEngineGlobalsPtr":       AddrGameEngineGlobalsPtr,
		"AddrGameGlobalsPtr":             AddrGameGlobalsPtr,
		"AddrGameOverFlag":               AddrGameOverFlag,
		"AddrGameServerPtr":              AddrGameServerPtr,
		"AddrGameTimeGlobalsPtr":         AddrGameTimeGlobalsPtr,
		"AddrGameVariantGlobalPtr":       AddrGameVariantGlobalPtr,
		"AddrGlobalGameGlobalsPtr":       AddrGlobalGameGlobalsPtr,
		"AddrGlobalScenarioPtr":          AddrGlobalScenarioPtr,
		"AddrGlobalStageName":            AddrGlobalStageName,
		"AddrGlobalTagInstancesPtr":      AddrGlobalTagInstancesPtr,
		"AddrIsTeamGame":                 AddrIsTeamGame,
		"AddrMainMenuActive":             AddrMainMenuActive,
		"AddrObjectHeaderDatumPtr":       AddrObjectHeaderDatumPtr,
		"AddrPlayerDatumArrayPtr":        AddrPlayerDatumArrayPtr,
		"AddrPlayersGlobalsPtr":          AddrPlayersGlobalsPtr,
		"AddrScoreCTF":                   AddrScoreCTF,
		"AddrScoreKing":                  AddrScoreKing,
		"AddrScoreLimitCTF":              AddrScoreLimitCTF,
		"AddrScoreLimitOddball":          AddrScoreLimitOddball,
		"AddrScoreLimitSlayer":           AddrScoreLimitSlayer,
		"AddrScoreOddball":               AddrScoreOddball,
		"AddrScoreRace":                  AddrScoreRace,
		"AddrScoreSlayer":                AddrScoreSlayer,
		"AddrTagHeaderPtr":               AddrTagHeaderPtr,
		"AddrTeamsPtr":                   AddrTeamsPtr,
		"AddrUiBackScreenRec":            AddrUiBackScreenRec,
		"AddrUiCurrentScreenRec":         AddrUiCurrentScreenRec,
		"AddrUiFadeState":                AddrUiFadeState,
		"AddrUiMsClock":                  AddrUiMsClock,
		"AddrUiOskActive":                AddrUiOskActive,
		"AddrUiSlotClaimed":              AddrUiSlotClaimed,
		"AddrUiSlotProfile":              AddrUiSlotProfile,
		"AddrUiWidgetFocusPtr":           AddrUiWidgetFocusPtr,
		"AddrVariant":                    AddrVariant,
		"ConstUiWidgetHeapGVAHi":         ConstUiWidgetHeapGVAHi,
		"ConstUiWidgetHeapGVALo":         ConstUiWidgetHeapGVALo,
		"RefAddrCTFFlag0Ptr":             RefAddrCTFFlag0Ptr,
		"RefAddrCTFFlag1Ptr":             RefAddrCTFFlag1Ptr,
		"RefAddrDefaultFramerate":        RefAddrDefaultFramerate,
		"RefAddrFPWeaponPtr":             RefAddrFPWeaponPtr,
		"RefAddrFogParams":               RefAddrFogParams,
		"RefAddrGameStateBasePtr":        RefAddrGameStateBasePtr,
		"RefAddrGameStateSize":           RefAddrGameStateSize,
		"RefAddrGamepadState":            RefAddrGamepadState,
		"RefAddrGamepadStateAlt":         RefAddrGamepadStateAlt,
		"RefAddrGlobalRandomSeed":        RefAddrGlobalRandomSeed,
		"RefAddrGlobalVariant":           RefAddrGlobalVariant,
		"RefAddrHudMessagesPtr":          RefAddrHudMessagesPtr,
		"RefAddrInputAbstractGlbls":      RefAddrInputAbstractGlbls,
		"RefAddrInputAbstractInputState": RefAddrInputAbstractInputState,
		"RefAddrItemDatumSize":           RefAddrItemDatumSize,
		"RefAddrLookPitchRate":           RefAddrLookPitchRate,
		"RefAddrLookYawRate":             RefAddrLookYawRate,
		"RefAddrNetworkGameClient":       RefAddrNetworkGameClient,
		"RefAddrNetworkGameServer":       RefAddrNetworkGameServer,
		"RefAddrObjectDatumSize":         RefAddrObjectDatumSize,
		"RefAddrObjectTypeDefArray":      RefAddrObjectTypeDefArray,
		"RefAddrObjectTypeDefRangeHi":    RefAddrObjectTypeDefRangeHi,
		"RefAddrObserverCameraBase":      RefAddrObserverCameraBase,
		"RefAddrPerLocalUIGlobals":       RefAddrPerLocalUIGlobals,
		"RefAddrPlayerControlPtr":        RefAddrPlayerControlPtr,
		"RefAddrRefreshRate":             RefAddrRefreshRate,
		"RefAddrSoundCacheBasePtr":       RefAddrSoundCacheBasePtr,
		"RefAddrSoundCacheSize":          RefAddrSoundCacheSize,
		"RefAddrTagCacheBasePtr":         RefAddrTagCacheBasePtr,
		"RefAddrTagCacheSize":            RefAddrTagCacheSize,
		"RefAddrTextureCacheBasePtr":     RefAddrTextureCacheBasePtr,
		"RefAddrTextureCacheSize":        RefAddrTextureCacheSize,
		"RefAddrUnitDatumSize":           RefAddrUnitDatumSize,
		"RefAddrUpdateClientPlayerPtr":   RefAddrUpdateClientPlayerPtr,
		"RefAddrUpdateQueueAdjacent":     RefAddrUpdateQueueAdjacent,
		"RefAddrUpdateQueueCounterHi":    RefAddrUpdateQueueCounterHi,
		"RefAddrUpdateQueueCounterLo":    RefAddrUpdateQueueCounterLo,
	}
	wantS := map[string]string{
		"TagPathGameSettingNames":   TagPathGameSettingNames,
		"TagPathGametypeSelectList": TagPathGametypeSelectList,
		"TagPathMPMapDescriptions":  TagPathMPMapDescriptions,
		"TagPathMPMapList":          TagPathMPMapList,
		"TagPathMPMapSelectList":    TagPathMPMapSelectList,
	}
	rv := reflect.ValueOf(o)
	for name, want := range wantU {
		if got := uint32(rv.FieldByName(name).Uint()); got != want {
			t.Errorf("%s = 0x%X, want 0x%X (constant)", name, got, want)
		}
	}
	for name, want := range wantS {
		if got := rv.FieldByName(name).String(); got != want {
			t.Errorf("%s = %q, want %q (constant)", name, got, want)
		}
	}
	if rv.NumField() != len(wantU)+len(wantS) {
		t.Errorf("struct has %d fields, parity table covers %d", rv.NumField(), len(wantU)+len(wantS))
	}
}
