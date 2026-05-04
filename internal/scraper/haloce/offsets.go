// Package haloce contains the Halo: Combat Evolved (NTSC, title 0x4D530101) memory
// scraper. Offsets here are reconciled against two historical sources:
//
//   - atlas/xemu-cartographer-legacy/internal/scraper/haloce/offsets.go (128 constants)
//   - atlas/HaloCaster/HaloCE/halocaster.py (515 inline hex constants, 2587 LOC)
//
// Reconciliation workbook: see ROADMAP.md M2a. Every hex constant HaloCaster reads
// is captured in this package — active read-path constants live in this file;
// every other corroborated offset lives in offsets_reference.go organized by struct.
//
// Each constant carries an origin tag of the form `// halocaster.py:NNN`. All
// offsets are status `unverified` until M7's runtime sanity-check pass — they are
// believed correct based on two-source agreement but have not been re-verified
// against the current xemu memory layout.
//
// ----------------------------------------------------------------------
// Investigations resolved
// ----------------------------------------------------------------------
// 0x2E4004 vs 0x2E4068 ("main menu active"):
//   AddrMainMenuActive is 0x2E4068 (HC:1428, used in HC's actual logic). The
//   alternate 0x2E4004 read at HC:573 is an exploratory unused global; ignored.
//
// 0x1B4 (camo vs drop_time):
//   Same address read with different semantics depending on object subclass.
//   OffDynCamo is biped-specific (u8: 0x41=no camo, 0x51=active). HC also reads
//   the same offset as a u32 "drop_time" for generic objects (HC:803). Both are
//   valid in their respective contexts; OffDynCamo is biped-only here.
//
// 0x1C arming_time vs target_object_index (projectile):
//   HC reads projectile +0x1C as both target_object_index s32 (HC:818) and
//   arming_time f32 (HC:821). This is a HaloCaster bug — the two cannot share
//   one offset. See offsets_reference.go for the documented overlap.
//
// ----------------------------------------------------------------------
// Skipped (debug-only / known-bad)
// ----------------------------------------------------------------------
//   - 0x80010000 kernel header dump      (HC:722)
//   - 0xD00E82D0 hardcoded packet ptr    (HC:1278; HC notes "changes on restart")
//   - 0x80060220, 0x80060380             (HC:723; commented-out kernel diagnostic)
//   - 0x9c514                            (HC:2461-2463; debug write-bytes target)
//   - 0x106536, 0x10653E, 0x106F3F-FFFB,
//     0x10721B                           (HC:1879-1888; commented-out pgcr_debug)
//   - 0x3e590bb7                         (HC:1923; one-off error message ID)
//   - 0xBB648, 0x1F8C98 framerate cfg    (HC:2139-2140; engine config, not gameplay)
//   - Memory cache base/size pointers    (HC:330-344, 904-911) — Go reads /proc/<pid>/mem
//                                         directly; QMP-cache hack from HC unnecessary.
//                                         (Captured as RefAddrGameStateBasePtr et al. in
//                                         offsets_reference.go for completeness.)
//
// Not promoted to constants (intentionally):
//   - Loop counters / array indices: 0x0..0x9, 0xA..0xF as small literals where HC
//     uses them inline (e.g., `range(4)`, struct slot positions). They re-appear as
//     real offsets in the per-struct sections above when they ARE meaningful.
//   - Bit masks 0x7F (HC:1305 debug print), 0xFC (HC:1726-1730 commented-out
//     animation playback type masks). Not memory addresses or struct offsets.
//   - 0x2E4000 (HC:573, the +4 base of the ignored 0x2E4004 alternate-main-menu
//     read). Resolved to AddrMainMenuActive=0x2E4068; the 0x2E4000 base is unused.
//
package haloce

// All addresses below are Halo: CE guest virtual addresses (GVAs).
// (TitleID lives in game.go alongside the scraper.Register() init.)

// ----------------------------------------------------------------------
// Active read path — pointer globals (low GVA, deref u32 → struct addr)
// ----------------------------------------------------------------------
const (
	AddrPlayerDatumArrayPtr   uint32 = 0x2FAD28 // halocaster.py:558
	AddrPlayersGlobalsPtr     uint32 = 0x2FAD20 // halocaster.py:562
	AddrTeamsPtr              uint32 = 0x2FAD24 // halocaster.py:563
	AddrGameGlobalsPtr        uint32 = 0x27629C // halocaster.py:564
	AddrGlobalGameGlobalsPtr  uint32 = 0x39BE4C // halocaster.py:565
	AddrGameServerPtr         uint32 = 0x2E3628 // halocaster.py:566
	AddrGameClientPtr         uint32 = 0x2E362C // halocaster.py:567
	AddrObjectHeaderDatumPtr  uint32 = 0x2FC6AC // halocaster.py:756
	AddrGameTimeGlobalsPtr    uint32 = 0x2F8CA0 // halocaster.py:570
	AddrGlobalTagInstancesPtr uint32 = 0x39CE24 // halocaster.py:571
	AddrGlobalScenarioPtr     uint32 = 0x39BE5C // halocaster.py:583
	AddrGameEngineGlobalsPtr  uint32 = 0x2F9110 // halocaster.py:843
	AddrGameVariantGlobalPtr  uint32 = 0x2FAB60 // halocaster.py:1192
)

// ----------------------------------------------------------------------
// Active read path — direct value globals (low GVA, value-at-address)
// ----------------------------------------------------------------------
const (
	AddrGameConnection     uint32 = 0x2E3684 // u16 — halocaster.py:568,1449 (0=menu/SP, 1=syslink, 2=hosting, 3=film)
	AddrIsTeamGame         uint32 = 0x2F90C4 // u8  — halocaster.py:569,1899
	AddrMainMenuActive     uint32 = 0x2E4068 // u8  — halocaster.py:1428 (0x2E4004 in HC:573 was unused; ignored)
	AddrGameCanScore       uint32 = 0x2FABF0 // u32 — halocaster.py:1901 (0=can score, non-zero=game over)
	AddrMultiplayerMapName uint32 = 0x2E37CD // null-term ASCII — halocaster.py:1892
	AddrGlobalStageName    uint32 = 0x2FAC20 // null-term ASCII (host only) — halocaster.py:1891
	AddrVariant            uint32 = 0x2F90F4 // u8 variant/mode index — halocaster.py:1890
)

// ----------------------------------------------------------------------
// Active read path — score base addresses by gametype (low GVAs)
// ----------------------------------------------------------------------
const (
	AddrScoreCTF          uint32 = 0x2762B4 // u32[2] red/blue — halocaster.py:1153,1172
	AddrScoreSlayer       uint32 = 0x276710 // u32[16] FFA or u32[2] team — halocaster.py:1154,1175
	AddrScoreOddball      uint32 = 0x27653C // halocaster.py:1155,1178
	AddrScoreKing         uint32 = 0x2762D8 // halocaster.py:1156,1180
	AddrScoreRace         uint32 = 0x2766C8 // halocaster.py:1157,1181
	AddrScoreLimitCTF     uint32 = 0x2762BC // u32 — halocaster.py:1173
	AddrScoreLimitSlayer  uint32 = 0x2F90E8 // u32 — halocaster.py:1177
	AddrScoreLimitOddball uint32 = 0x276538 // u32 — halocaster.py:1179
)

// PlayerScoreBaseOffset is the byte offset from a team-score base address to the
// per-player score table for non-CTF gametypes. Per-player score = read_s32(
// teamScoreAddress + PlayerScoreBaseOffset + 4*playerIndex). CTF per-player
// scores live in the static-player struct at OffPlrCTFScore instead.
//
// Origin: halocaster.py:1162-1165 (player_score_addresses_by_gametype derivation).
const PlayerScoreBaseOffset uint32 = 64

// AllLowGVAs is the set of low guest VAs the M2 reader pre-translates at
// Instance.Init time. Returned by Reader.LowGVAs().
//
// As of the "scrape every non-Broken offset" pass, every low-GVA pointer/value
// global declared in this package is pre-translated at startup. Cache-pointer
// addresses (RefAddrGameStateBasePtr et al.) are diagnostic-only but cheap to
// pre-translate alongside the rest.
var AllLowGVAs = []uint32{
	// Pointer globals
	AddrPlayerDatumArrayPtr,
	AddrPlayersGlobalsPtr,
	AddrTeamsPtr,
	AddrGameGlobalsPtr,
	AddrGlobalGameGlobalsPtr,
	AddrGameServerPtr,
	AddrGameClientPtr,
	AddrObjectHeaderDatumPtr,
	AddrGameTimeGlobalsPtr,
	AddrGlobalTagInstancesPtr,
	AddrGlobalScenarioPtr,
	AddrGameEngineGlobalsPtr,
	AddrGameVariantGlobalPtr,
	// Direct value globals
	AddrGameConnection,
	AddrIsTeamGame,
	AddrMainMenuActive,
	AddrGameCanScore,
	AddrMultiplayerMapName,
	AddrGlobalStageName,
	AddrVariant,
	// Score bases
	AddrScoreCTF,
	AddrScoreSlayer,
	AddrScoreOddball,
	AddrScoreKing,
	AddrScoreRace,
	AddrScoreLimitCTF,
	AddrScoreLimitSlayer,
	AddrScoreLimitOddball,
	// Reference low-GVA pointer globals (added during full-scrape pass)
	RefAddrHudMessagesPtr,
	RefAddrPlayerControlPtr,
	RefAddrUpdateClientPlayerPtr,
	RefAddrFPWeaponPtr,
	RefAddrInputAbstractGlbls,
	RefAddrCTFFlag0Ptr,
	RefAddrCTFFlag1Ptr,
	RefAddrUpdateQueueCounterLo,
	RefAddrUpdateQueueCounterHi,
	RefAddrUpdateQueueAdjacent,
	// Reference low-GVA direct-value globals
	RefAddrPerLocalUIGlobals,
	RefAddrInputAbstractInputState,
	RefAddrGamepadStateAlt,
	RefAddrGamepadState,
	RefAddrLookYawRate,
	RefAddrLookPitchRate,
	RefAddrObserverCameraBase,
	RefAddrNetworkGameClient,
	RefAddrNetworkGameServer,
	RefAddrFogParams,
	RefAddrGlobalVariant,
	RefAddrGlobalRandomSeed,
	RefAddrObjectTypeDefArray,
	RefAddrObjectDatumSize,
	RefAddrUnitDatumSize,
	RefAddrItemDatumSize,
	RefAddrObjectTypeDefRangeLo,
	RefAddrObjectTypeDefRangeHi,
	RefAddrDefaultFramerate,
	RefAddrRefreshRate,
	// Memory cache pointers (diagnostic only)
	RefAddrGameStateBasePtr,
	RefAddrTagCacheBasePtr,
	RefAddrTextureCacheBasePtr,
	RefAddrSoundCacheBasePtr,
	RefAddrGameStateSize,
	RefAddrTagCacheSize,
	RefAddrTextureCacheSize,
	RefAddrSoundCacheSize,
}

// ----------------------------------------------------------------------
// GameTimeGlobals struct (at *AddrGameTimeGlobalsPtr)
// ----------------------------------------------------------------------
const (
	OffGTGInitialized      uint32 = 0x00 // u8  — halocaster.py:701,1419
	OffGTGActive           uint32 = 0x01 // u8  — halocaster.py:702,1420
	OffGTGPaused           uint32 = 0x02 // u8  — halocaster.py:703,1421
	OffGTGMonitorState     uint32 = 0x04 // s16 (diag) — halocaster.py:704
	OffGTGMonitorCounter   uint32 = 0x06 // s16 (diag) — halocaster.py:705
	OffGTGMonitorLatency   uint32 = 0x08 // s16 (diag) — halocaster.py:706
	OffGTGGameTime         uint32 = 0x0C // u32 ticks (30Hz) — halocaster.py:707,1415
	OffGTGElapsed          uint32 = 0x10 // u32 — halocaster.py:708,1416
	OffGTGSpeed            uint32 = 0x18 // f32 (1.0 = normal speed) — halocaster.py:709,1422
	OffGTGLeftoverDeltaTime uint32 = 0x1C // f32 (diag) — halocaster.py:710,1423
)

// ----------------------------------------------------------------------
// GameEngineGlobals struct (at *AddrGameEngineGlobalsPtr)
// Pointer is 0 in pregame lobby.
// ----------------------------------------------------------------------
const (
	OffGEGGametype uint32 = 0x04 // u32 gametype ID (1–7) — halocaster.py:1889
)

// ----------------------------------------------------------------------
// GlobalVariant struct (static base at RefAddrGlobalVariant 0x2F90A8;
// mirrored at AddrGameVariantGlobalPtr 0x2FAB60 during active matches).
// Treated as a struct base, NOT a pointer — read directly via the
// Init-time-translated HVA (no QMP indirection). The first 24 bytes are
// the variant name as UTF-16-LE. Updated only at match-start, so the
// gametype field here is authoritative once a match is running.
//
// Confirmed via probe captures: CTF variant "CTF 3C 10S" → +0x18 = 1,
// Slayer "TS TRAINING" → 2, Oddball "Accumulate" → 3.
// ----------------------------------------------------------------------
const (
	OffGVGametype uint32 = 0x18 // u32 gametype ID (1–7)
)

// ----------------------------------------------------------------------
// PlayerDatumArray header (at *AddrPlayerDatumArrayPtr)
// ----------------------------------------------------------------------
const (
	OffPDAMaxCount     uint32 = 0x20 // u16 — halocaster.py:559
	OffPDAElementSize  uint32 = 0x22 // u16 (typically 0xD4=212) — halocaster.py:560
	OffPDACurrentCount uint32 = 0x2E // u16 — halocaster.py:1409
	OffPDAFirstElement uint32 = 0x34 // u32 — halocaster.py:561
)

// ----------------------------------------------------------------------
// Static player struct (at firstElement + index*elementSize)
// ----------------------------------------------------------------------
const (
	OffPlrLocalIndex        uint32 = 0x02 // s16: -1 if remote — halocaster.py:1816
	OffPlrName              uint32 = 0x04 // [24] UTF-16LE, 12 chars — halocaster.py:1821
	OffPlrTeam              uint32 = 0x20 // u32 (0=red, 1=blue, 0..15=ffa) — halocaster.py:1824
	OffPlrActionTarget      uint32 = 0x24 // u32 object handle for next interaction (-1 on spawn) — halocaster.py:1825
	OffPlrAction            uint32 = 0x28 // u16 (6=over weapon, 7=only-1-held, 8=near vehicle, 0=none) — halocaster.py:1826
	OffPlrActionSeat        uint32 = 0x2A // u16 — halocaster.py:1827
	OffPlrRespawnTimer      uint32 = 0x2C // u32 countdown ticks — halocaster.py:1828
	OffPlrRespawnPenalty    uint32 = 0x30 // u32 — halocaster.py:1829
	OffPlrObjectHandle      uint32 = 0x34 // s32: -1 when dead — halocaster.py:1830
	OffPlrObjectID          uint32 = 0x36 // u16 (high 16 bits of OffPlrObjectHandle = object array index) — halocaster.py:1832
	OffPlrPrevObjHandle     uint32 = 0x38 // s32: prev object handle (for death-tick reads) — halocaster.py:1833
	OffPlrLastTargetObjRef  uint32 = 0x40 // u32 — halocaster.py:1834
	OffPlrTimeLastShot      uint32 = 0x44 // u32 game tick of last shot — halocaster.py:1835
	OffPlrCamoTimer         uint32 = 0x68 // u32 — halocaster.py:1837
	OffPlrPlayerSpeed       uint32 = 0x6C // f32 — halocaster.py:1836
	OffPlrTimeLastDeath     uint32 = 0x84 // u32 game tick (0 at start of game) — halocaster.py:1531,1838
	OffPlrTargetPlayerIndex uint32 = 0x88 // u32 — halocaster.py:1839
	OffPlrKillStreak        uint32 = 0x92 // u16: resets to 0 on death — halocaster.py:1840
	OffPlrMultikill         uint32 = 0x94 // u16: resets to 0 on death — halocaster.py:1841
	OffPlrTimeLastKill      uint32 = 0x96 // s16 game ticks; -1 on death — halocaster.py:1842
	OffPlrKills             uint32 = 0x98 // s16 — halocaster.py:1843
	OffPlrAssists           uint32 = 0xA0 // s16 — halocaster.py:1844
	OffPlrTeamKills         uint32 = 0xA8 // s16 — halocaster.py:1845
	OffPlrDeaths            uint32 = 0xAA // s16 — halocaster.py:1846
	OffPlrSuicides          uint32 = 0xAC // s16 — halocaster.py:1847
	OffPlrShotsFired        uint32 = 0xAE // s32 — halocaster.py:1848
	OffPlrShotsHit          uint32 = 0xB2 // s16 — halocaster.py:1849
	OffPlrCTFScore          uint32 = 0xC4 // s16 — halocaster.py:1851
	OffPlrQuit              uint32 = 0xD1 // u8: 1 = player quit — halocaster.py:1852
)

// ----------------------------------------------------------------------
// ObjectHeaderDatumArray header (at *AddrObjectHeaderDatumPtr)
// ----------------------------------------------------------------------
const (
	OffOHDMaxElements  uint32 = 0x20 // u16 — halocaster.py:1455
	OffOHDElementSize  uint32 = 0x22 // u16 (typically 12) — halocaster.py:1456
	OffOHDAllocCount   uint32 = 0x2E // u16: iterate this many entries — halocaster.py:757,1457
	OffOHDElementCount uint32 = 0x30 // u16 — halocaster.py:1458
	OffOHDFirstElement uint32 = 0x34 // u32: re-read every tick — halocaster.py:758,1459
)

// ----------------------------------------------------------------------
// Object header entry (stride = element_size = 12)
// ----------------------------------------------------------------------
const (
	OffObjEntryDataAddr uint32 = 0x08 // u32: object_data_addr (0 if slot empty) — halocaster.py:771,1474
)

// ----------------------------------------------------------------------
// Common object data offsets (at object_data_addr, all object types)
// ----------------------------------------------------------------------
const (
	OffObjTagIndex uint32 = 0x00    // s16 tag index (low 16 bits) — halocaster.py:776
	OffObjFlags    uint32 = 0x04    // u32 (&0x10000=garbage, &0x8=connected_to_map, &0x1=vehicle weapon) — halocaster.py:786,1633
	OffObjX        uint32 = 0x0C    // f32 — halocaster.py:787
	OffObjY        uint32 = 0x10    // f32 — halocaster.py:788
	OffObjZ        uint32 = 0x14    // f32 — halocaster.py:789
	OffObjType     uint32 = 0x64    // u8 (0=biped,1=vehicle,2=weapon,3=equipment,...) — halocaster.py:778
	ObjFlagGarbage uint32 = 0x10000 // halocaster.py:1633
)

// ----------------------------------------------------------------------
// Dynamic player / biped object (at object_data_addr, type=0)
// ----------------------------------------------------------------------
const (
	OffDynX               uint32 = 0x0C  // f32 — halocaster.py:1634
	OffDynY               uint32 = 0x10  // f32 — halocaster.py:1635
	OffDynZ               uint32 = 0x14  // f32 — halocaster.py:1636
	OffDynVelX            uint32 = 0x18  // f32 — halocaster.py:1637
	OffDynVelY            uint32 = 0x1C  // f32 — halocaster.py:1638
	OffDynVelZ            uint32 = 0x20  // f32 — halocaster.py:1639
	OffDynMaxHealth       uint32 = 0x88  // f32 — halocaster.py:1662
	OffDynMaxShields      uint32 = 0x8C  // f32 — halocaster.py:1663
	OffDynHealth          uint32 = 0x90  // f32 — halocaster.py:1664
	OffDynShields         uint32 = 0x94  // f32 — halocaster.py:1665
	OffDynShieldsStatus   uint32 = 0xB6  // u16 — halocaster.py:1676
	OffDynParentObject    uint32 = 0xCC  // u32: vehicle handle; 0xFFFFFFFF=on foot — halocaster.py:1682
	OffDynCamo            uint32 = 0x1B4 // u8 (biped): 0x41=no camo, 0x51=active camo — halocaster.py:1686
	OffDynCurrentAction   uint32 = 0x1B8 // u32 bitfield — halocaster.py:1688
	OffDynAimX            uint32 = 0x1EC // f32 aiming_vector x — halocaster.py:1709
	OffDynAimY            uint32 = 0x1F0 // f32 aiming_vector y — halocaster.py:1710
	OffDynAimZ            uint32 = 0x1F4 // f32 aiming_vector z — halocaster.py:1711
	OffDynSelectedSlot    uint32 = 0x2A2 // s16: 0=primary, 1=secondary, -1=none — halocaster.py:1735
	OffDynWeaponSlot0     uint32 = 0x2A8 // u32 handle — halocaster.py:1744
	OffDynWeaponSlot1     uint32 = 0x2AC // u32 handle
	OffDynWeaponSlot2     uint32 = 0x2B0 // u32 handle
	OffDynWeaponSlot3     uint32 = 0x2B4 // u32 handle
	OffDynFrags           uint32 = 0x2CE // u8 — halocaster.py:1751
	OffDynPlasmas         uint32 = 0x2CF // u8 — halocaster.py:1752
	OffDynZoomLevel       uint32 = 0x2D0 // s8 (read as u8, cast to int8) — halocaster.py:1753
	OffDynCamoAmount      uint32 = 0x32C // f32 (0=nocamo, 1=fullcamo) — halocaster.py:1755
	OffDynDamageTable     uint32 = 0x3E0 // 4-slot damage history — halocaster.py:1509
	OffDynMeleeRemaining  uint32 = 0x45D // u8 — halocaster.py:1785
	OffDynMeleeDamageTick uint32 = 0x45E // u8: equals 0x45D when melee impacts — halocaster.py:1786
	OffDynCrouchScale     uint32 = 0x464 // f32 (0.0=standing, 1.0=crouching) — halocaster.py:1627,1763
)

// current_action bitfield masks (OffDynCurrentAction).
// Origin: halocaster.py:1688-1697 inline comments.
const (
	ActionCrouch      uint32 = 0x0001
	ActionJump        uint32 = 0x0002
	ActionFire        uint32 = 0x0008
	ActionFlashlight  uint32 = 0x0010
	ActionPressAction uint32 = 0x0440
	ActionShooting    uint32 = 0x0800
	ActionGrenade     uint32 = 0x2FC4
	ActionHoldAction  uint32 = 0x4000
)

// ----------------------------------------------------------------------
// Damage table (at dynPlayerAddr + OffDynDamageTable), 4 entries × 16 bytes
// ----------------------------------------------------------------------
const (
	DamageTableSlots          = 4
	DamageEntrySize           = 16
	OffDmgTime         uint32 = 0x00 // u32 game tick; 0xFFFFFFFF=empty — halocaster.py:1513
	OffDmgAmount       uint32 = 0x04 // f32 — halocaster.py:1515
	OffDmgDealerObjHdl uint32 = 0x08 // u32 dynamic object handle — halocaster.py:1522
	OffDmgDealerPlrHdl uint32 = 0x0C // u32 static player handle (&0xFFFF=index) — halocaster.py:1516
)

// DamageEmptySentinel is the OffDmgTime value indicating an empty damage slot.
// Origin: halocaster.py:1514.
const DamageEmptySentinel uint32 = 0xFFFFFFFF

// ----------------------------------------------------------------------
// Weapon object offsets (at object_data_addr, type=2)
// ----------------------------------------------------------------------
const (
	OffWepIsReloading uint32 = 0x258 // u8 (1 while reloading until reload_time hits 2) — halocaster.py:1589
	OffWepCanFire     uint32 = 0x259 // u8 — halocaster.py:1590
	OffWepReloadTime  uint32 = 0x25A // s16 countdown ticks — halocaster.py:1591
	OffWepAmmoPack    uint32 = 0x25E // s16 reserve ammo — halocaster.py:1592
	OffWepAmmoMag     uint32 = 0x260 // s16 current magazine — halocaster.py:1593
	OffWepCharge      uint32 = 0xF0  // f32 energy remaining (0–1) — halocaster.py:1588
	OffWepEnergyUsed  uint32 = 0x1F0 // f32: 1.0 = depleted energy weapon — halocaster.py:1597
)

// ----------------------------------------------------------------------
// Weapon tag data offsets (at tag_data_ptr, accessed via tag instance array)
// ----------------------------------------------------------------------
const (
	OffWepTagWeaponType uint32 = 0x309 // u8: &0x8 = energy weapon — halocaster.py:1578
)

// EnergyWeaponMask bit-tests OffWepTagWeaponType. Origin: halocaster.py:1579.
const EnergyWeaponMask uint8 = 0x8

// ----------------------------------------------------------------------
// Biped tag data offsets (at biped tag_data_ptr)
// ----------------------------------------------------------------------
const (
	OffBipedTagCamHeightStanding  uint32 = 0x400 // f32 — halocaster.py:1625
	OffBipedTagCamHeightCrouching uint32 = 0x404 // f32 — halocaster.py:1626
)

// ----------------------------------------------------------------------
// Tag instance array (at *AddrGlobalTagInstancesPtr, stride 32 bytes per entry)
// ----------------------------------------------------------------------
const (
	TagInstStride        = 32
	OffTagNamePtr uint32 = 0x10 // u32 → null-terminated tag name string — halocaster.py:654
	OffTagDataPtr uint32 = 0x14 // u32 → raw tag data struct — halocaster.py:655,1577
)

// ----------------------------------------------------------------------
// Scenario item spawn offsets (at *AddrGlobalScenarioPtr)
// ----------------------------------------------------------------------
const (
	OffScenarioItemCount uint32 = 900  // s32 — halocaster.py:631
	OffScenarioItemFirst uint32 = 904  // u32 — halocaster.py:632
	ScenarioItemStride          = 144  // bytes per entry — halocaster.py:641
	OffScenItemUnknownAttr uint32 = 0x0E // s16 (HC uses as filter) — halocaster.py:642
	OffScenItemGameType  uint32 = 0x04 // u8 gametype-restricted spawn flag — halocaster.py:663
	OffScenItemTagIndex  uint32 = 0x5C // s32 tag index (-1 if empty) — halocaster.py:648
	OffScenItemX         uint32 = 0x40 // f32 — halocaster.py:664
	OffScenItemY         uint32 = 0x44 // f32 — halocaster.py:665
	OffScenItemZ         uint32 = 0x48 // f32 — halocaster.py:666
	// Spawn interval lookup: read_u32(tagDataPtr + OffTagDataPtr) → base; read_s16(base + 0x0C)
	OffTagRespawnIntervalOff uint32 = 0x14 // u32 at tag_data → pointer to interval table — halocaster.py:655
	OffTagRespawnInterval    uint32 = 0x0C // s16 within interval table
)

// ----------------------------------------------------------------------
// Common sentinel / magic values used across reads
// ----------------------------------------------------------------------
const (
	HandleEmpty       uint32 = 0xFFFFFFFF // handle "no object" — halocaster.py:1505
	HandleIndexMask   uint32 = 0x0000FFFF // handle & 0xFFFF = array index — halocaster.py:1471
	PregameSentinel   uint32 = 0xDEADBEEF // game_globals+0x10 during pregame — halocaster.py:1933
	HighGVAThreshold  uint32 = 0x80000000 // GVA threshold; ≥ this = direct heap, < this = needs translation — halocaster.py:485
	CamoStateNo       uint8  = 0x41       // OffDynCamo value when no camo — halocaster.py:1686
	CamoStateActive   uint8  = 0x51       // OffDynCamo value when active camo — halocaster.py:1686
)

// Shield-status u16 values at OffDynShieldsStatus (halocaster.py:1676).
const (
	ShieldsStatusNormal     uint16 = 0x0000
	ShieldsStatusDepleted   uint16 = 0x0008 // shields fully depleted
	ShieldsStatusOvershield uint16 = 0x0010 // overshield charging
	ShieldsStatusCharging   uint16 = 0x1000 // shields charging
	// HaloCaster also documents observed composite values at the same offset.
	// HC's two comments (HC:1675 vs HC:1676) disagree on which mask bit means
	// "shields charging" — the simpler bits above are HC:1676's authoritative
	// list; these composites are from HC:1675 observation.
	ShieldsStatusObservedCharging   uint16 = 0x4096 // halocaster.py:1675 (composite: shields charging, observed)
	ShieldsStatusObservedOvershield uint16 = 0x4112 // halocaster.py:1675 (composite: overshield charging, observed)
)

// ============================================================================
// Reference low-GVA pointer globals (consumed by extended subsystems)
// ============================================================================
const (
	RefAddrHudMessagesPtr        uint32 = 0x276B40 // halocaster.py:572 — read_u32 → HUD message table base, stride 0x460
	RefAddrPlayerControlPtr      uint32 = 0x276794 // halocaster.py:944 — read_u32 → per-local control struct base
	RefAddrUpdateClientPlayerPtr uint32 = 0x2E8870 // halocaster.py:945,1287 — read_u32 → update queue (input replication)
	RefAddrFPWeaponPtr           uint32 = 0x276B48 // halocaster.py:1067 — read_u32 → first-person weapon array, stride 7840 per local
	RefAddrInputAbstractGlbls    uint32 = 0x2E45A0 // halocaster.py:1041 — read_u32 → input_abstraction_globals
	RefAddrCTFFlag0Ptr           uint32 = 0x2762A4 // halocaster.py:845 — read_u32 → flag-base position float triple
	RefAddrCTFFlag1Ptr           uint32 = 0x2762A8 // halocaster.py:846 (= 0x2762A4+4) — read_u32 → flag-base position float triple
	RefAddrUpdateQueueCounterLo  uint32 = 0x2E87E4 // halocaster.py:711,1277 — paired counter
	RefAddrUpdateQueueCounterHi  uint32 = 0x2E87E8 // halocaster.py:711,1299 — diff = max actions allowed this tick
	RefAddrUpdateQueueAdjacent   uint32 = 0x2E8874 // halocaster.py:1308 — adjacent counter (debug only)
)

// ============================================================================
// Reference low-GVA direct-value globals
// ============================================================================
const (
	RefAddrPerLocalUIGlobals       uint32 = 0x2E40D0 // halocaster.py:922 — base; stride 56 per local player
	RefAddrInputAbstractInputState uint32 = 0x2E4600 // halocaster.py:963 — base; stride 0x1C per local player
	RefAddrGamepadStateAlt         uint32 = 0x276AFC // halocaster.py:983 — alt gamepad ptr
	RefAddrGamepadState            uint32 = 0x276A5C // halocaster.py:984 — gamepad state base; stride 0x28 per player
	RefAddrLookYawRate             uint32 = 0x2E4684 // halocaster.py:1039 — f32; stride 4 per local
	RefAddrLookPitchRate           uint32 = 0x2E4694 // halocaster.py:1040 — f32; stride 4 per local
	RefAddrObserverCameraBase      uint32 = 0x271550 // halocaster.py:340,1087,1915 — base; stride 668 (167*4) per local
	RefAddrNetworkGameClient       uint32 = 0x2FB180 // halocaster.py:1242,1276 — network_game_client struct
	RefAddrNetworkGameServer       uint32 = 0x2FBE40 // halocaster.py:1262 — network_game_server struct
	RefAddrFogParams               uint32 = 0x2FC8A8 // halocaster.py:864,867 — fog_params base
	RefAddrGlobalVariant           uint32 = 0x2F90A8 // halocaster.py:1187 — global_variant container
	RefAddrGlobalRandomSeed        uint32 = 0x2E3648 // halocaster.py:1932 — u32 RNG seed
	RefAddrObjectTypeDefArray      uint32 = 0x1FCB78 // halocaster.py:734,742 — u32[] table of object-type def pointers
	RefAddrObjectDatumSize         uint32 = 0x1FC0E0 // halocaster.py:765 — u16 base object struct size
	RefAddrUnitDatumSize           uint32 = 0x1FC188 // halocaster.py:766 — u16 unit subclass size
	RefAddrItemDatumSize           uint32 = 0x1FC380 // halocaster.py:767 — u16 item subclass size
	RefAddrObjectTypeDefRangeLo    uint32 = 0x1FC0D0 // halocaster.py:344 — object-type defs cache range start
	RefAddrObjectTypeDefRangeHi   uint32 = 0x1FCBA4 // halocaster.py:344 — object-type defs cache range end
	RefAddrDefaultFramerate        uint32 = 0xBB648  // halocaster.py:2139 — engine config (cosmetic)
	RefAddrRefreshRate             uint32 = 0x1F8C98 // halocaster.py:2140 — engine config (cosmetic)
)

// HudMessageStride is the per-message stride within the table at *RefAddrHudMessagesPtr.
// Origin: halocaster.py:729.
const HudMessageStride uint32 = 0x460

// ============================================================================
// game_globals struct (at *AddrGameGlobalsPtr) — extended fields
// ============================================================================
const (
	OffGGMapLoaded             uint32 = 0x00 // u8 — halocaster.py:1425
	OffGGActive                uint32 = 0x01 // u8 — halocaster.py:1426
	OffGGPlayersAreDoubleSpeed uint32 = 0x02 // u8 — halocaster.py:1918
	OffGGGameLoadingInProgress uint32 = 0x03 // u8 — halocaster.py:1919
	OffGGPrecacheMapStatus     uint32 = 0x04 // f32 — halocaster.py:1920
	OffGGGameDifficultyLevel   uint32 = 0x0E // u8 — halocaster.py:1921
	OffGGStoredGlobalRandom    uint32 = 0x10 // u32 (0xdeadbeef during pregame/mapselect) — halocaster.py:1933
)

// ============================================================================
// players_globals struct (at *AddrPlayersGlobalsPtr) — extended fields
// ============================================================================
const (
	OffPGLocalPlayerCount uint32 = 0x24 // u16 — halocaster.py:1906
)

// ============================================================================
// Dynamic player / biped object — extended diagnostic fields
// All HC line numbers from halocaster.py:1632-1797.
// ============================================================================
const (
	// Leg / facing rotations (HC:1640-1645)
	OffDynLegsPitch uint32 = 0x24 // f32 — halocaster.py:1640
	OffDynLegsYaw   uint32 = 0x28 // f32 — halocaster.py:1641
	OffDynLegsRoll  uint32 = 0x2C // f32 — halocaster.py:1642
	OffDynPitch1    uint32 = 0x30 // f32 (HC: "0,0,1 in most cases") — halocaster.py:1643
	OffDynYaw1      uint32 = 0x34 // f32 — halocaster.py:1644
	OffDynRoll1     uint32 = 0x38 // f32 — halocaster.py:1645

	// Angular velocity (HC:1646-1648)
	OffDynAngVelX uint32 = 0x3C // f32 — halocaster.py:1646
	OffDynAngVelY uint32 = 0x40 // f32 — halocaster.py:1647
	OffDynAngVelZ uint32 = 0x44 // f32 — halocaster.py:1648

	// Aim-assist sphere (HC:1649-1652)
	OffDynAimAssistSphereX      uint32 = 0x50 // f32 — halocaster.py:1649
	OffDynAimAssistSphereY      uint32 = 0x54 // f32 — halocaster.py:1650
	OffDynAimAssistSphereZ      uint32 = 0x58 // f32 — halocaster.py:1651
	OffDynAimAssistSphereRadius uint32 = 0x5C // f32 — halocaster.py:1652

	// Object scale + sub-type (HC:1653-1657)
	OffDynScale           uint32 = 0x60 // f32 (items only?) — halocaster.py:1653
	OffDynTypeU16         uint32 = 0x64 // u16 (read as u16; OffObjType reads as u8 at same addr) — halocaster.py:1654
	OffDynRenderFlags     uint32 = 0x66 // u16 — halocaster.py:1655
	OffDynWeaponOwnerTeam uint32 = 0x68 // s16 (weapon-only context) — halocaster.py:1656
	OffDynPowerupUnk2     uint32 = 0x6A // s16 — halocaster.py:1657
	OffDynIdleTicks       uint32 = 0x6C // s16 (overlaps with OffPlrPlayerSpeed in different struct) — halocaster.py:1658

	// Animation handle / id / tick (HC:1659-1661, 1734)
	OffDynAnimationUnk1 uint32 = 0x7C // u32 — halocaster.py:1659,1734
	OffDynAnimationUnk2 uint32 = 0x80 // s16 — halocaster.py:1660,1734
	OffDynAnimationUnk3 uint32 = 0x82 // s16 — halocaster.py:1661,1734

	// Damage countdowns (HC:1666-1671)
	OffDynDmgCountdown_98 uint32 = 0x98 // f32 (counts down immediately on damage) — halocaster.py:1666
	OffDynDmgCountdown_9C uint32 = 0x9C // f32 — halocaster.py:1667
	OffDynDmgCountdown_A4 uint32 = 0xA4 // f32 (delayed countdown) — halocaster.py:1668
	OffDynDmgCountdown_A8 uint32 = 0xA8 // f32 — halocaster.py:1669
	OffDynDmgCounter_AC   uint32 = 0xAC // s32 (-1 normally, ramps up on damage) — halocaster.py:1670
	OffDynDmgCounter_B0   uint32 = 0xB0 // s32 — halocaster.py:1671

	// Shields (extended)
	OffDynShieldsStatus2     uint32 = 0xB2 // u16 (HC commented-out duplicate read) — halocaster.py:1672
	OffDynShieldsChargeDelay uint32 = 0xB4 // u16 — halocaster.py:1673

	// Object-table linkage
	OffDynNextObject  uint32 = 0xC4 // s32 — halocaster.py:1679
	OffDynNextObject2 uint32 = 0xC8 // u32 — halocaster.py:1680

	// Generic object camo overlap — same address as OffDynCamo (biped) but read u32
	OffDynStateFlags uint32 = 0x1A4 // u8 — halocaster.py:1802
	OffDynDropTime   uint32 = 0x1B4 // u32 (generic-object semantic; biped reads OffDynCamo here) — halocaster.py:803

	// Flashlight + stunned
	OffDynFlashlight uint32 = 0x1B6 // u8 — halocaster.py:1687
	OffDynStunned    uint32 = 0x3D4 // f32 (HC notes "this isn't actually stunned") — halocaster.py:1699

	// Aim / look unit vectors (HC:1703-1720)
	OffDynXunk0          uint32 = 0x1D4 // f32 — halocaster.py:1703
	OffDynYunk0          uint32 = 0x1D8 // f32 — halocaster.py:1704
	OffDynZunk0          uint32 = 0x1DC // f32 — halocaster.py:1705
	OffDynXAimA          uint32 = 0x1E0 // f32 unit vector — halocaster.py:1706
	OffDynYAimA          uint32 = 0x1E4 // f32 unit vector — halocaster.py:1707
	OffDynZAimA          uint32 = 0x1E8 // f32 unit vector — halocaster.py:1708
	OffDynXAim0          uint32 = 0x1F8 // f32 (projectile aim) — halocaster.py:1712
	OffDynYAim0          uint32 = 0x1FC // f32 — halocaster.py:1713
	OffDynZAim0          uint32 = 0x200 // f32 — halocaster.py:1714
	OffDynXAim1          uint32 = 0x204 // f32 — halocaster.py:1715
	OffDynYAim1          uint32 = 0x208 // f32 — halocaster.py:1716
	OffDynZAim1          uint32 = 0x20C // f32 — halocaster.py:1717
	OffDynLookingVectorX uint32 = 0x210 // f32 — halocaster.py:1718
	OffDynLookingVectorY uint32 = 0x214 // f32 — halocaster.py:1719
	OffDynLookingVectorZ uint32 = 0x218 // f32 — halocaster.py:1720

	// Movement throttles (HC:1721-1723)
	OffDynMoveForward uint32 = 0x228 // f32 — halocaster.py:1721
	OffDynMoveLeft    uint32 = 0x22C // f32 — halocaster.py:1722
	OffDynMoveUp      uint32 = 0x230 // f32 (banshee/observer?) — halocaster.py:1723

	// Melee + animation tags (HC:1731-1733)
	OffDynMeleeDamageType uint32 = 0x239 // u8 (4=continuous melee, 3=impact, 0=players) — halocaster.py:1731
	OffDynAnimation1      uint32 = 0x253 // u8 — halocaster.py:1732
	OffDynAnimation2      uint32 = 0x254 // u8 — halocaster.py:1733

	// Equipment / camo extended
	OffDynCurrentEquipment uint32 = 0x2C8 // u32 — halocaster.py:1750
	OffDynCamoSelfRevealed uint32 = 0x3D2 // u16 (set when camo revealed by shooting) — halocaster.py:1759

	// Facing vectors (HC:1766-1768)
	OffDynFacing1 uint32 = 0x46C // f32 — halocaster.py:1766
	OffDynFacing2 uint32 = 0x470 // f32 — halocaster.py:1767
	OffDynFacing3 uint32 = 0x474 // f32 — halocaster.py:1768

	// Air / landing (HC:1776-1790)
	OffDynAirborne                   uint32 = 0x424 // u8 (&1=airborne, &2=slipping) — halocaster.py:1776
	OffDynLandingStunCurrentDuration uint32 = 0x428 // u8 — halocaster.py:1777
	OffDynLandingStunTargetDuration  uint32 = 0x429 // u8 (typically 30 max) — halocaster.py:1778
	OffDynAirborneTicks              uint32 = 0x459 // u8 — halocaster.py:1779
	OffDynSlippingTicks              uint32 = 0x45A // u8 — halocaster.py:1782
	OffDynStopTicks                  uint32 = 0x45B // u8 — halocaster.py:1783
	OffDynJumpRecoveryTimer          uint32 = 0x45C // u8 — halocaster.py:1784
	OffDynLanding                    uint32 = 0x45F // u16 — halocaster.py:1788
	OffDynAirState460                uint32 = 0x460 // s16 (-1=walking, 0=landing, 1=fall damage) — halocaster.py:1790
)

// ============================================================================
// Biped tag data — extended fields (HC:1795-1796)
// ============================================================================
const (
	OffBipedTagFlags             uint32 = 0x2F4 // u32 — halocaster.py:1795
	OffBipedTagAutoaimPillRadius uint32 = 0x458 // f32 — halocaster.py:1796
)

// ============================================================================
// Weapon object — extended fields (HC:1583-1597)
// ============================================================================
const (
	// HC-commented-out positions (not actively read; kept for completeness)
	OffWepObjX uint32 = 0x50 // f32 (HC commented) — halocaster.py:1583
	OffWepObjY uint32 = 0x54 // f32 (HC commented) — halocaster.py:1584
	OffWepObjZ uint32 = 0x58 // f32 (HC commented) — halocaster.py:1585

	OffWepHeatMeter   uint32 = 0xD4  // f32 — halocaster.py:1586
	OffWepUsedEnergy  uint32 = 0xE0  // f32 (energy weapons only) — halocaster.py:1587
	OffWepOwnerHandle uint32 = 0x1E0 // u32 (HC notes "not really owner; correlates to current action") — halocaster.py:1595
)

// ============================================================================
// Weapon tag data — zoom + auto-aim parameters (HC:1600-1607)
// ============================================================================
const (
	OffWepTagZoomLevels     uint32 = 986  // s16 — halocaster.py:1600
	OffWepTagZoomMin        uint32 = 988  // f32 — halocaster.py:1601
	OffWepTagZoomMax        uint32 = 992  // f32 — halocaster.py:1602
	OffWepTagAutoaimAngle   uint32 = 996  // f32 radians — halocaster.py:1603
	OffWepTagAutoaimRange   uint32 = 1000 // f32 — halocaster.py:1604
	OffWepTagMagnetismAngle uint32 = 1004 // f32 — halocaster.py:1605
	OffWepTagMagnetismRange uint32 = 1008 // f32 — halocaster.py:1606
	OffWepTagDeviationAngle uint32 = 1012 // f32 — halocaster.py:1607
)

// ============================================================================
// Object header datum — common object offsets HC reads but reader.go doesn't yet
// (HC:790-803 from get_objects())
// ============================================================================
const (
	OffObjHeaderDataLen  uint32 = 12    // header_data byte length (entry stride) — halocaster.py:785
	OffObjAngVelX        uint32 = 0x3C  // f32 — halocaster.py:793
	OffObjAngVelY        uint32 = 0x40  // f32 — halocaster.py:794
	OffObjAngVelZ        uint32 = 0x44  // f32 — halocaster.py:795
	OffObjUnkDamage1     uint32 = 0x68  // s16 — halocaster.py:797
	OffObjTimeExisting   uint32 = 0x6C  // s16 — halocaster.py:796
	OffObjOwnerUnitRef   uint32 = 0x70  // u32 — halocaster.py:798
	OffObjOwnerObjectRef uint32 = 0x74  // u32 — halocaster.py:799
	OffObjUltimateParent uint32 = 0x1E4 // u32 — halocaster.py:801
)

// ============================================================================
// Projectile sub-struct (at object_address + RefAddrItemDatumSize) — HC:813-832
// ============================================================================
//
// NOTE: HC reads offset +0x1C twice: as `target_object_index` (s32, HC:818) and
// as `arming_time` (f32, HC:821). One must be wrong. Documented as-is here; M7
// runtime verification should resolve which is the real field.
const (
	OffProjFlags             uint32 = 0x00 // u32 — halocaster.py:813
	OffProjAction            uint32 = 0x04 // s16 — halocaster.py:815
	OffProjHitMaterialType   uint32 = 0x06 // s16 — halocaster.py:816
	OffProjIgnoreObjectIndex uint32 = 0x08 // s32 — halocaster.py:817
	OffProjDetonationTimer   uint32 = 0x14 // f32 — halocaster.py:819
	OffProjDetonationTimerDelta uint32 = 0x18 // f32 — halocaster.py:820
	// 0x1C: HaloCaster bug — same offset read as both target_object_index s32 and arming_time f32
	OffProjTargetObjectIndex      uint32 = 0x1C // s32 (HC:818) OR arming_time f32 (HC:821); resolve at M7
	OffProjArmingTimeDelta        uint32 = 0x20 // f32 — halocaster.py:822
	OffProjDistanceTraveled       uint32 = 0x24 // f32 — halocaster.py:823
	OffProjDecelerationTimer      uint32 = 0x28 // f32 — halocaster.py:824
	OffProjDecelerationTimerDelta uint32 = 0x2C // f32 — halocaster.py:825
	OffProjDeceleration           uint32 = 0x30 // f32 — halocaster.py:826
	OffProjMaximumDamageDistance  uint32 = 0x34 // f32 — halocaster.py:827
	OffProjRotationAxisX          uint32 = 0x3C // f32 — halocaster.py:828
	OffProjRotationAxisY          uint32 = 0x40 // f32 — halocaster.py:829
	OffProjRotationAxisZ          uint32 = 0x44 // f32 — halocaster.py:830
	OffProjRotationSine           uint32 = 0x48 // f32 — halocaster.py:831
	OffProjRotationCosine         uint32 = 0x4C // f32 — halocaster.py:832
)

// ============================================================================
// Object-type-definition entry (at *RefAddrObjectTypeDefArray + 4*type)
// HC:732-746
// ============================================================================
const (
	OffObjTypeDefStringPtr uint32 = 0x00 // u32 → null-terminated type name — halocaster.py:736
	OffObjTypeDefDatumSize uint32 = 0x08 // u16 — halocaster.py:744
)

// ============================================================================
// Scenario player spawns (separate from item spawns; at *AddrGlobalScenarioPtr)
// HC:584-608
// ============================================================================
const (
	OffScenarioPlayerSpawnCount uint32 = 852 // s32 — halocaster.py:584,336
	OffScenarioPlayerSpawnFirst uint32 = 856 // u32 — halocaster.py:585,334
	ScenarioPlayerSpawnStride          = 52  // bytes per entry — halocaster.py:611,337

	OffPlayerSpawnX         uint32 = 0x00 // f32 — halocaster.py:596
	OffPlayerSpawnY         uint32 = 0x04 // f32 — halocaster.py:597
	OffPlayerSpawnZ         uint32 = 0x08 // f32 — halocaster.py:598
	OffPlayerSpawnFacing    uint32 = 0x0C // f32 — halocaster.py:599
	OffPlayerSpawnTeamIndex uint32 = 0x10 // u8 — halocaster.py:600
	OffPlayerSpawnBspIndex  uint32 = 0x11 // u8 — halocaster.py:601
	OffPlayerSpawnUnk0      uint32 = 0x12 // u16 — halocaster.py:602
	OffPlayerSpawnGametype0 uint32 = 0x14 // u8 — halocaster.py:603
	OffPlayerSpawnGametype1 uint32 = 0x15 // u8 — halocaster.py:604
	OffPlayerSpawnGametype2 uint32 = 0x16 // u8 — halocaster.py:605
	OffPlayerSpawnGametype3 uint32 = 0x17 // u8 — halocaster.py:606
)

// ============================================================================
// CTF flag base position (3 floats at *RefAddrCTFFlag0Ptr / *RefAddrCTFFlag1Ptr)
// HC:849-857
// ============================================================================
const (
	OffCTFFlagX uint32 = 0x00 // f32 — halocaster.py:849
	OffCTFFlagY uint32 = 0x04 // f32 — halocaster.py:850
	OffCTFFlagZ uint32 = 0x08 // f32 — halocaster.py:851
)

// ============================================================================
// Fog parameters (at RefAddrFogParams) — HC:864-873
// ============================================================================
const (
	OffFogColorR      uint32 = 0x04 // f32 — halocaster.py:868
	OffFogColorG      uint32 = 0x08 // f32 — halocaster.py:869
	OffFogColorB      uint32 = 0x0C // f32 — halocaster.py:870
	OffFogMaxDensity  uint32 = 0x10 // f32 — halocaster.py:871
	OffFogAtmoMinDist uint32 = 0x14 // f32 (defaults to 1024?) — halocaster.py:872
	OffFogAtmoMaxDist uint32 = 0x18 // f32 (defaults to 2048?) — halocaster.py:873
)

// ============================================================================
// Per-local-player UI globals (at RefAddrPerLocalUIGlobals + local*56) — HC:922-935
// Stride 56 bytes per local player (0..3).
// ============================================================================
const (
	PerLocalUIStride              uint32 = 56
	OffUIProfileName              uint32 = 0x00 // wchar (TODO in HC; not actually read) — halocaster.py:925
	OffUIColor                    uint32 = 24   // u8 — halocaster.py:926
	OffUIButtonConfig             uint32 = 40   // u8 — halocaster.py:927
	OffUIJoystickConfig           uint32 = 41   // u8 — halocaster.py:928
	OffUISensitivity              uint32 = 42   // u8 — halocaster.py:929
	OffUIJoystickInverted         uint32 = 43   // u8 — halocaster.py:930
	OffUIRumbleEnabled            uint32 = 44   // u8 — halocaster.py:931
	OffUIFlightInverted           uint32 = 45   // u8 — halocaster.py:932
	OffUIAutocenterEnabled        uint32 = 46   // u8 — halocaster.py:933
	OffUIActivePlayerProfileIndex uint32 = 48   // u32 (used for saving profile data) — halocaster.py:934
	OffUIJoinedMultiplayerGame    uint32 = 52   // u8 — halocaster.py:935
)

// ============================================================================
// Input abstraction state (at RefAddrInputAbstractInputState + local*0x1C)
// HC:962-980. Stride 0x1C per local player.
// ============================================================================
const (
	InputAbstractStateStride uint32 = 0x1C

	OffIASBtnA                 uint32 = 0x00 // u8 — halocaster.py:964
	OffIASBtnBlack             uint32 = 0x01 // u8 — halocaster.py:965
	OffIASBtnX                 uint32 = 0x02 // u8 — halocaster.py:966
	OffIASBtnY                 uint32 = 0x03 // u8 — halocaster.py:967
	OffIASBtnB                 uint32 = 0x04 // u8 — halocaster.py:968
	OffIASBtnWhite             uint32 = 0x05 // u8 — halocaster.py:969
	OffIASLeftTrigger          uint32 = 0x06 // u8 — halocaster.py:970
	OffIASRightTrigger         uint32 = 0x07 // u8 — halocaster.py:971
	OffIASBtnStart             uint32 = 0x08 // u8 — halocaster.py:972
	OffIASBtnBack              uint32 = 0x09 // u8 — halocaster.py:973
	OffIASLeftStickButton      uint32 = 0x0A // u8 — halocaster.py:974
	OffIASRightStickButton     uint32 = 0x0B // u8 — halocaster.py:975
	OffIASLeftStickVertical    uint32 = 0x0C // f32 — halocaster.py:976
	OffIASLeftStickHorizontal  uint32 = 0x10 // f32 — halocaster.py:977
	OffIASRightStickHorizontal uint32 = 0x14 // f32 — halocaster.py:978
	OffIASRightStickVertical   uint32 = 0x18 // f32 — halocaster.py:979
)

// ============================================================================
// Gamepad state (at RefAddrGamepadState + player*0x28) — HC:985-1010
// Stride 0x28 per player.
// ============================================================================
const (
	GamepadStateStride uint32 = 0x28

	OffGPBtnA                 uint32 = 0x00 // u8 — halocaster.py:985
	OffGPBtnB                 uint32 = 0x01 // u8 — halocaster.py:986
	OffGPBtnX                 uint32 = 0x02 // u8 — halocaster.py:987
	OffGPBtnY                 uint32 = 0x03 // u8 — halocaster.py:988
	OffGPBtnBlack             uint32 = 0x04 // u8 — halocaster.py:989
	OffGPBtnWhite             uint32 = 0x05 // u8 — halocaster.py:990
	OffGPLeftTrigger          uint32 = 0x06 // u8 — halocaster.py:991
	OffGPRightTrigger         uint32 = 0x07 // u8 — halocaster.py:992
	OffGPBtnADuration         uint32 = 0x10 // u8 — halocaster.py:993
	OffGPBtnBDuration         uint32 = 0x11 // u8 — halocaster.py:994
	OffGPBtnXDuration         uint32 = 0x12 // u8 — halocaster.py:995
	OffGPBtnYDuration         uint32 = 0x13 // u8 — halocaster.py:996
	OffGPBlackDuration        uint32 = 0x14 // u8 — halocaster.py:997
	OffGPWhiteDuration        uint32 = 0x15 // u8 — halocaster.py:998
	OffGPLTDuration           uint32 = 0x16 // u8 — halocaster.py:999
	OffGPRTDuration           uint32 = 0x17 // u8 — halocaster.py:1000
	OffGPDpadUpDuration       uint32 = 0x18 // u8 — halocaster.py:1001
	OffGPDpadDownDuration     uint32 = 0x19 // u8 — halocaster.py:1002
	OffGPDpadLeftDuration     uint32 = 0x1A // u8 — halocaster.py:1003
	OffGPDpadRightDuration    uint32 = 0x1B // u8 — halocaster.py:1004
	OffGPLeftStickDuration    uint32 = 0x1E // u8 — halocaster.py:1005
	OffGPRightStickDuration   uint32 = 0x1F // u8 — halocaster.py:1006
	OffGPLeftStickHorizontal  uint32 = 0x20 // s16 — halocaster.py:1007
	OffGPLeftStickVertical    uint32 = 0x22 // s16 — halocaster.py:1008
	OffGPRightStickHorizontal uint32 = 0x24 // s16 — halocaster.py:1009
	OffGPRightStickVertical   uint32 = 0x26 // s16 — halocaster.py:1010
)

// ============================================================================
// Player control struct (at *RefAddrPlayerControlPtr + (local << 6))
// HC:954-959
// ============================================================================
const (
	OffPCDesiredYaw      uint32 = 0x1C // f32 — halocaster.py:954
	OffPCDesiredPitch    uint32 = 0x20 // f32 — halocaster.py:955
	OffPCZoomLevel       uint32 = 0x34 // s16 (= 0x10 + 0x24) — halocaster.py:956
	OffPCAimAssistTarget uint32 = 0x38 // u32 (= 0x10 + 0x28) — halocaster.py:957
	OffPCAimAssistNear   uint32 = 0x3C // f32 (= 0x10 + 0x2C) — halocaster.py:958
	OffPCAimAssistFar    uint32 = 0x40 // f32 (= 0x10 + 0x30) — halocaster.py:959
)

// ============================================================================
// Update-queue per-player record (within update_client_player + player*0x28)
// HC:1014-1033
// ============================================================================
const (
	UpdateQueuePlayerStride uint32 = 0x28

	OffUQUnitRef          uint32 = 0x00 // u16 — halocaster.py:1015
	OffUQButtonField      uint32 = 0x04 // u8 (bitfield) — halocaster.py:949,1016
	OffUQActionField      uint32 = 0x05 // u8 (bitfield) — halocaster.py:950,1023
	OffUQDesiredYaw       uint32 = 0x0C // f32 — halocaster.py:1026
	OffUQDesiredPitch     uint32 = 0x10 // f32 — halocaster.py:1027
	OffUQForward          uint32 = 0x14 // f32 — halocaster.py:1028
	OffUQLeft             uint32 = 0x18 // f32 — halocaster.py:1029
	OffUQRightTriggerHeld uint32 = 0x1C // f32 — halocaster.py:1030
	OffUQDesiredWeapon    uint32 = 0x20 // u16 — halocaster.py:1031
	OffUQDesiredGrenades  uint32 = 0x22 // u16 — halocaster.py:1032
	OffUQZoomLevel        uint32 = 0x24 // s16 — halocaster.py:1033
)

// Update-queue button-field bitmasks (HC:1017-1022).
const (
	UQBtnCrouch     uint8 = 0x01
	UQBtnJump       uint8 = 0x02
	UQBtnFire       uint8 = 0x08
	UQBtnFlashlight uint8 = 0x10
	UQBtnReload     uint8 = 0x40
	UQBtnMelee      uint8 = 0x80
)

// Update-queue action-field bitmasks (HC:1024-1025).
const (
	UQActThrowGrenade uint8 = 0x30
	UQActUseAction    uint8 = 0x40
)

// ============================================================================
// Update-queue header (at *RefAddrUpdateClientPlayerPtr) — HC:1287-1330
// ============================================================================
const (
	OffUQHdrFirstElement uint32 = 0x34 // u32 (read_u32 → first element addr) — halocaster.py:1288
	OffUQHdrBlindFirst   uint32 = 0x38 // u32 — halocaster.py:1289
	OffUQHdrUnk1         uint32 = 0x20 // s16 (max element count?) — halocaster.py:1323
	OffUQHdrUnk2         uint32 = 0x22 // s16 (element length?) — halocaster.py:1324
	OffUQHdrUnk3         uint32 = 0x24 // s16 — halocaster.py:1325
	OffUQHdrUnk4         uint32 = 0x2E // s16 (element count?) — halocaster.py:1326
	OffUQHdrUnk5         uint32 = 0x30 // s16 — halocaster.py:1327
	OffUQHdrUnk6         uint32 = 0x32 // u16 — halocaster.py:1328
)

// Data-queue header (at RefAddrUpdateQueueCounterLo dereferenced) — HC:1311-1316.
const (
	OffDataQueueTick         uint32 = 0x00 // s32 — halocaster.py:1312
	OffDataQueueGlobalRandom uint32 = 0x04 // u32 — halocaster.py:1313
	OffDataQueueTick2        uint32 = 0x08 // s32 — halocaster.py:1314
	OffDataQueueUnk1         uint32 = 0x0C // u16 (player index?) — halocaster.py:1315
	OffDataQueuePlayerCount  uint32 = 0x0E // s16 — halocaster.py:1316
)

// ============================================================================
// First-person weapon (at *RefAddrFPWeaponPtr + 7840*local) — HC:1071-1078
// ============================================================================
const (
	FPWeaponStride uint32 = 7840

	OffFPWWeaponRendered         uint32 = 0x00 // u32 — halocaster.py:1071
	OffFPWPlayerObject           uint32 = 0x04 // u32 — halocaster.py:1072
	OffFPWWeaponObject           uint32 = 0x08 // u32 — halocaster.py:1073
	OffFPWState                  uint32 = 0x0C // s16 — halocaster.py:1074 (0=idle,5=idle anim,6=fire,10=melee,14=reload,19=switch,20=grenade)
	OffFPWIdleAnimationThreshold uint32 = 0x0E // s16 — halocaster.py:1075
	OffFPWIdleAnimationCounter   uint32 = 0x10 // s16 — halocaster.py:1076
	OffFPWAnimationID            uint32 = 0x16 // s16 — halocaster.py:1077
	OffFPWAnimationTick          uint32 = 0x18 // s16 — halocaster.py:1078
)

// ============================================================================
// Observer camera (at RefAddrObserverCameraBase + 668*local) — HC:1091-1100
// HC stride: 167 floats × 4 bytes = 668 per local player
// ============================================================================
const (
	ObserverCameraStride uint32 = 668

	OffObsCamX    uint32 = 0x00 // f32 — halocaster.py:1091
	OffObsCamY    uint32 = 0x04 // f32 — halocaster.py:1092
	OffObsCamZ    uint32 = 0x08 // f32 — halocaster.py:1093
	OffObsCamVelX uint32 = 0x14 // f32 (≈ player_vel*pi) — halocaster.py:1094
	OffObsCamVelY uint32 = 0x18 // f32 — halocaster.py:1095
	OffObsCamVelZ uint32 = 0x1C // f32 — halocaster.py:1096
	OffObsCamAimX uint32 = 0x20 // f32 — halocaster.py:1097
	OffObsCamAimY uint32 = 0x24 // f32 — halocaster.py:1098
	OffObsCamAimZ uint32 = 0x28 // f32 — halocaster.py:1099
	OffObsCamFOV  uint32 = 0x38 // f32 vertical FOV in radians — halocaster.py:1100
)

// ============================================================================
// Model-node bone offsets within dynamic player — HC:1108-1126
// 19 hard-coded bones; HC:1107 also lists 0x438 ("player location") commented out.
// Each entry is xyz triple of f32 starting at the listed offset.
// ============================================================================
var ModelNodeBoneOffsets = []uint32{
	0x4A8, // halocaster.py:1108
	0x4DC, // halocaster.py:1109
	0x510, // halocaster.py:1110
	0x544, // halocaster.py:1111
	0x578, // halocaster.py:1112
	0x5AC, // halocaster.py:1113
	0x5E0, // halocaster.py:1114
	0x614, // halocaster.py:1115
	0x648, // halocaster.py:1116
	0x67C, // halocaster.py:1117
	0x6B0, // halocaster.py:1118
	0x6E4, // halocaster.py:1119
	0x718, // halocaster.py:1120
	0x74C, // halocaster.py:1121
	0x780, // halocaster.py:1122
	0x7B4, // halocaster.py:1123
	0x7E8, // halocaster.py:1124
	0x81C, // halocaster.py:1125
	0x850, // halocaster.py:1126
}

// OffDynPlayerLocationCommented is HC:1107's commented-out bone offset, kept here
// for completeness. Unclear if it's a real bone or HC's editorialized note.
const OffDynPlayerLocationCommented uint32 = 0x438

// ============================================================================
// Network game data sub-struct — HC:1213-1238
// (Located at network_game_client + 2140 for client, similar offset for server.)
// ============================================================================
const (
	OffNGDMaximumPlayerCount uint32 = 270 // u8 — halocaster.py:1222
	OffNGDMachineCount       uint32 = 274 // s16 — halocaster.py:1215
	OffNGDNetworkMachines    uint32 = 276 // base; stride 68 per machine — halocaster.py:1216
	OffNGDPlayerCount        uint32 = 548 // s16 — halocaster.py:1217
	OffNGDNetworkPlayers     uint32 = 550 // base; stride 32 per player — halocaster.py:1218
)

// Per-machine record (HC:1224-1226). Stride 68.
const (
	NetworkMachineStride      uint32 = 68
	OffNetMachineName         uint32 = 0x00 // wchar (64 bytes) — halocaster.py:1225
	OffNetMachineMachineIndex uint32 = 64   // u8 — halocaster.py:1226
)

// Per-network-player record (HC:1228-1235). Stride 32.
const (
	NetworkPlayerStride         uint32 = 32
	OffNetPlayerName            uint32 = 0x00 // wchar (24 bytes / 12 chars) — halocaster.py:1229
	OffNetPlayerColor           uint32 = 24   // s16 — halocaster.py:1230
	OffNetPlayerUnused          uint32 = 26   // s16 — halocaster.py:1231
	OffNetPlayerMachineIndex    uint32 = 28   // u8 — halocaster.py:1232
	OffNetPlayerControllerIndex uint32 = 29   // u8 — halocaster.py:1233
	OffNetPlayerTeam            uint32 = 30   // u8 — halocaster.py:1234
	OffNetPlayerListIndex       uint32 = 31   // u8 — halocaster.py:1235
)

// ============================================================================
// Network game client struct (at RefAddrNetworkGameClient) — HC:1244-1255
// ============================================================================
const (
	OffNGCMachineIndex       uint32 = 0    // u16 — halocaster.py:1245
	OffNGCPingTargetIP       uint32 = 2056 // s32 — halocaster.py:1247
	OffNGCPacketsSent        uint32 = 2084 // s16 — halocaster.py:1248
	OffNGCPacketsReceived    uint32 = 2086 // s16 — halocaster.py:1249
	OffNGCAveragePing        uint32 = 2088 // s16 — halocaster.py:1250
	OffNGCPingActive         uint32 = 2090 // u8 — halocaster.py:1251
	OffNGCNetworkGameData    uint32 = 2140 // sub-struct (see OffNGD* above) — halocaster.py:1255
	OffNGCSecondsToGameStart uint32 = 3236 // s16 — halocaster.py:1252
)

// ============================================================================
// Network game server struct (at RefAddrNetworkGameServer) — HC:1267-1269
// ============================================================================
const (
	OffNGSCountdownActive       uint32 = 1172 // u8 — halocaster.py:1267
	OffNGSCountdownPaused       uint32 = 1173 // u8 — halocaster.py:1268
	OffNGSCountdownAdjustedTime uint32 = 1174 // u8 — halocaster.py:1269
)

// ============================================================================
// Animation tag data — HC:1345-1351
// (Accessed via tag_address+120 for the animation array; per-anim stride 180.)
// ============================================================================
const (
	OffAnimTagArrayPtr uint32 = 120 // u32 → animation array — halocaster.py:1346
	AnimEntryStride    uint32 = 180 // bytes per animation entry

	OffAnimLength uint32 = 34 // s16 — halocaster.py:1348
	OffAnimUnk46  uint32 = 46 // s16 — halocaster.py:1349
	OffAnimUnk52  uint32 = 52 // s16 — halocaster.py:1350
	OffAnimUnk54  uint32 = 54 // s16 — halocaster.py:1351
)

// ============================================================================
// Memory cache pointers (DIAG; not actively read by Go reader)
// HC:330, 904-911. Sizes are at separate addresses (HC:908-911).
// ============================================================================
const (
	RefAddrGameStateBasePtr    uint32 = 0x2E2D14 // halocaster.py:904
	RefAddrTagCacheBasePtr     uint32 = 0x2E2D18 // halocaster.py:905
	RefAddrTextureCacheBasePtr uint32 = 0x2E2D1C // halocaster.py:906
	RefAddrSoundCacheBasePtr   uint32 = 0x2E2D20 // halocaster.py:907

	RefAddrGameStateSize    uint32 = 0x32E4A // halocaster.py:330,908
	RefAddrTagCacheSize     uint32 = 0x32E5D // halocaster.py:909
	RefAddrTextureCacheSize uint32 = 0x32E75 // halocaster.py:910
	RefAddrSoundCacheSize   uint32 = 0x32E8A // halocaster.py:911
)
