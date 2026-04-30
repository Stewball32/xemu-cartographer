package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readWeaponObjectExtended reads per-tick weapon-object diagnostic state
// (heat / used energy / owner handle / reload state / world position).
// World position is only meaningful when the weapon is dropped (no owner);
// guarded by OffWepOwnerHandle == HandleEmpty.
//
// Source: extended OffWep* constants in offsets.go.
func (r *Reader) readWeaponObjectExtended(objDataAddr uint32) *scraper.TickWeaponObjectExtended {
	mem := r.inst.Mem
	ext := &scraper.TickWeaponObjectExtended{}

	ext.HeatMeter, _ = mem.ReadF32(objDataAddr + OffWepHeatMeter)
	ext.UsedEnergy, _ = mem.ReadF32(objDataAddr + OffWepUsedEnergy)
	ext.OwnerHandle, _ = mem.ReadU32(objDataAddr + OffWepOwnerHandle)
	ext.IsReloading, _ = mem.ReadU8(objDataAddr + OffWepIsReloading)
	ext.CanFire, _ = mem.ReadU8(objDataAddr + OffWepCanFire)
	ext.ReloadTime, _ = mem.ReadS16(objDataAddr + OffWepReloadTime)

	if ext.OwnerHandle == HandleEmpty {
		x, _ := mem.ReadF32(objDataAddr + OffWepObjX)
		y, _ := mem.ReadF32(objDataAddr + OffWepObjY)
		z, _ := mem.ReadF32(objDataAddr + OffWepObjZ)
		ext.World = &scraper.XYZ{X: x, Y: y, Z: z}
	}

	return ext
}

// readWeaponTagData returns the static weapon-tag metadata (zoom levels,
// autoaim/magnetism parameters) for one tag, populating the per-Reader cache
// on first access. Subsequent calls return the cached pointer.
//
// Source: OffWepTag* constants in offsets.go.
func (r *Reader) readWeaponTagData(tagIdx int16) *scraper.StaticWeaponTagData {
	if cached, ok := r.weaponTagDataCache[tagIdx]; ok {
		return cached
	}
	if r.tagInstBase < HighGVAThreshold {
		return nil
	}
	mem := r.inst.Mem

	tagInstEntry := r.tagInstBase + uint32(TagInstStride)*uint32(uint16(tagIdx))
	tagDataPtr, _ := mem.ReadU32(tagInstEntry + OffTagDataPtr)
	if tagDataPtr < HighGVAThreshold {
		return nil
	}

	td := &scraper.StaticWeaponTagData{}
	td.ZoomLevels, _ = mem.ReadS16(tagDataPtr + OffWepTagZoomLevels)
	td.ZoomMin, _ = mem.ReadF32(tagDataPtr + OffWepTagZoomMin)
	td.ZoomMax, _ = mem.ReadF32(tagDataPtr + OffWepTagZoomMax)
	td.AutoaimAngle, _ = mem.ReadF32(tagDataPtr + OffWepTagAutoaimAngle)
	td.AutoaimRange, _ = mem.ReadF32(tagDataPtr + OffWepTagAutoaimRange)
	td.MagnetismAngle, _ = mem.ReadF32(tagDataPtr + OffWepTagMagnetismAngle)
	td.MagnetismRange, _ = mem.ReadF32(tagDataPtr + OffWepTagMagnetismRange)
	td.DeviationAngle, _ = mem.ReadF32(tagDataPtr + OffWepTagDeviationAngle)
	td.Animations = r.readAnimEntries(tagDataPtr)

	r.weaponTagDataCache[tagIdx] = td
	return td
}
