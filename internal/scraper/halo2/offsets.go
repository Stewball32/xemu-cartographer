// Package halo2 implements scraper.GameReader for Halo 2 (NTSC retail, TitleID
// 0x4D530064). It reads live game state from an xemu instance using the W1-W4
// datum-array offset map (halo-offset-mapper/offset-maps/h2-stock.offsets.json,
// 121 runtime-verified offsets) rather than the legacy stats-block globals.
//
// Approach (verified live 2026-07-01, xemu 0.8.136, splitscreen on Turf):
//
//	static .data ptr (low GVA)  ->  s_data_array header (high GVA)
//	    header: signature@0x2C ("@t@d"), active_count@0x38, block_ptr@0x44
//	players[16] (stride 0x21C): index@0x1C team@0x20 unit@0x2C name@0x44(wchar16)
//	unit handle -> object entry table (objects.block_ptr, stride 0xC, data_ptr@0x8)
//	biped: health@0x84 shield@0xA0 maxHealth@0xE4 maxShield@0xE8 pos@0x34 aim@0x150
//
// The legacy reader's stats-block globals (0x8362xxxx: K/D/A, gametype, GRG
// names/team) do NOT resolve under this xemu build — confirmed empty on a live
// read 2026-07-01 — so those fields are stubbed here and marked for
// re-derivation (M20 known-broken list). All offsets below carry their
// h2-stock.offsets.json confidence in a comment; UNVERIFIED markers preserved.
package halo2

// TitleID is the Xbox title certificate ID for Halo 2 (NTSC retail).
// Verified live via XBE cert read (GVA 0x00010000+0x118 -> +0x08) 2026-07-01.
const TitleID uint32 = 0x4D530064

// ---------------------------------------------------------------------------
// Static .data array pointers (low GVAs — translated at Init via LowGVAs()).
// All runtime-verified in h2-stock.offsets.json.
// ---------------------------------------------------------------------------
const (
	AddrH2PlayersArrayPtr     uint32 = 0x4F01F4 // -> players s_data_array
	AddrH2ObjectArrayPtr      uint32 = 0x4E78D0 // -> objects s_data_array
	AddrH2TagHeaderPtr        uint32 = 0x54F24C // -> tag header (@0x80061000)
	AddrH2ScenarioNamePoolPtr uint32 = 0x4EDA3C // -> scenario tag-name pool
)

// AllLowGVAs are the low guest VAs this plugin needs translated at Init time.
var AllLowGVAs = []uint32{
	AddrH2PlayersArrayPtr,
	AddrH2ObjectArrayPtr,
	AddrH2TagHeaderPtr,
	AddrH2ScenarioNamePoolPtr,
}

// ---------------------------------------------------------------------------
// s_data_array header (struct h2_data_array) — offsets from the deref'd ptr.
// The element block pointer is +0x44/+0x48 (NOT the r1e mod's +0x30 stub).
// ---------------------------------------------------------------------------
const (
	OffH2DataArrayMax         uint32 = 0x20 // uint32 max element count
	OffH2DataArrayElemSize    uint32 = 0x24 // uint32 element size
	OffH2DataArraySignature   uint32 = 0x2C // char[4] "@t@d" validity marker
	OffH2DataArrayActiveCount uint32 = 0x38 // uint32 current active count
	OffH2DataArrayBlockPtr    uint32 = 0x44 // uint32 element block START (high GVA)
	OffH2DataArrayBlockEnd    uint32 = 0x48 // uint32 element block END
)

// DataArraySignature is the ASCII marker at OffH2DataArraySignature ("@t@d").
const DataArraySignature = "@t@d"

// ---------------------------------------------------------------------------
// Object entry table (struct h2_object_entry) — element of the objects array.
// ---------------------------------------------------------------------------
const (
	OffH2ObjEntrySalt    uint32 = 0x0 // uint16 salt (== handle>>16)
	OffH2ObjEntryDataPtr uint32 = 0x8 // uint32 -> object data (high GVA)
	ConstH2ObjectMax     uint32 = 0x800
	ConstH2ObjElemSize   uint32 = 0xC
)

// ---------------------------------------------------------------------------
// Player datum (struct h2_player_datum) — element of the players array.
// ---------------------------------------------------------------------------
const (
	OffH2PlrDatumId         uint32 = 0x0   // uint32 datum id (0/0xFFFFFFFF = empty slot)
	OffH2PlrPlayerId        uint32 = 0x4   // uint32 player id
	OffH2PlrIndex           uint32 = 0x1C  // int32  player index
	OffH2PlrTeam            uint32 = 0x20  // int32  team index
	OffH2PlrUnitHandle      uint32 = 0x2C  // uint32 handle -> object table
	OffH2PlrName            uint32 = 0x44  // wchar[16] UTF-16LE gamertag
	ConstH2PlayerRecordSize uint32 = 0x21C // 540
	ConstH2PlayerMax        uint32 = 0x10  // 16
)

// ---------------------------------------------------------------------------
// Object / biped data (struct h2_object_data) — from an entry's data_ptr.
// ---------------------------------------------------------------------------
const (
	OffH2ObjTagId            uint32 = 0x0   // uint32 object tag id
	OffH2BipedHeldWeapon     uint32 = 0x10  // uint32 held weapon handle
	OffH2ObjParent           uint32 = 0x14  // uint32 parent object handle
	OffH2ObjPosition         uint32 = 0x34  // float[3] READ-ONLY RENDER COPY (see notes)
	OffH2ObjForward          uint32 = 0x70  // float[2] facing (x@0x70,y@0x74)
	OffH2BipedHealth         uint32 = 0x84  // float health fraction (1.0=full)
	OffH2ObjVelocity         uint32 = 0x88  // float[3] velocity
	OffH2BipedShield         uint32 = 0xA0  // float shield fraction (1.0=full)
	OffH2ObjType             uint32 = 0xAA  // uint8 object subtype
	OffH2BipedMaxHealth      uint32 = 0xE4  // float max health (45.0 Spartan)
	OffH2BipedMaxShield      uint32 = 0xE8  // float max shield (70.0 Spartan)
	OffH2ObjAim              uint32 = 0x150 // float[3] aim/look (read-only output)
	OffH2BipedCurWeaponSlot  uint32 = 0x212 // uint8 current weapon slot
	OffH2BipedWeaponSlots    uint32 = 0x218 // uint32[4] weapon object handles
	OffH2BipedFragGrenades   uint32 = 0x23E // uint8 frag count
	OffH2BipedPlasmaGrenades uint32 = 0x23F // uint8 plasma count (two-source)
	OffH2BipedZoomLevel      uint32 = 0x240 // int16 zoom level
	OffH2BipedAirborne       uint32 = 0x348 // uint8 airborne flag
)

// NOTE: OffH2ObjPosition (0x34) is the read-only RENDER position copy — writes
// there do NOT move the unit (authoritative physics position is in a physics
// substructure, unmapped). Readable and correct for spectating/overlay.

// ---------------------------------------------------------------------------
// Weapon data (struct h2_weapon_data) — a weapon object's data.
// ---------------------------------------------------------------------------
const (
	OffH2WepReserve uint32 = 0x22A // uint16 reserve ammo
	OffH2WepMag     uint32 = 0x22C // uint16 loaded rounds
)

// ---------------------------------------------------------------------------
// Tag header (struct h2_tag_header) — from AddrH2TagHeaderPtr.
// ---------------------------------------------------------------------------
const (
	OffH2TagHdrInstanceArrayPtr uint32 = 0x8  // uint32 -> tag instance array
	OffH2TagHdrScenarioId       uint32 = 0xC  // uint32 loaded scenario tag id
	OffH2TagHdrGlobalsId        uint32 = 0x10 // uint32 globals tag id
	OffH2TagHdrCount            uint32 = 0x18 // uint32 tag count
	// tag instance (stride 0x10): group@0x0(char4) id@0x4 data_ptr@0x8 size@0xC
	OffH2TagInstGroup    uint32 = 0x0
	OffH2TagInstId       uint32 = 0x4
	OffH2TagInstDataPtr  uint32 = 0x8
	ConstH2TagInstStride uint32 = 0x10
)

// ---------------------------------------------------------------------------
// LEGACY stats-block globals (high GVAs) — DO NOT RESOLVE on this xemu build.
// Confirmed empty on a live read 2026-07-01. Kept for provenance / future
// re-derivation (M20 known-broken: K/D/A, gametype, GRG names/team, events).
// UNVERIFIED — preserved from atlas/xemu-cartographer-legacy per M20.
// ---------------------------------------------------------------------------
const (
	LegacyGVAGameResultsGlobals uint32 = 0x8362AFB0 // GRG (names/gametype) — STALE
	LegacyGVASessionPlayers     uint32 = 0x83691880 // session players (team) — STALE
	LegacyGVAGameStats          uint32 = 0x8362BF02 // K/D/A stride 0x36A — STALE
	LegacyGVAGameStatsExtra     uint32 = 0x8364D014 // K/D/A players 5-15 — STALE
	LegacyGVAVariantInfo        uint32 = 0x83606B50 // scenario path — STALE
	LegacyGVAEventCount         uint32 = 0x8362AFA4 // event count — always 0 (BROKEN)
	LegacyOffGSKills            uint32 = 0x00
	LegacyOffGSAssists          uint32 = 0x02
	LegacyOffGSDeaths           uint32 = 0x04
	LegacyStatStride            uint32 = 0x36A
)
