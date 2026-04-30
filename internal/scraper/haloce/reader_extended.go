package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readDynPlayerExtended reads the ~60 dynamic-biped diagnostic fields not
// covered by readDynPlayerFull. All offsets are within the same object_data
// block we already deref'd; this function only does memory reads, no pointer
// chasing. Source: dynamic biped extended OffDyn* constants in offsets.go.
func (r *Reader) readDynPlayerExtended(objDataAddr uint32) *scraper.TickPlayerExtended {
	mem := r.inst.Mem
	ext := &scraper.TickPlayerExtended{}

	// Leg / facing rotations
	ext.LegsPitch, _ = mem.ReadF32(objDataAddr + OffDynLegsPitch)
	ext.LegsYaw, _ = mem.ReadF32(objDataAddr + OffDynLegsYaw)
	ext.LegsRoll, _ = mem.ReadF32(objDataAddr + OffDynLegsRoll)
	ext.Pitch1, _ = mem.ReadF32(objDataAddr + OffDynPitch1)
	ext.Yaw1, _ = mem.ReadF32(objDataAddr + OffDynYaw1)
	ext.Roll1, _ = mem.ReadF32(objDataAddr + OffDynRoll1)

	// Angular velocity
	ext.AngVelX, _ = mem.ReadF32(objDataAddr + OffDynAngVelX)
	ext.AngVelY, _ = mem.ReadF32(objDataAddr + OffDynAngVelY)
	ext.AngVelZ, _ = mem.ReadF32(objDataAddr + OffDynAngVelZ)

	// Aim-assist sphere
	ext.AimAssistSphereX, _ = mem.ReadF32(objDataAddr + OffDynAimAssistSphereX)
	ext.AimAssistSphereY, _ = mem.ReadF32(objDataAddr + OffDynAimAssistSphereY)
	ext.AimAssistSphereZ, _ = mem.ReadF32(objDataAddr + OffDynAimAssistSphereZ)
	ext.AimAssistSphereRadius, _ = mem.ReadF32(objDataAddr + OffDynAimAssistSphereRadius)

	// Object scale + sub-type
	ext.Scale, _ = mem.ReadF32(objDataAddr + OffDynScale)
	ext.TypeU16, _ = mem.ReadU16(objDataAddr + OffDynTypeU16)
	ext.RenderFlags, _ = mem.ReadU16(objDataAddr + OffDynRenderFlags)
	ext.WeaponOwnerTeam, _ = mem.ReadS16(objDataAddr + OffDynWeaponOwnerTeam)
	ext.PowerupUnk2, _ = mem.ReadS16(objDataAddr + OffDynPowerupUnk2)
	ext.IdleTicks, _ = mem.ReadS16(objDataAddr + OffDynIdleTicks)

	// Animation handle / id / tick
	ext.AnimationUnk1, _ = mem.ReadU32(objDataAddr + OffDynAnimationUnk1)
	ext.AnimationUnk2, _ = mem.ReadS16(objDataAddr + OffDynAnimationUnk2)
	ext.AnimationUnk3, _ = mem.ReadS16(objDataAddr + OffDynAnimationUnk3)

	// Damage countdowns
	ext.DmgCountdown98, _ = mem.ReadF32(objDataAddr + OffDynDmgCountdown_98)
	ext.DmgCountdown9C, _ = mem.ReadF32(objDataAddr + OffDynDmgCountdown_9C)
	ext.DmgCountdownA4, _ = mem.ReadF32(objDataAddr + OffDynDmgCountdown_A4)
	ext.DmgCountdownA8, _ = mem.ReadF32(objDataAddr + OffDynDmgCountdown_A8)
	ext.DmgCounterAC, _ = mem.ReadS32(objDataAddr + OffDynDmgCounter_AC)
	ext.DmgCounterB0, _ = mem.ReadS32(objDataAddr + OffDynDmgCounter_B0)

	// Shields (extended)
	ext.ShieldsStatus2, _ = mem.ReadU16(objDataAddr + OffDynShieldsStatus2)
	ext.ShieldsChargeDelay, _ = mem.ReadU16(objDataAddr + OffDynShieldsChargeDelay)

	// Object-table linkage
	ext.NextObject, _ = mem.ReadS32(objDataAddr + OffDynNextObject)
	ext.NextObject2, _ = mem.ReadU32(objDataAddr + OffDynNextObject2)

	// State / flashlight / stunned
	ext.StateFlags, _ = mem.ReadU8(objDataAddr + OffDynStateFlags)
	ext.Flashlight, _ = mem.ReadU8(objDataAddr + OffDynFlashlight)
	ext.Stunned, _ = mem.ReadF32(objDataAddr + OffDynStunned)

	// Aim / look unit vectors
	ext.Xunk0, _ = mem.ReadF32(objDataAddr + OffDynXunk0)
	ext.Yunk0, _ = mem.ReadF32(objDataAddr + OffDynYunk0)
	ext.Zunk0, _ = mem.ReadF32(objDataAddr + OffDynZunk0)
	ext.XAimA, _ = mem.ReadF32(objDataAddr + OffDynXAimA)
	ext.YAimA, _ = mem.ReadF32(objDataAddr + OffDynYAimA)
	ext.ZAimA, _ = mem.ReadF32(objDataAddr + OffDynZAimA)
	ext.XAim0, _ = mem.ReadF32(objDataAddr + OffDynXAim0)
	ext.YAim0, _ = mem.ReadF32(objDataAddr + OffDynYAim0)
	ext.ZAim0, _ = mem.ReadF32(objDataAddr + OffDynZAim0)
	ext.XAim1, _ = mem.ReadF32(objDataAddr + OffDynXAim1)
	ext.YAim1, _ = mem.ReadF32(objDataAddr + OffDynYAim1)
	ext.ZAim1, _ = mem.ReadF32(objDataAddr + OffDynZAim1)
	ext.LookingVectorX, _ = mem.ReadF32(objDataAddr + OffDynLookingVectorX)
	ext.LookingVectorY, _ = mem.ReadF32(objDataAddr + OffDynLookingVectorY)
	ext.LookingVectorZ, _ = mem.ReadF32(objDataAddr + OffDynLookingVectorZ)

	// Movement throttles
	ext.MoveForward, _ = mem.ReadF32(objDataAddr + OffDynMoveForward)
	ext.MoveLeft, _ = mem.ReadF32(objDataAddr + OffDynMoveLeft)
	ext.MoveUp, _ = mem.ReadF32(objDataAddr + OffDynMoveUp)

	// Melee + animation tags
	ext.MeleeDamageType, _ = mem.ReadU8(objDataAddr + OffDynMeleeDamageType)
	ext.Animation1, _ = mem.ReadU8(objDataAddr + OffDynAnimation1)
	ext.Animation2, _ = mem.ReadU8(objDataAddr + OffDynAnimation2)

	// Equipment / camo extended
	ext.CurrentEquipment, _ = mem.ReadU32(objDataAddr + OffDynCurrentEquipment)
	ext.CamoSelfRevealed, _ = mem.ReadU16(objDataAddr + OffDynCamoSelfRevealed)

	// Facing vectors
	ext.Facing1, _ = mem.ReadF32(objDataAddr + OffDynFacing1)
	ext.Facing2, _ = mem.ReadF32(objDataAddr + OffDynFacing2)
	ext.Facing3, _ = mem.ReadF32(objDataAddr + OffDynFacing3)

	// Air / landing
	ext.Airborne, _ = mem.ReadU8(objDataAddr + OffDynAirborne)
	ext.LandingStunCurrentDuration, _ = mem.ReadU8(objDataAddr + OffDynLandingStunCurrentDuration)
	ext.LandingStunTargetDuration, _ = mem.ReadU8(objDataAddr + OffDynLandingStunTargetDuration)
	ext.AirborneTicks, _ = mem.ReadU8(objDataAddr + OffDynAirborneTicks)
	ext.SlippingTicks, _ = mem.ReadU8(objDataAddr + OffDynSlippingTicks)
	ext.StopTicks, _ = mem.ReadU8(objDataAddr + OffDynStopTicks)
	ext.JumpRecoveryTimer, _ = mem.ReadU8(objDataAddr + OffDynJumpRecoveryTimer)
	ext.Landing, _ = mem.ReadU16(objDataAddr + OffDynLanding)
	ext.AirState460, _ = mem.ReadS16(objDataAddr + OffDynAirState460)

	return ext
}
