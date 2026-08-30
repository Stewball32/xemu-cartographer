package halo2

import (
	"reflect"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
)

// TestBaselineOffsetsMatchConstants is the behavior-preservation proof for the
// offset-versioning refactor: every field bound from the halo2 baseline config
// must equal the previously hardcoded package constant of the same name.
func TestBaselineOffsetsMatchConstants(t *testing.T) {
	o, err := OffsetsFromSet(offsets.Baseline("halo2"))
	if err != nil {
		t.Fatalf("bind baseline: %v", err)
	}
	wantU := map[string]uint32{
		"AddrH2ObjectArrayPtr":        AddrH2ObjectArrayPtr,
		"AddrH2PlayersArrayPtr":       AddrH2PlayersArrayPtr,
		"AddrH2ScenarioNamePoolPtr":   AddrH2ScenarioNamePoolPtr,
		"AddrH2TagHeaderPtr":          AddrH2TagHeaderPtr,
		"OffH2BipedAirborne":          OffH2BipedAirborne,
		"OffH2BipedCurWeaponSlot":     OffH2BipedCurWeaponSlot,
		"OffH2BipedFragGrenades":      OffH2BipedFragGrenades,
		"OffH2BipedHealth":            OffH2BipedHealth,
		"OffH2BipedHeldWeapon":        OffH2BipedHeldWeapon,
		"OffH2BipedMaxHealth":         OffH2BipedMaxHealth,
		"OffH2BipedMaxShield":         OffH2BipedMaxShield,
		"OffH2BipedPlasmaGrenades":    OffH2BipedPlasmaGrenades,
		"OffH2BipedShield":            OffH2BipedShield,
		"OffH2BipedWeaponSlots":       OffH2BipedWeaponSlots,
		"OffH2BipedZoomLevel":         OffH2BipedZoomLevel,
		"OffH2DataArrayActiveCount":   OffH2DataArrayActiveCount,
		"OffH2DataArrayBlockEnd":      OffH2DataArrayBlockEnd,
		"OffH2DataArrayBlockPtr":      OffH2DataArrayBlockPtr,
		"OffH2DataArrayElemSize":      OffH2DataArrayElemSize,
		"OffH2DataArrayMax":           OffH2DataArrayMax,
		"OffH2DataArraySignature":     OffH2DataArraySignature,
		"OffH2ObjAim":                 OffH2ObjAim,
		"OffH2ObjEntryDataPtr":        OffH2ObjEntryDataPtr,
		"OffH2ObjEntrySalt":           OffH2ObjEntrySalt,
		"OffH2ObjForward":             OffH2ObjForward,
		"OffH2ObjParent":              OffH2ObjParent,
		"OffH2ObjPosition":            OffH2ObjPosition,
		"OffH2ObjTagId":               OffH2ObjTagId,
		"OffH2ObjType":                OffH2ObjType,
		"OffH2ObjVelocity":            OffH2ObjVelocity,
		"OffH2PlrDatumId":             OffH2PlrDatumId,
		"OffH2PlrIndex":               OffH2PlrIndex,
		"OffH2PlrName":                OffH2PlrName,
		"OffH2PlrPlayerId":            OffH2PlrPlayerId,
		"OffH2PlrTeam":                OffH2PlrTeam,
		"OffH2PlrUnitHandle":          OffH2PlrUnitHandle,
		"OffH2TagHdrCount":            OffH2TagHdrCount,
		"OffH2TagHdrGlobalsId":        OffH2TagHdrGlobalsId,
		"OffH2TagHdrInstanceArrayPtr": OffH2TagHdrInstanceArrayPtr,
		"OffH2TagHdrScenarioId":       OffH2TagHdrScenarioId,
		"OffH2TagInstDataPtr":         OffH2TagInstDataPtr,
		"OffH2TagInstGroup":           OffH2TagInstGroup,
		"OffH2TagInstId":              OffH2TagInstId,
		"OffH2WepMag":                 OffH2WepMag,
		"OffH2WepReserve":             OffH2WepReserve,
		// 2026-08-29 import — the verified-but-unwired stats/config/machine
		// globals (halo-offset-mapper h2-stock map + kd-semantics 2026-07-11).
		"AddrH2KillsPerPlayer":       AddrH2KillsPerPlayer,
		"AddrH2DeathsPerPlayer":      AddrH2DeathsPerPlayer,
		"AddrH2KillsTotal":           AddrH2KillsTotal,
		"AddrH2Gametype":             AddrH2Gametype,
		"AddrH2GamePhase":            AddrH2GamePhase, // 0 sentinel on stock
		"AddrH2NetLocalMachineIndex": AddrH2NetLocalMachineIndex,
		"AddrH2NetMachineMacArray":   AddrH2NetMachineMacArray,
		"AddrH2NetMachineTable":      AddrH2NetMachineTable,
		"OffH2PlrMachineIndex":       OffH2PlrMachineIndex,
		"OffH2PlrMacOctet":           OffH2PlrMacOctet,
		"OffH2PlrBetrayals":          OffH2PlrBetrayals,
		"OffH2NetMachineName":        OffH2NetMachineName,
		"OffH2ScenarioPathInPool":    OffH2ScenarioPathInPool,
	}
	wantS := map[string]string{}
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
