package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The xbox class payload shapes are owned by the wire contract package
// (xc-scraper/wire/xbox.go); the names below are aliases so the builders
// in v2_adapters.go and every consumer keep one type identity.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`xbox`).
type (
	// XboxPayload is the data for an xbox-class envelope — Xbox machine +
	// XBE identity + kernel clock. Change-driven; rare.
	XboxPayload = wire.XboxPayload
	// XboxTimeZone is the EEPROM time-zone record.
	XboxTimeZone = wire.XboxTimeZone
	// XboxXBE is the XBE certificate fields.
	XboxXBE = wire.XboxXBE
	// XboxKernel is the kernel-clock triad.
	XboxKernel = wire.XboxKernel
)
