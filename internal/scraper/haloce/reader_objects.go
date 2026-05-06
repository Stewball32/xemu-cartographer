package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readObjects walks the OHD object array and returns one TickObject per
// non-garbage allocated entry. Captures the full per-object generic state:
// position, angular velocity, type, flags, tag name, time-existing, owner
// references, ultimate-parent.
//
// Source: OffObj* constants in offsets.go. The same alloc loop pattern is
// also used by readPowerItemStatus / readPowerItemSpawns for power-item
// detection — kept duplicated for now so the per-tick power-item path stays
// self-contained.
func (r *Reader) readObjects() []scraper.TickObject {
	if r.ohdBase < HighGVAThreshold {
		return nil
	}
	mem := r.inst.Mem

	objElemSize, _ := mem.ReadU16(r.ohdBase + OffOHDElementSize)
	objHeaderFirst, _ := mem.ReadU32(r.ohdBase + OffOHDFirstElement)
	objAllocCount, _ := mem.ReadU16(r.ohdBase + OffOHDAllocCount)
	if objHeaderFirst < HighGVAThreshold || objElemSize == 0 {
		return nil
	}

	out := make([]scraper.TickObject, 0, objAllocCount)
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

		tagIdx, _ := mem.ReadS16(objDataAddr + OffObjTagIndex)
		tagName, _ := r.readTagName(tagIdx)
		objType, _ := mem.ReadU8(objDataAddr + OffObjType)

		x, _ := mem.ReadF32(objDataAddr + OffObjX)
		y, _ := mem.ReadF32(objDataAddr + OffObjY)
		z, _ := mem.ReadF32(objDataAddr + OffObjZ)
		angVelX, _ := mem.ReadF32(objDataAddr + OffObjAngVelX)
		angVelY, _ := mem.ReadF32(objDataAddr + OffObjAngVelY)
		angVelZ, _ := mem.ReadF32(objDataAddr + OffObjAngVelZ)
		unkDamage1, _ := mem.ReadS16(objDataAddr + OffObjUnkDamage1)
		timeExisting, _ := mem.ReadS16(objDataAddr + OffObjTimeExisting)
		ownerUnitRef, _ := mem.ReadU32(objDataAddr + OffObjOwnerUnitRef)
		ownerObjectRef, _ := mem.ReadU32(objDataAddr + OffObjOwnerObjectRef)
		ultimateParent, _ := mem.ReadU32(objDataAddr + OffObjUltimateParent)

		out = append(out, scraper.TickObject{
			ObjectID:       uint32(i),
			Tag:            tagName,
			Type:           objType,
			Flags:          flags,
			X:              x,
			Y:              y,
			Z:              z,
			AngVelX:        angVelX,
			AngVelY:        angVelY,
			AngVelZ:        angVelZ,
			UnkDamage1:     unkDamage1,
			TimeExisting:   timeExisting,
			OwnerUnitRef:   ownerUnitRef,
			OwnerObjectRef: ownerObjectRef,
			UltimateParent: ultimateParent,
		})
	}
	return out
}
