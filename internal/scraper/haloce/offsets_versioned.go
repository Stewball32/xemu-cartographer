// Code generated for the offset-versioning layer (baseline extracted from the
// package constants). The struct fields mirror the config keys 1:1; binding is
// reflection-driven so a missing key in a set fails loudly at bind time.
package haloce

import (
	"fmt"
	"reflect"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
)

// Offsets is the versioned address layer this plugin reads through at runtime.
// Game BEHAVIOR stays in code; these values come from the instance's offset set
// (default: the game's baseline). Field names match the config keys exactly.
type Offsets struct {
	AddrGameCanScore               uint32
	AddrGameClientPtr              uint32
	AddrGameConnection             uint32
	AddrGameEngineGlobalsPtr       uint32
	AddrGameGlobalsPtr             uint32
	AddrGameOverFlag               uint32
	AddrGameServerPtr              uint32
	AddrGameTimeGlobalsPtr         uint32
	AddrGameVariantGlobalPtr       uint32
	AddrGlobalGameGlobalsPtr       uint32
	AddrGlobalScenarioPtr          uint32
	AddrGlobalStageName            uint32
	AddrGlobalTagInstancesPtr      uint32
	AddrIsTeamGame                 uint32
	AddrMainMenuActive             uint32
	AddrObjectHeaderDatumPtr       uint32
	AddrPlayerDatumArrayPtr        uint32
	AddrPlayersGlobalsPtr          uint32
	AddrScoreCTF                   uint32
	AddrScoreKing                  uint32
	AddrScoreLimitCTF              uint32
	AddrScoreLimitOddball          uint32
	AddrScoreLimitSlayer           uint32
	AddrScoreOddball               uint32
	AddrScoreRace                  uint32
	AddrScoreSlayer                uint32
	AddrTagHeaderPtr               uint32
	AddrTeamsPtr                   uint32
	AddrUiBackScreenRec            uint32
	AddrUiCurrentScreenRec         uint32
	AddrUiFadeState                uint32
	AddrUiMsClock                  uint32
	AddrUiOskActive                uint32
	AddrUiSlotClaimed              uint32
	AddrUiSlotProfile              uint32
	AddrUiWidgetFocusPtr           uint32
	AddrVariant                    uint32
	ConstUiWidgetHeapGVAHi         uint32
	ConstUiWidgetHeapGVALo         uint32
	RefAddrCTFFlag0Ptr             uint32
	RefAddrCTFFlag1Ptr             uint32
	RefAddrDefaultFramerate        uint32
	RefAddrFPWeaponPtr             uint32
	RefAddrFogParams               uint32
	RefAddrGameStateBasePtr        uint32
	RefAddrGameStateSize           uint32
	RefAddrGamepadState            uint32
	RefAddrGamepadStateAlt         uint32
	RefAddrGlobalRandomSeed        uint32
	RefAddrGlobalVariant           uint32
	RefAddrHudMessagesPtr          uint32
	RefAddrInputAbstractGlbls      uint32
	RefAddrInputAbstractInputState uint32
	RefAddrItemDatumSize           uint32
	RefAddrLookPitchRate           uint32
	RefAddrLookYawRate             uint32
	RefAddrNetworkGameClient       uint32
	RefAddrNetworkGameServer       uint32
	RefAddrObjectDatumSize         uint32
	RefAddrObjectTypeDefArray      uint32
	RefAddrObjectTypeDefRangeHi    uint32
	RefAddrObserverCameraBase      uint32
	RefAddrPerLocalUIGlobals       uint32
	RefAddrPlayerControlPtr        uint32
	RefAddrRefreshRate             uint32
	RefAddrSoundCacheBasePtr       uint32
	RefAddrSoundCacheSize          uint32
	RefAddrTagCacheBasePtr         uint32
	RefAddrTagCacheSize            uint32
	RefAddrTextureCacheBasePtr     uint32
	RefAddrTextureCacheSize        uint32
	RefAddrUnitDatumSize           uint32
	RefAddrUpdateClientPlayerPtr   uint32
	RefAddrUpdateQueueAdjacent     uint32
	RefAddrUpdateQueueCounterHi    uint32
	RefAddrUpdateQueueCounterLo    uint32
	TagPathGameSettingNames        string
	TagPathGametypeSelectList      string
	TagPathMPMapDescriptions       string
	TagPathMPMapList               string
	TagPathMPMapSelectList         string
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
	s, err := offsets.Baseline("haloce")
	if err != nil {
		panic(fmt.Sprintf("haloce: baseline offsets: %v", err))
	}
	o, err := OffsetsFromSet(s)
	if err != nil {
		panic(fmt.Sprintf("haloce: baseline offsets: %v", err))
	}
	return o
}
