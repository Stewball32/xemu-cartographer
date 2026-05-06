package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readBones reads the 19 model-node bone positions from a dynamic biped.
// Each bone is an xyz triple of f32 at one of ModelNodeBoneOffsets.
// 57 memory reads per player per tick — the heaviest per-player addition.
//
// Source: ModelNodeBoneOffsets in offsets.go.
func (r *Reader) readBones(objDataAddr uint32) []scraper.TickBone {
	mem := r.inst.Mem
	out := make([]scraper.TickBone, 0, len(ModelNodeBoneOffsets))
	for i, off := range ModelNodeBoneOffsets {
		x, _ := mem.ReadF32(objDataAddr + off)
		y, _ := mem.ReadF32(objDataAddr + off + 4)
		z, _ := mem.ReadF32(objDataAddr + off + 8)
		out = append(out, scraper.TickBone{
			Index: i,
			X:     x,
			Y:     y,
			Z:     z,
		})
	}
	return out
}
