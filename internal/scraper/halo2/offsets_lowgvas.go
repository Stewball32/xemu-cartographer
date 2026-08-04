package halo2

// AllLowGVAs returns the low guest VAs this instance's offset set pre-translates
// at Instance.Init time — the versioned equivalent of the package-level
// AllLowGVAs constant list (same composition, values from the bound set).
func (o *Offsets) AllLowGVAs() []uint32 {
	return []uint32{
		o.AddrH2PlayersArrayPtr,
		o.AddrH2ObjectArrayPtr,
		o.AddrH2TagHeaderPtr,
		o.AddrH2ScenarioNamePoolPtr,
	}
}
