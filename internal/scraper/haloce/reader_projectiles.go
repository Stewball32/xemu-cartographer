package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// ObjectTypeProjectile is the value at OffObjType for projectile objects.
// Standard Halo object-type enum: 0=biped, 1=vehicle, 2=weapon, 3=equipment,
// 4=garbage, 5=projectile, 6=scenery, ... — needs M7 verification on Xbox.
const ObjectTypeProjectile uint8 = 5

// readProjectiles walks the OHD object array and returns one TickProjectile
// per non-garbage projectile (filtered on OffObjType == ObjectTypeProjectile).
// Each projectile's per-tick state lives at objDataAddr + itemDatumSize, so
// this also reads RefAddrItemDatumSize once per call to determine the offset.
//
// Source: OffProj* + RefAddrItemDatumSize constants. M7 needs to verify both
// the type filter (5 vs other) and the +0x1C overlap (target_object_index
// vs arming_time — currently exposed as TargetObjectIndex int32).
func (r *Reader) readProjectiles() []scraper.TickProjectile {
	if r.ohdBase < HighGVAThreshold {
		return nil
	}
	inst := r.inst
	mem := inst.Mem

	// Read item-datum size (u16, low GVA) — gives the byte offset within
	// each object data block where projectile-specific fields start.
	itemDatumHVA, err := inst.LowHVA(RefAddrItemDatumSize)
	if err != nil {
		return nil
	}
	itemDatumSize, _ := mem.ReadU16At(itemDatumHVA)
	if itemDatumSize == 0 {
		return nil
	}

	objElemSize, _ := mem.ReadU16(r.ohdBase + OffOHDElementSize)
	objHeaderFirst, _ := mem.ReadU32(r.ohdBase + OffOHDFirstElement)
	objAllocCount, _ := mem.ReadU16(r.ohdBase + OffOHDAllocCount)
	if objHeaderFirst < HighGVAThreshold || objElemSize == 0 {
		return nil
	}

	out := make([]scraper.TickProjectile, 0)
	for i := uint16(0); i < objAllocCount; i++ {
		entryAddr := objHeaderFirst + uint32(i)*uint32(objElemSize)
		objDataAddr, _ := mem.ReadU32(entryAddr + OffObjEntryDataAddr)
		if objDataAddr < HighGVAThreshold {
			continue
		}
		flags, _ := mem.ReadU32(objDataAddr + OffObjFlags)
		if flags&ObjFlagGarbage != 0 {
			continue
		}
		objType, _ := mem.ReadU8(objDataAddr + OffObjType)
		if objType != ObjectTypeProjectile {
			continue
		}

		tagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
		tagName, _ := r.readTagName(tagIdx)
		x, _ := mem.ReadF32(objDataAddr + OffObjX)
		y, _ := mem.ReadF32(objDataAddr + OffObjY)
		z, _ := mem.ReadF32(objDataAddr + OffObjZ)

		// Projectile sub-struct base.
		pBase := objDataAddr + uint32(itemDatumSize)

		pFlags, _ := mem.ReadU32(pBase + OffProjFlags)
		action, _ := mem.ReadS16(pBase + OffProjAction)
		hitMat, _ := mem.ReadS16(pBase + OffProjHitMaterialType)
		ignoreObj, _ := mem.ReadS32(pBase + OffProjIgnoreObjectIndex)
		detTimer, _ := mem.ReadF32(pBase + OffProjDetonationTimer)
		detTimerDelta, _ := mem.ReadF32(pBase + OffProjDetonationTimerDelta)
		// 0x1C: HC bug — read as s32 here (target_object_index). M7 to confirm.
		targetIdx, _ := mem.ReadS32(pBase + OffProjTargetObjectIndex)
		armTimeDelta, _ := mem.ReadF32(pBase + OffProjArmingTimeDelta)
		distTraveled, _ := mem.ReadF32(pBase + OffProjDistanceTraveled)
		decelTimer, _ := mem.ReadF32(pBase + OffProjDecelerationTimer)
		decelTimerDelta, _ := mem.ReadF32(pBase + OffProjDecelerationTimerDelta)
		decel, _ := mem.ReadF32(pBase + OffProjDeceleration)
		maxDmgDist, _ := mem.ReadF32(pBase + OffProjMaximumDamageDistance)
		rotAxisX, _ := mem.ReadF32(pBase + OffProjRotationAxisX)
		rotAxisY, _ := mem.ReadF32(pBase + OffProjRotationAxisY)
		rotAxisZ, _ := mem.ReadF32(pBase + OffProjRotationAxisZ)
		rotSin, _ := mem.ReadF32(pBase + OffProjRotationSine)
		rotCos, _ := mem.ReadF32(pBase + OffProjRotationCosine)

		out = append(out, scraper.TickProjectile{
			ObjectID:               uint32(i),
			Tag:                    tagName,
			X:                      x,
			Y:                      y,
			Z:                      z,
			Flags:                  pFlags,
			Action:                 action,
			HitMaterialType:        hitMat,
			IgnoreObjectIndex:      ignoreObj,
			DetonationTimer:        detTimer,
			DetonationTimerDelta:   detTimerDelta,
			TargetObjectIndex:      targetIdx,
			ArmingTimeDelta:        armTimeDelta,
			DistanceTraveled:       distTraveled,
			DecelerationTimer:      decelTimer,
			DecelerationTimerDelta: decelTimerDelta,
			Deceleration:           decel,
			MaximumDamageDistance:  maxDmgDist,
			RotationAxisX:          rotAxisX,
			RotationAxisY:          rotAxisY,
			RotationAxisZ:          rotAxisZ,
			RotationSine:           rotSin,
			RotationCosine:         rotCos,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
