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
// Reference low-GVA constants in offsets_reference.go (HUD messages, observer
// camera, network globals, etc.) are intentionally excluded — including them
// would force unnecessary QMP round-trips at startup. Future readers that need
// those should append to this slice.
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
