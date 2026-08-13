// Code generated for the offset-versioning layer (baseline extracted from the
// package constants). The struct fields mirror the config keys 1:1; binding is
// reflection-driven so a missing key in a set fails loudly at bind time.
package halo2

import (
	"fmt"
	"reflect"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
)

// Offsets is the versioned address layer this plugin reads through at runtime.
// Game BEHAVIOR stays in code; these values come from the instance's offset set
// (default: the game's baseline). Field names match the config keys exactly.
type Offsets struct {
	AddrH2ObjectArrayPtr        uint32
	AddrH2PlayersArrayPtr       uint32
	AddrH2ScenarioNamePoolPtr   uint32
	AddrH2TagHeaderPtr          uint32
	OffH2BipedAirborne          uint32
	OffH2BipedCurWeaponSlot     uint32
	OffH2BipedFragGrenades      uint32
	OffH2BipedHealth            uint32
	OffH2BipedHeldWeapon        uint32
	OffH2BipedMaxHealth         uint32
	OffH2BipedMaxShield         uint32
	OffH2BipedPlasmaGrenades    uint32
	OffH2BipedShield            uint32
	OffH2BipedWeaponSlots       uint32
	OffH2BipedZoomLevel         uint32
	OffH2DataArrayActiveCount   uint32
	OffH2DataArrayBlockEnd      uint32
	OffH2DataArrayBlockPtr      uint32
	OffH2DataArrayElemSize      uint32
	OffH2DataArrayMax           uint32
	OffH2DataArraySignature     uint32
	OffH2ObjAim                 uint32
	OffH2ObjEntryDataPtr        uint32
	OffH2ObjEntrySalt           uint32
	OffH2ObjForward             uint32
	OffH2ObjParent              uint32
	OffH2ObjPosition            uint32
	OffH2ObjTagId               uint32
	OffH2ObjType                uint32
	OffH2ObjVelocity            uint32
	OffH2PlrDatumId             uint32
	OffH2PlrIndex               uint32
	OffH2PlrName                uint32
	OffH2PlrPlayerId            uint32
	OffH2PlrTeam                uint32
	OffH2PlrUnitHandle          uint32
	OffH2TagHdrCount            uint32
	OffH2TagHdrGlobalsId        uint32
	OffH2TagHdrInstanceArrayPtr uint32
	OffH2TagHdrScenarioId       uint32
	OffH2TagInstDataPtr         uint32
	OffH2TagInstGroup           uint32
	OffH2TagInstId              uint32
	OffH2WepMag                 uint32
	OffH2WepReserve             uint32
}

// OffsetsFromSet binds a generic offset set into the typed struct. Every field
// must be present in the set — a partial set errors here, never mid-read.
func OffsetsFromSet(s *offsets.Set) (Offsets, error) {
	var o Offsets
	rv := reflect.ValueOf(&o).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		switch rt.Field(i).Type.Kind() {
		case reflect.String:
			v, err := s.Str(name)
			if err != nil {
				return Offsets{}, fmt.Errorf("bind %s: %w", name, err)
			}
			rv.Field(i).SetString(v)
		default:
			v, err := s.Addr(name)
			if err != nil {
				return Offsets{}, fmt.Errorf("bind %s: %w", name, err)
			}
			rv.Field(i).SetUint(uint64(v))
		}
	}
	return o, nil
}

// BaselineOffsets returns the game's baseline binding. Panics only if the
// embedded baseline is malformed (caught by tests + init).
func BaselineOffsets() Offsets {
	o, err := OffsetsFromSet(offsets.Baseline("halo2"))
	if err != nil {
		panic(fmt.Sprintf("halo2: baseline offsets: %v", err))
	}
	return o
}
