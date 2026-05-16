package manager

import "time"

// envelopeTypeXbox is the wire type for the per-instance xbox class —
// Xbox machine + XBE identity + kernel clock. Change-driven; rare.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`xbox`).
const envelopeTypeXbox = "xbox"

// XboxPayload is the data for an xbox-class envelope.
type XboxPayload struct {
	TitleID       uint32        `json:"title_id"`
	Title         string        `json:"title"`
	Name          string        `json:"name"`
	SerialNumber  string        `json:"serial_number"`
	MACAddress    string        `json:"mac_address"`
	VideoStandard string        `json:"video_standard"`
	TimeZone      *XboxTimeZone `json:"time_zone"`
	XBE           *XboxXBE      `json:"xbe"`
	Kernel        *XboxKernel   `json:"kernel"`
}

// XboxTimeZone is the EEPROM time-zone record. Nesting it (rather than
// flattening bias_minutes / std_name / dlt_name onto XboxPayload directly)
// means bias_minutes = 0 (GMT) stays present on the wire — the surrounding
// object's presence signals "read", not any one field's non-zero value.
type XboxTimeZone struct {
	BiasMinutes int32  `json:"bias_minutes"`
	StdName     string `json:"std_name"`
	DltName     string `json:"dlt_name"`
}

// XboxXBE is the XBE certificate fields. Nested for the same reason as
// TimeZone — preserves zero-valid fields like disk_number = 0.
type XboxXBE struct {
	TitleName    string `json:"title_name"`
	Version      uint32 `json:"version"`
	GameRegion   uint32 `json:"game_region"`
	DiskNumber   uint32 `json:"disk_number"`
	AllowedMedia uint32 `json:"allowed_media"`
}

// XboxKernel is the kernel-clock triad. UptimeSeconds is a float for
// human readability rather than the 14-digit nanosecond int that v1
// shipped (`kernel_uptime_ns`).
type XboxKernel struct {
	SystemTime    time.Time `json:"system_time"`
	BootTime      time.Time `json:"boot_time"`
	UptimeSeconds float64   `json:"uptime_seconds"`
}
