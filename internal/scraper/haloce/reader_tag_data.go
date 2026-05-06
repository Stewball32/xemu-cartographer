package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// AnimEntriesScanCap is the maximum number of animation entries we'll walk
// for a single tag's animation array. The engine stores arbitrarily many
// animations per tag; in practice tags have <50 anims. Capping conservatively
// avoids runaway reads if the array pointer or stride is wrong.
const AnimEntriesScanCap = 64

// readAnimEntries walks the animation array attached to a tag-data block and
// returns up to AnimEntriesScanCap valid entries. Stops early when an entry
// has zero/negative length (treated as the end of the array).
//
// Source: OffAnim* constants. Called from readWeaponTagData and (in future)
// any other tag-walker that wants animation metadata.
func (r *Reader) readAnimEntries(tagDataPtr uint32) []scraper.AnimEntry {
	if tagDataPtr < HighGVAThreshold {
		return nil
	}
	mem := r.inst.Mem

	arrayPtr, _ := mem.ReadU32(tagDataPtr + OffAnimTagArrayPtr)
	if arrayPtr < HighGVAThreshold {
		return nil
	}

	out := make([]scraper.AnimEntry, 0, AnimEntriesScanCap)
	for i := 0; i < AnimEntriesScanCap; i++ {
		entry := arrayPtr + uint32(i)*AnimEntryStride
		length, _ := mem.ReadS16(entry + OffAnimLength)
		if length <= 0 {
			break
		}
		u46, _ := mem.ReadS16(entry + OffAnimUnk46)
		u52, _ := mem.ReadS16(entry + OffAnimUnk52)
		u54, _ := mem.ReadS16(entry + OffAnimUnk54)
		out = append(out, scraper.AnimEntry{
			Index:  i,
			Length: length,
			Unk46:  u46,
			Unk52:  u52,
			Unk54:  u54,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// readBipedTagData returns the static biped-tag metadata (Flags +
// AutoaimPillRadius) for one tag index, populating the per-Reader cache on
// first access.
//
// Source: OffBipedTag* constants.
func (r *Reader) readBipedTagData(tagIdx int16) *scraper.StaticBipedTagData {
	if cached, ok := r.bipedTagCache[tagIdx]; ok {
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

	flags, _ := mem.ReadU32(tagDataPtr + OffBipedTagFlags)
	radius, _ := mem.ReadF32(tagDataPtr + OffBipedTagAutoaimPillRadius)

	td := &scraper.StaticBipedTagData{
		TagIndex:          tagIdx,
		Flags:             flags,
		AutoaimPillRadius: radius,
	}
	r.bipedTagCache[tagIdx] = td
	return td
}
