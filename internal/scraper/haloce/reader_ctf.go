package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readCTFFlags reads the two CTF flag-base positions
// (at *RefAddrCTFFlag0Ptr / *RefAddrCTFFlag1Ptr). Returns nil when neither
// pointer is set.
//
// Carrier detection (Status / CarrierIndex) is intentionally minimal in this
// pass — emits the static base positions only with Status="home". Proper
// carrier tracking requires identifying the flag tag and integrating with
// the world-object scan / player weapon-slot map; deferred to M7 alongside
// gametype detection so the CTF gating is also reliable.
//
// Source: RefAddrCTFFlag0Ptr / RefAddrCTFFlag1Ptr + OffCTFFlag* constants.
func (r *Reader) readCTFFlags() []scraper.TickCTFFlag {
	inst := r.inst
	mem := inst.Mem

	read := func(team uint32, addr uint32) (scraper.TickCTFFlag, bool) {
		base, err := inst.DerefLowPtr(addr)
		if err != nil || base < HighGVAThreshold {
			return scraper.TickCTFFlag{}, false
		}
		x, _ := mem.ReadF32(base + OffCTFFlagX)
		y, _ := mem.ReadF32(base + OffCTFFlagY)
		z, _ := mem.ReadF32(base + OffCTFFlagZ)
		return scraper.TickCTFFlag{
			Team:   team,
			X:      x,
			Y:      y,
			Z:      z,
			Status: "home",
		}, true
	}

	out := make([]scraper.TickCTFFlag, 0, 2)
	if f, ok := read(0, RefAddrCTFFlag0Ptr); ok {
		out = append(out, f)
	}
	if f, ok := read(1, RefAddrCTFFlag1Ptr); ok {
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
