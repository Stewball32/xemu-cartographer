// This file captures every memory offset HaloCaster's halocaster.py reads that
// is NOT currently consumed by the M2 reader (offsets.go is the active set).
//
// Purpose: complete the M2a audit. Every constant has a halocaster.py:NNN tag.
// All are status `unverified` until first runtime use.
//
// These constants compile but are unused by the active read path. Future overlay
// or persistence work can reference them by name without re-deriving from the
// HaloCaster Python.
//
// Organization mirrors offsets.go (per-struct grouping). Within each group the
// `_ = ConstName` discard at the bottom suppresses Go's "unused" lint so the
// package builds cleanly until consumers wire them up.
//
// To "promote" any of these into the active path, move the constant into
// offsets.go, drop it from the _ = block here, and (for low GVAs) append to
// AllLowGVAs in offsets.go.

package haloce

// ============================================================================
// Reference low-GVA pointer globals (not in AllLowGVAs)
// ============================================================================
const (
	RefAddrHudMessagesPtr     uint32 = 0x276B40 // halocaster.py:572 — read_u32 → HUD message table base, stride 0x460
	RefAddrPlayerControlPtr   uint32 = 0x276794 // halocaster.py:944 — read_u32 → per-local control struct base
	RefAddrUpdateClientPlayerPtr uint32 = 0x2E8870 // halocaster.py:945,1287 — read_u32 → update queue (input replication)
	RefAddrFPWeaponPtr        uint32 = 0x276B48 // halocaster.py:1067 — read_u32 → first-person weapon array, stride 7840 per local
	RefAddrInputAbstractGlbls uint32 = 0x2E45A0 // halocaster.py:1041 — read_u32 → input_abstraction_globals
	RefAddrCTFFlag0Ptr        uint32 = 0x2762A4 // halocaster.py:845 — read_u32 → flag-base position float triple
	RefAddrCTFFlag1Ptr        uint32 = 0x2762A8 // halocaster.py:846 (= 0x2762A4+4) — read_u32 → flag-base position float triple
	RefAddrUpdateQueueCounterLo uint32 = 0x2E87E4 // halocaster.py:711,1277 — paired counter
	RefAddrUpdateQueueCounterHi uint32 = 0x2E87E8 // halocaster.py:711,1299 — diff = max actions allowed this tick
	RefAddrUpdateQueueAdjacent  uint32 = 0x2E8874 // halocaster.py:1308 — adjacent counter (debug only)
)

// ============================================================================
// Reference low-GVA direct-value globals (not in AllLowGVAs)
// ============================================================================
const (
	RefAddrPerLocalUIGlobals     uint32 = 0x2E40D0 // halocaster.py:922 — base; stride 56 per local player
	RefAddrInputAbstractInputState uint32 = 0x2E4600 // halocaster.py:963 — base; stride 0x1C per local player
	RefAddrGamepadStateAlt       uint32 = 0x276AFC // halocaster.py:983 — alt gamepad ptr
	RefAddrGamepadState          uint32 = 0x276A5C // halocaster.py:984 — gamepad state base; stride 0x28 per player
	RefAddrLookYawRate           uint32 = 0x2E4684 // halocaster.py:1039 — f32; stride 4 per local
	RefAddrLookPitchRate         uint32 = 0x2E4694 // halocaster.py:1040 — f32; stride 4 per local
	RefAddrObserverCameraBase    uint32 = 0x271550 // halocaster.py:340,1087,1915 — base; stride 668 (167*4) per local
	RefAddrNetworkGameClient     uint32 = 0x2FB180 // halocaster.py:1242,1276 — network_game_client struct
	RefAddrNetworkGameServer     uint32 = 0x2FBE40 // halocaster.py:1262 — network_game_server struct
	RefAddrFogParams             uint32 = 0x2FC8A8 // halocaster.py:864,867 — fog_params base
	RefAddrGlobalVariant         uint32 = 0x2F90A8 // halocaster.py:1187 — global_variant container
	RefAddrGlobalRandomSeed      uint32 = 0x2E3648 // halocaster.py:1932 — u32 RNG seed
	RefAddrObjectTypeDefArray    uint32 = 0x1FCB78 // halocaster.py:734,742 — u32[] table of object-type def pointers
	RefAddrObjectDatumSize       uint32 = 0x1FC0E0 // halocaster.py:765 — u16 base object struct size
	RefAddrUnitDatumSize         uint32 = 0x1FC188 // halocaster.py:766 — u16 unit subclass size
	RefAddrItemDatumSize         uint32 = 0x1FC380 // halocaster.py:767 — u16 item subclass size
	RefAddrObjectTypeDefRangeLo  uint32 = 0x1FC0D0 // halocaster.py:344 — object-type defs cache range start
	RefAddrObjectTypeDefRangeHi  uint32 = 0x1FCBA4 // halocaster.py:344 — object-type defs cache range end
	RefAddrDefaultFramerate      uint32 = 0xBB648  // halocaster.py:2139 — engine config (cosmetic)
	RefAddrRefreshRate           uint32 = 0x1F8C98 // halocaster.py:2140 — engine config (cosmetic)
)

// HudMessageStride is the per-message stride within the table at *RefAddrHudMessagesPtr.
// Origin: halocaster.py:729.
const HudMessageStride uint32 = 0x460

// ============================================================================
// Reference: extended GameTimeGlobals offsets are already in offsets.go
// (OffGTGMonitorState, OffGTGMonitorCounter, OffGTGMonitorLatency,
// OffGTGLeftoverDeltaTime were promoted into the active set — diagnostic but
// trivially cheap to read alongside the active fields).
// ============================================================================

// ============================================================================
// game_globals struct (at *AddrGameGlobalsPtr) — extended fields
// ============================================================================
const (
	OffGGMapLoaded               uint32 = 0x00 // u8 — halocaster.py:1425
	OffGGActive                  uint32 = 0x01 // u8 — halocaster.py:1426
	OffGGPlayersAreDoubleSpeed   uint32 = 0x02 // u8 — halocaster.py:1918
	OffGGGameLoadingInProgress   uint32 = 0x03 // u8 — halocaster.py:1919
	OffGGPrecacheMapStatus       uint32 = 0x04 // f32 — halocaster.py:1920
	OffGGGameDifficultyLevel     uint32 = 0x0E // u8 — halocaster.py:1921
	OffGGStoredGlobalRandom      uint32 = 0x10 // u32 (0xdeadbeef during pregame/mapselect) — halocaster.py:1933
)

// ============================================================================
// players_globals struct (at *AddrPlayersGlobalsPtr) — extended fields
// ============================================================================
const (
	OffPGLocalPlayerCount uint32 = 0x24 // u16 — halocaster.py:1906
)

// ============================================================================
// Static player struct — extended diagnostic fields (HC reads but unused here)
// ============================================================================
// Most useful static-player fields are already in offsets.go. None known beyond
// what's listed. This section is intentionally empty for symmetry.

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
	OffDynScale            uint32 = 0x60 // f32 (items only?) — halocaster.py:1653
	OffDynTypeU16          uint32 = 0x64 // u16 (read as u16; OffObjType reads as u8 at same addr) — halocaster.py:1654
	OffDynRenderFlags      uint32 = 0x66 // u16 — halocaster.py:1655
	OffDynWeaponOwnerTeam  uint32 = 0x68 // s16 (weapon-only context) — halocaster.py:1656
	OffDynPowerupUnk2      uint32 = 0x6A // s16 — halocaster.py:1657
	OffDynIdleTicks        uint32 = 0x6C // s16 (overlaps with OffPlrPlayerSpeed in different struct) — halocaster.py:1658

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

	// HaloCaster commented-out / known-broken fields, kept here for completeness.
	// HC marks each as exploratory; do not trust these offsets without re-verification.
	OffDynStunnedCandidateBroken             uint32 = 0x1CB // s32 — halocaster.py:1698 ("not actually stunned")
	OffDynMaybeDesiredFacingVectorXBroken    uint32 = 0x1C8 // f32 — halocaster.py:1700 (HC commented-out)
	OffDynMaybeDesiredFacingVectorYBroken    uint32 = 0x1CC // f32 — halocaster.py:1701 (HC FIXME "y is null")
	OffDynMaybeDesiredFacingVectorZBroken    uint32 = 0x1D0 // f32 — halocaster.py:1702 (HC commented-out)
	OffDynSelectedWeaponIndex2Broken         uint32 = 0x2A4 // s16 — halocaster.py:1736 (HC commented-out)
	OffDynCamoThing2Broken                   uint32 = 0x330 // f32 — halocaster.py:1756 (HC commented-out)

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
	OffDynAirborne                  uint32 = 0x424 // u8 (&1=airborne, &2=slipping) — halocaster.py:1776
	OffDynLandingStunCurrentDuration uint32 = 0x428 // u8 — halocaster.py:1777
	OffDynLandingStunTargetDuration  uint32 = 0x429 // u8 (typically 30 max) — halocaster.py:1778
	OffDynAirborneTicks             uint32 = 0x459 // u8 — halocaster.py:1779
	OffDynSlippingTicks             uint32 = 0x45A // u8 — halocaster.py:1782
	OffDynStopTicks                 uint32 = 0x45B // u8 — halocaster.py:1783
	OffDynJumpRecoveryTimer         uint32 = 0x45C // u8 — halocaster.py:1784
	OffDynLanding                   uint32 = 0x45F // u16 — halocaster.py:1788
	OffDynAirState460               uint32 = 0x460 // s16 (-1=walking, 0=landing, 1=fall damage) — halocaster.py:1790
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

	OffWepHeatMeter      uint32 = 0xD4  // f32 — halocaster.py:1586
	OffWepUsedEnergy     uint32 = 0xE0  // f32 (energy weapons only) — halocaster.py:1587
	OffWepOwnerHandle    uint32 = 0x1E0 // u32 (HC notes "not really owner; correlates to current action") — halocaster.py:1595
)

// ============================================================================
// Weapon tag data — zoom + auto-aim parameters (HC:1600-1607)
// ============================================================================
const (
	OffWepTagZoomLevels      uint32 = 986  // s16 — halocaster.py:1600
	OffWepTagZoomMin         uint32 = 988  // f32 — halocaster.py:1601
	OffWepTagZoomMax         uint32 = 992  // f32 — halocaster.py:1602
	OffWepTagAutoaimAngle    uint32 = 996  // f32 radians — halocaster.py:1603
	OffWepTagAutoaimRange    uint32 = 1000 // f32 — halocaster.py:1604
	OffWepTagMagnetismAngle  uint32 = 1004 // f32 — halocaster.py:1605
	OffWepTagMagnetismRange  uint32 = 1008 // f32 — halocaster.py:1606
	OffWepTagDeviationAngle  uint32 = 1012 // f32 — halocaster.py:1607
)

// ============================================================================
// Object header datum — common object offsets HC reads but reader.go doesn't yet
// (HC:790-803 from get_objects())
// ============================================================================
const (
	OffObjHeaderDataLen      uint32 = 12   // header_data byte length (entry stride) — halocaster.py:785
	OffObjAngVelX            uint32 = 0x3C // f32 — halocaster.py:793
	OffObjAngVelY            uint32 = 0x40 // f32 — halocaster.py:794
	OffObjAngVelZ            uint32 = 0x44 // f32 — halocaster.py:795
	OffObjUnkDamage1         uint32 = 0x68 // s16 — halocaster.py:797
	OffObjTimeExisting       uint32 = 0x6C // s16 — halocaster.py:796
	OffObjOwnerUnitRef       uint32 = 0x70 // u32 — halocaster.py:798
	OffObjOwnerObjectRef     uint32 = 0x74 // u32 — halocaster.py:799
	OffObjUltimateParent     uint32 = 0x1E4 // u32 — halocaster.py:801
)

// ============================================================================
// Projectile sub-struct (at object_address + RefAddrItemDatumSize) — HC:813-832
// ============================================================================
//
// NOTE: HC reads offset +0x1C twice: as `target_object_index` (s32, HC:818) and
// as `arming_time` (f32, HC:821). One must be wrong. Documented as-is here; M7
// runtime verification should resolve which is the real field.
const (
	OffProjFlags                  uint32 = 0x00 // u32 — halocaster.py:813
	OffProjAction                 uint32 = 0x04 // s16 — halocaster.py:815
	OffProjHitMaterialType        uint32 = 0x06 // s16 — halocaster.py:816
	OffProjIgnoreObjectIndex      uint32 = 0x08 // s32 — halocaster.py:817
	OffProjDetonationTimer        uint32 = 0x14 // f32 — halocaster.py:819
	OffProjDetonationTimerDelta   uint32 = 0x18 // f32 — halocaster.py:820
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

	OffPlayerSpawnX            uint32 = 0x00 // f32 — halocaster.py:596
	OffPlayerSpawnY            uint32 = 0x04 // f32 — halocaster.py:597
	OffPlayerSpawnZ            uint32 = 0x08 // f32 — halocaster.py:598
	OffPlayerSpawnFacing       uint32 = 0x0C // f32 — halocaster.py:599
	OffPlayerSpawnTeamIndex    uint32 = 0x10 // u8 — halocaster.py:600
	OffPlayerSpawnBspIndex     uint32 = 0x11 // u8 — halocaster.py:601
	OffPlayerSpawnUnk0         uint32 = 0x12 // u16 — halocaster.py:602
	OffPlayerSpawnGametype0    uint32 = 0x14 // u8 — halocaster.py:603
	OffPlayerSpawnGametype1    uint32 = 0x15 // u8 — halocaster.py:604
	OffPlayerSpawnGametype2    uint32 = 0x16 // u8 — halocaster.py:605
	OffPlayerSpawnGametype3    uint32 = 0x17 // u8 — halocaster.py:606
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
	OffFogColorR        uint32 = 0x04 // f32 — halocaster.py:868
	OffFogColorG        uint32 = 0x08 // f32 — halocaster.py:869
	OffFogColorB        uint32 = 0x0C // f32 — halocaster.py:870
	OffFogMaxDensity    uint32 = 0x10 // f32 — halocaster.py:871
	OffFogAtmoMinDist   uint32 = 0x14 // f32 (defaults to 1024?) — halocaster.py:872
	OffFogAtmoMaxDist   uint32 = 0x18 // f32 (defaults to 2048?) — halocaster.py:873
)

// ============================================================================
// Per-local-player UI globals (at RefAddrPerLocalUIGlobals + local*56) — HC:922-935
// Stride 56 bytes per local player (0..3).
// ============================================================================
const (
	PerLocalUIStride                uint32 = 56
	OffUIProfileName                uint32 = 0x00 // wchar (TODO in HC; not actually read) — halocaster.py:925
	OffUIColor                      uint32 = 24   // u8 — halocaster.py:926
	OffUIButtonConfig               uint32 = 40   // u8 — halocaster.py:927
	OffUIJoystickConfig             uint32 = 41   // u8 — halocaster.py:928
	OffUISensitivity                uint32 = 42   // u8 — halocaster.py:929
	OffUIJoystickInverted           uint32 = 43   // u8 — halocaster.py:930
	OffUIRumbleEnabled              uint32 = 44   // u8 — halocaster.py:931
	OffUIFlightInverted             uint32 = 45   // u8 — halocaster.py:932
	OffUIAutocenterEnabled          uint32 = 46   // u8 — halocaster.py:933
	OffUIActivePlayerProfileIndex   uint32 = 48   // u32 (used for saving profile data) — halocaster.py:934
	OffUIJoinedMultiplayerGame      uint32 = 52   // u8 — halocaster.py:935
)

// ============================================================================
// Input abstraction state (at RefAddrInputAbstractInputState + local*0x1C)
// HC:962-980. Stride 0x1C per local player.
// ============================================================================
const (
	InputAbstractStateStride uint32 = 0x1C

	OffIASBtnA               uint32 = 0x00 // u8 — halocaster.py:964
	OffIASBtnBlack           uint32 = 0x01 // u8 — halocaster.py:965
	OffIASBtnX               uint32 = 0x02 // u8 — halocaster.py:966
	OffIASBtnY               uint32 = 0x03 // u8 — halocaster.py:967
	OffIASBtnB               uint32 = 0x04 // u8 — halocaster.py:968
	OffIASBtnWhite           uint32 = 0x05 // u8 — halocaster.py:969
	OffIASLeftTrigger        uint32 = 0x06 // u8 — halocaster.py:970
	OffIASRightTrigger       uint32 = 0x07 // u8 — halocaster.py:971
	OffIASBtnStart           uint32 = 0x08 // u8 — halocaster.py:972
	OffIASBtnBack            uint32 = 0x09 // u8 — halocaster.py:973
	OffIASLeftStickButton    uint32 = 0x0A // u8 — halocaster.py:974
	OffIASRightStickButton   uint32 = 0x0B // u8 — halocaster.py:975
	OffIASLeftStickVertical  uint32 = 0x0C // f32 — halocaster.py:976
	OffIASLeftStickHorizontal uint32 = 0x10 // f32 — halocaster.py:977
	OffIASRightStickHorizontal uint32 = 0x14 // f32 — halocaster.py:978
	OffIASRightStickVertical uint32 = 0x18 // f32 — halocaster.py:979
)

// ============================================================================
// Gamepad state (at RefAddrGamepadState + player*0x28) — HC:985-1010
// Stride 0x28 per player.
// ============================================================================
const (
	GamepadStateStride uint32 = 0x28

	OffGPBtnA          uint32 = 0x00 // u8 — halocaster.py:985
	OffGPBtnB          uint32 = 0x01 // u8 — halocaster.py:986
	OffGPBtnX          uint32 = 0x02 // u8 — halocaster.py:987
	OffGPBtnY          uint32 = 0x03 // u8 — halocaster.py:988
	OffGPBtnBlack      uint32 = 0x04 // u8 — halocaster.py:989
	OffGPBtnWhite      uint32 = 0x05 // u8 — halocaster.py:990
	OffGPLeftTrigger   uint32 = 0x06 // u8 — halocaster.py:991
	OffGPRightTrigger  uint32 = 0x07 // u8 — halocaster.py:992
	OffGPBtnADuration  uint32 = 0x10 // u8 — halocaster.py:993
	OffGPBtnBDuration  uint32 = 0x11 // u8 — halocaster.py:994
	OffGPBtnXDuration  uint32 = 0x12 // u8 — halocaster.py:995
	OffGPBtnYDuration  uint32 = 0x13 // u8 — halocaster.py:996
	OffGPBlackDuration uint32 = 0x14 // u8 — halocaster.py:997
	OffGPWhiteDuration uint32 = 0x15 // u8 — halocaster.py:998
	OffGPLTDuration    uint32 = 0x16 // u8 — halocaster.py:999
	OffGPRTDuration    uint32 = 0x17 // u8 — halocaster.py:1000
	OffGPDpadUpDuration uint32 = 0x18 // u8 — halocaster.py:1001
	OffGPDpadDownDuration uint32 = 0x19 // u8 — halocaster.py:1002
	OffGPDpadLeftDuration uint32 = 0x1A // u8 — halocaster.py:1003
	OffGPDpadRightDuration uint32 = 0x1B // u8 — halocaster.py:1004
	OffGPLeftStickDuration uint32 = 0x1E // u8 — halocaster.py:1005
	OffGPRightStickDuration uint32 = 0x1F // u8 — halocaster.py:1006
	OffGPLeftStickHorizontal uint32 = 0x20 // s16 — halocaster.py:1007
	OffGPLeftStickVertical   uint32 = 0x22 // s16 — halocaster.py:1008
	OffGPRightStickHorizontal uint32 = 0x24 // s16 — halocaster.py:1009
	OffGPRightStickVertical  uint32 = 0x26 // s16 — halocaster.py:1010
)

// ============================================================================
// Player control struct (at *RefAddrPlayerControlPtr + (local << 6))
// HC:954-959
// ============================================================================
const (
	OffPCDesiredYaw       uint32 = 0x1C // f32 — halocaster.py:954
	OffPCDesiredPitch     uint32 = 0x20 // f32 — halocaster.py:955
	OffPCZoomLevel        uint32 = 0x34 // s16 (= 0x10 + 0x24) — halocaster.py:956
	OffPCAimAssistTarget  uint32 = 0x38 // u32 (= 0x10 + 0x28) — halocaster.py:957
	OffPCAimAssistNear    uint32 = 0x3C // f32 (= 0x10 + 0x2C) — halocaster.py:958
	OffPCAimAssistFar     uint32 = 0x40 // f32 (= 0x10 + 0x30) — halocaster.py:959
)

// ============================================================================
// Update-queue per-player record (within update_client_player + player*0x28)
// HC:1014-1033
// ============================================================================
const (
	UpdateQueuePlayerStride uint32 = 0x28

	OffUQUnitRef         uint32 = 0x00 // u16 — halocaster.py:1015
	OffUQButtonField     uint32 = 0x04 // u8 (bitfield) — halocaster.py:949,1016
	OffUQActionField     uint32 = 0x05 // u8 (bitfield) — halocaster.py:950,1023
	OffUQDesiredYaw      uint32 = 0x0C // f32 — halocaster.py:1026
	OffUQDesiredPitch    uint32 = 0x10 // f32 — halocaster.py:1027
	OffUQForward         uint32 = 0x14 // f32 — halocaster.py:1028
	OffUQLeft            uint32 = 0x18 // f32 — halocaster.py:1029
	OffUQRightTriggerHeld uint32 = 0x1C // f32 — halocaster.py:1030
	OffUQDesiredWeapon   uint32 = 0x20 // u16 — halocaster.py:1031
	OffUQDesiredGrenades uint32 = 0x22 // u16 — halocaster.py:1032
	OffUQZoomLevel       uint32 = 0x24 // s16 — halocaster.py:1033
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
	OffDataQueueTick        uint32 = 0x00 // s32 — halocaster.py:1312
	OffDataQueueGlobalRandom uint32 = 0x04 // u32 — halocaster.py:1313
	OffDataQueueTick2       uint32 = 0x08 // s32 — halocaster.py:1314
	OffDataQueueUnk1        uint32 = 0x0C // u16 (player index?) — halocaster.py:1315
	OffDataQueuePlayerCount uint32 = 0x0E // s16 — halocaster.py:1316
)

// ============================================================================
// First-person weapon (at *RefAddrFPWeaponPtr + 7840*local) — HC:1071-1078
// ============================================================================
const (
	FPWeaponStride uint32 = 7840

	OffFPWWeaponRendered          uint32 = 0x00 // u32 — halocaster.py:1071
	OffFPWPlayerObject            uint32 = 0x04 // u32 — halocaster.py:1072
	OffFPWWeaponObject            uint32 = 0x08 // u32 — halocaster.py:1073
	OffFPWState                   uint32 = 0x0C // s16 — halocaster.py:1074 (0=idle,5=idle anim,6=fire,10=melee,14=reload,19=switch,20=grenade)
	OffFPWIdleAnimationThreshold  uint32 = 0x0E // s16 — halocaster.py:1075
	OffFPWIdleAnimationCounter    uint32 = 0x10 // s16 — halocaster.py:1076
	OffFPWAnimationID             uint32 = 0x16 // s16 — halocaster.py:1077
	OffFPWAnimationTick           uint32 = 0x18 // s16 — halocaster.py:1078
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
	NetworkMachineStride       uint32 = 68
	OffNetMachineName          uint32 = 0x00 // wchar (64 bytes) — halocaster.py:1225
	OffNetMachineMachineIndex  uint32 = 64   // u8 — halocaster.py:1226
)

// Per-network-player record (HC:1228-1235). Stride 32.
const (
	NetworkPlayerStride        uint32 = 32
	OffNetPlayerName           uint32 = 0x00 // wchar (24 bytes / 12 chars) — halocaster.py:1229
	OffNetPlayerColor          uint32 = 24   // s16 — halocaster.py:1230
	OffNetPlayerUnused         uint32 = 26   // s16 — halocaster.py:1231
	OffNetPlayerMachineIndex   uint32 = 28   // u8 — halocaster.py:1232
	OffNetPlayerControllerIndex uint32 = 29   // u8 — halocaster.py:1233
	OffNetPlayerTeam           uint32 = 30   // u8 — halocaster.py:1234
	OffNetPlayerListIndex      uint32 = 31   // u8 — halocaster.py:1235
)

// ============================================================================
// Network game client struct (at RefAddrNetworkGameClient) — HC:1244-1255
// ============================================================================
const (
	OffNGCMachineIndex      uint32 = 0    // u16 — halocaster.py:1245
	OffNGCPingTargetIP      uint32 = 2056 // s32 — halocaster.py:1247
	OffNGCPacketsSent       uint32 = 2084 // s16 — halocaster.py:1248
	OffNGCPacketsReceived   uint32 = 2086 // s16 — halocaster.py:1249
	OffNGCAveragePing       uint32 = 2088 // s16 — halocaster.py:1250
	OffNGCPingActive        uint32 = 2090 // u8 — halocaster.py:1251
	OffNGCNetworkGameData   uint32 = 2140 // sub-struct (see OffNGD* above) — halocaster.py:1255
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
// Memory cache pointers (DIAG; not used by Go reader — included for completeness)
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

// ============================================================================
// Sentinel: ensure all reference constants compile cleanly even without consumers.
// Each `_ = X` discard tells the Go compiler "yes, this is intentionally unused
// for now." Remove the entry when the constant gets a real consumer.
// ============================================================================
var _ = []any{
	// Pointer / direct globals
	RefAddrHudMessagesPtr, RefAddrPlayerControlPtr, RefAddrUpdateClientPlayerPtr,
	RefAddrFPWeaponPtr, RefAddrInputAbstractGlbls, RefAddrCTFFlag0Ptr, RefAddrCTFFlag1Ptr,
	RefAddrUpdateQueueCounterLo, RefAddrUpdateQueueCounterHi, RefAddrUpdateQueueAdjacent,
	RefAddrPerLocalUIGlobals, RefAddrInputAbstractInputState, RefAddrGamepadStateAlt,
	RefAddrGamepadState, RefAddrLookYawRate, RefAddrLookPitchRate,
	RefAddrObserverCameraBase, RefAddrNetworkGameClient, RefAddrNetworkGameServer,
	RefAddrFogParams, RefAddrGlobalVariant, RefAddrGlobalRandomSeed,
	RefAddrObjectTypeDefArray, RefAddrObjectDatumSize, RefAddrUnitDatumSize,
	RefAddrItemDatumSize, RefAddrObjectTypeDefRangeLo, RefAddrObjectTypeDefRangeHi,
	RefAddrDefaultFramerate, RefAddrRefreshRate,

	// game_globals / players_globals extended
	OffGGMapLoaded, OffGGActive, OffGGPlayersAreDoubleSpeed, OffGGGameLoadingInProgress,
	OffGGPrecacheMapStatus, OffGGGameDifficultyLevel, OffGGStoredGlobalRandom,
	OffPGLocalPlayerCount,

	// Dynamic player extended (legs / aim assist / countdowns / aim vectors / etc.)
	OffDynLegsPitch, OffDynLegsYaw, OffDynLegsRoll, OffDynPitch1, OffDynYaw1, OffDynRoll1,
	OffDynAngVelX, OffDynAngVelY, OffDynAngVelZ,
	OffDynAimAssistSphereX, OffDynAimAssistSphereY, OffDynAimAssistSphereZ, OffDynAimAssistSphereRadius,
	OffDynScale, OffDynTypeU16, OffDynRenderFlags, OffDynWeaponOwnerTeam, OffDynPowerupUnk2,
	OffDynIdleTicks, OffDynAnimationUnk1, OffDynAnimationUnk2, OffDynAnimationUnk3,
	OffDynDmgCountdown_98, OffDynDmgCountdown_9C, OffDynDmgCountdown_A4, OffDynDmgCountdown_A8,
	OffDynDmgCounter_AC, OffDynDmgCounter_B0, OffDynShieldsStatus2, OffDynShieldsChargeDelay,
	OffDynNextObject, OffDynNextObject2, OffDynStateFlags, OffDynDropTime,
	OffDynFlashlight, OffDynStunned,
	OffDynStunnedCandidateBroken, OffDynMaybeDesiredFacingVectorXBroken,
	OffDynMaybeDesiredFacingVectorYBroken, OffDynMaybeDesiredFacingVectorZBroken,
	OffDynSelectedWeaponIndex2Broken, OffDynCamoThing2Broken,
	OffDynXunk0, OffDynYunk0, OffDynZunk0, OffDynXAimA, OffDynYAimA, OffDynZAimA,
	OffDynXAim0, OffDynYAim0, OffDynZAim0, OffDynXAim1, OffDynYAim1, OffDynZAim1,
	OffDynLookingVectorX, OffDynLookingVectorY, OffDynLookingVectorZ,
	OffDynMoveForward, OffDynMoveLeft, OffDynMoveUp,
	OffDynMeleeDamageType, OffDynAnimation1, OffDynAnimation2,
	OffDynCurrentEquipment, OffDynCamoSelfRevealed,
	OffDynFacing1, OffDynFacing2, OffDynFacing3,
	OffDynAirborne, OffDynLandingStunCurrentDuration, OffDynLandingStunTargetDuration,
	OffDynAirborneTicks, OffDynSlippingTicks, OffDynStopTicks, OffDynJumpRecoveryTimer,
	OffDynLanding, OffDynAirState460,

	// Biped tag extended
	OffBipedTagFlags, OffBipedTagAutoaimPillRadius,

	// Weapon obj / tag extended
	OffWepObjX, OffWepObjY, OffWepObjZ, OffWepHeatMeter, OffWepUsedEnergy, OffWepOwnerHandle,
	OffWepTagZoomLevels, OffWepTagZoomMin, OffWepTagZoomMax,
	OffWepTagAutoaimAngle, OffWepTagAutoaimRange,
	OffWepTagMagnetismAngle, OffWepTagMagnetismRange, OffWepTagDeviationAngle,

	// Generic object extended
	OffObjHeaderDataLen, OffObjAngVelX, OffObjAngVelY, OffObjAngVelZ,
	OffObjUnkDamage1, OffObjTimeExisting, OffObjOwnerUnitRef, OffObjOwnerObjectRef,
	OffObjUltimateParent,

	// Projectile sub-struct
	OffProjFlags, OffProjAction, OffProjHitMaterialType, OffProjIgnoreObjectIndex,
	OffProjDetonationTimer, OffProjDetonationTimerDelta, OffProjTargetObjectIndex,
	OffProjArmingTimeDelta, OffProjDistanceTraveled, OffProjDecelerationTimer,
	OffProjDecelerationTimerDelta, OffProjDeceleration, OffProjMaximumDamageDistance,
	OffProjRotationAxisX, OffProjRotationAxisY, OffProjRotationAxisZ,
	OffProjRotationSine, OffProjRotationCosine,

	// Object-type definitions
	OffObjTypeDefStringPtr, OffObjTypeDefDatumSize,

	// Scenario player spawns
	OffScenarioPlayerSpawnCount, OffScenarioPlayerSpawnFirst, ScenarioPlayerSpawnStride,
	OffPlayerSpawnX, OffPlayerSpawnY, OffPlayerSpawnZ, OffPlayerSpawnFacing,
	OffPlayerSpawnTeamIndex, OffPlayerSpawnBspIndex, OffPlayerSpawnUnk0,
	OffPlayerSpawnGametype0, OffPlayerSpawnGametype1, OffPlayerSpawnGametype2, OffPlayerSpawnGametype3,

	// CTF flag positions
	OffCTFFlagX, OffCTFFlagY, OffCTFFlagZ,

	// Fog
	OffFogColorR, OffFogColorG, OffFogColorB,
	OffFogMaxDensity, OffFogAtmoMinDist, OffFogAtmoMaxDist,

	// Per-local UI globals
	PerLocalUIStride, OffUIProfileName, OffUIColor, OffUIButtonConfig,
	OffUIJoystickConfig, OffUISensitivity, OffUIJoystickInverted, OffUIRumbleEnabled,
	OffUIFlightInverted, OffUIAutocenterEnabled, OffUIActivePlayerProfileIndex,
	OffUIJoinedMultiplayerGame,

	// Input abstraction
	InputAbstractStateStride,
	OffIASBtnA, OffIASBtnBlack, OffIASBtnX, OffIASBtnY, OffIASBtnB, OffIASBtnWhite,
	OffIASLeftTrigger, OffIASRightTrigger, OffIASBtnStart, OffIASBtnBack,
	OffIASLeftStickButton, OffIASRightStickButton,
	OffIASLeftStickVertical, OffIASLeftStickHorizontal,
	OffIASRightStickHorizontal, OffIASRightStickVertical,

	// Gamepad
	GamepadStateStride,
	OffGPBtnA, OffGPBtnB, OffGPBtnX, OffGPBtnY, OffGPBtnBlack, OffGPBtnWhite,
	OffGPLeftTrigger, OffGPRightTrigger,
	OffGPBtnADuration, OffGPBtnBDuration, OffGPBtnXDuration, OffGPBtnYDuration,
	OffGPBlackDuration, OffGPWhiteDuration, OffGPLTDuration, OffGPRTDuration,
	OffGPDpadUpDuration, OffGPDpadDownDuration, OffGPDpadLeftDuration, OffGPDpadRightDuration,
	OffGPLeftStickDuration, OffGPRightStickDuration,
	OffGPLeftStickHorizontal, OffGPLeftStickVertical,
	OffGPRightStickHorizontal, OffGPRightStickVertical,

	// Player control
	OffPCDesiredYaw, OffPCDesiredPitch, OffPCZoomLevel,
	OffPCAimAssistTarget, OffPCAimAssistNear, OffPCAimAssistFar,

	// Update queue per-player
	UpdateQueuePlayerStride,
	OffUQUnitRef, OffUQButtonField, OffUQActionField,
	OffUQDesiredYaw, OffUQDesiredPitch, OffUQForward, OffUQLeft,
	OffUQRightTriggerHeld, OffUQDesiredWeapon, OffUQDesiredGrenades, OffUQZoomLevel,

	// Update queue button/action bits
	UQBtnCrouch, UQBtnJump, UQBtnFire, UQBtnFlashlight, UQBtnReload, UQBtnMelee,
	UQActThrowGrenade, UQActUseAction,

	// Update queue header
	OffUQHdrFirstElement, OffUQHdrBlindFirst,
	OffUQHdrUnk1, OffUQHdrUnk2, OffUQHdrUnk3, OffUQHdrUnk4, OffUQHdrUnk5, OffUQHdrUnk6,
	OffDataQueueTick, OffDataQueueGlobalRandom, OffDataQueueTick2,
	OffDataQueueUnk1, OffDataQueuePlayerCount,

	// First-person weapon
	FPWeaponStride,
	OffFPWWeaponRendered, OffFPWPlayerObject, OffFPWWeaponObject, OffFPWState,
	OffFPWIdleAnimationThreshold, OffFPWIdleAnimationCounter,
	OffFPWAnimationID, OffFPWAnimationTick,

	// Observer camera
	ObserverCameraStride,
	OffObsCamX, OffObsCamY, OffObsCamZ,
	OffObsCamVelX, OffObsCamVelY, OffObsCamVelZ,
	OffObsCamAimX, OffObsCamAimY, OffObsCamAimZ, OffObsCamFOV,

	// Network game data
	OffNGDMaximumPlayerCount, OffNGDMachineCount, OffNGDNetworkMachines,
	OffNGDPlayerCount, OffNGDNetworkPlayers,
	NetworkMachineStride, OffNetMachineName, OffNetMachineMachineIndex,
	NetworkPlayerStride,
	OffNetPlayerName, OffNetPlayerColor, OffNetPlayerUnused,
	OffNetPlayerMachineIndex, OffNetPlayerControllerIndex, OffNetPlayerTeam, OffNetPlayerListIndex,

	// Network game client
	OffNGCMachineIndex, OffNGCPingTargetIP,
	OffNGCPacketsSent, OffNGCPacketsReceived, OffNGCAveragePing, OffNGCPingActive,
	OffNGCNetworkGameData, OffNGCSecondsToGameStart,

	// Network game server
	OffNGSCountdownActive, OffNGSCountdownPaused, OffNGSCountdownAdjustedTime,

	// Animation tag
	OffAnimTagArrayPtr, AnimEntryStride,
	OffAnimLength, OffAnimUnk46, OffAnimUnk52, OffAnimUnk54,

	// Memory cache (DIAG)
	RefAddrGameStateBasePtr, RefAddrTagCacheBasePtr, RefAddrTextureCacheBasePtr, RefAddrSoundCacheBasePtr,
	RefAddrGameStateSize, RefAddrTagCacheSize, RefAddrTextureCacheSize, RefAddrSoundCacheSize,

	// HUD message
	HudMessageStride,

	// Misc
	OffDynPlayerLocationCommented,
	ModelNodeBoneOffsets,
}
