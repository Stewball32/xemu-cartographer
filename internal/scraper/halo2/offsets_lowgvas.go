package halo2

// AllLowGVAs returns the low guest VAs this instance's offset set pre-translates
// at Instance.Init time — the versioned equivalent of the package-level
// AllLowGVAs constant list (same composition, values from the bound set).
// Zero-valued addresses are SKIPPED: 0 is the "not mapped on this build"
// sentinel (e.g. AddrH2GamePhase on stock), and translating GVA 0 would fail
// Init and keep the whole reader from binding.
func (o *Offsets) AllLowGVAs() []uint32 {
	candidates := []uint32{
		o.AddrH2PlayersArrayPtr,
		o.AddrH2ObjectArrayPtr,
		o.AddrH2TagHeaderPtr,
		o.AddrH2ScenarioNamePoolPtr,
		o.AddrH2KillsPerPlayer,
		o.AddrH2DeathsPerPlayer,
		o.AddrH2KillsTotal,
		o.AddrH2Gametype,
		o.AddrH2GamePhase,
		o.AddrH2NetLocalMachineIndex,
		o.AddrH2NetMachineMacArray,
		o.AddrH2NetMachineTable,
	}
	out := make([]uint32, 0, len(candidates))
	for _, gva := range candidates {
		if gva != 0 {
			out = append(out, gva)
		}
	}
	return out
}
