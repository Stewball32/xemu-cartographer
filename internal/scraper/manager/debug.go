package manager

import (
	"encoding/json"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// envelopeTypeDebug is the wire type for the per-instance debug class —
// opt-in, only read+emitted when a consumer (WS or capture policy)
// demands it. The honest home for everything reverse-engineered-but-not-
// understood: known-semantic fields under `extended`, undecoded offsets
// under `raw` keyed by hex offset.
//
// state_inputs / score_probe used to ride on this envelope; both moved
// to the on-demand probe class (see probe.go) so probe work doesn't
// run per-tick.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`debug`) and §10 (the
// raw convention).
const envelopeTypeDebug = "debug"

// DebugPayload is the data for a debug-class envelope.
type DebugPayload struct {
	Players []DebugPlayer `json:"players"`
}

type DebugPlayer struct {
	Index       int                  `json:"index"`
	Extended    *DebugPlayerExtended `json:"extended,omitempty"`
	Bones       []DebugBone          `json:"bones,omitempty"`
	UpdateQueue *DebugUpdateQueue    `json:"update_queue,omitempty"`
	// Raw holds undecoded biped-extended fields keyed by their hex memory
	// offset (e.g. "0x98", "0x460"). When a field's semantic is figured
	// out it moves to DebugPlayerExtended with a real name and the offset
	// key disappears. See §10.
	Raw map[string]json.RawMessage `json:"raw,omitempty"`
}

// DebugPlayerExtended is the curated subset of biped-extended fields that
// have a known semantic name. New fields graduate from DebugPlayer.Raw to
// here as they're decoded.
type DebugPlayerExtended struct {
	Legs            DebugLegRotation `json:"legs"`
	Airborne        bool             `json:"airborne"`
	AirborneTicks   uint8            `json:"airborne_ticks"`
	IdleTicks       int16            `json:"idle_ticks"`
	StateFlags      uint8            `json:"state_flags"`
	Stunned         float32          `json:"stunned"`
	AimAssistSphere *DebugSphere     `json:"aim_assist_sphere,omitempty"`
}

type DebugLegRotation struct {
	Pitch float32 `json:"pitch"`
	Yaw   float32 `json:"yaw"`
	Roll  float32 `json:"roll"`
}

type DebugSphere struct {
	Center scraper.Vec3 `json:"center"`
	Radius float32      `json:"radius"`
}

type DebugBone struct {
	Index int          `json:"index"`
	Pos   scraper.Vec3 `json:"pos"`
}

type DebugUpdateQueue struct {
	Buttons      DebugUpdateQueueButtons `json:"buttons"`
	DesiredYaw   float32                 `json:"desired_yaw"`
	DesiredPitch float32                 `json:"desired_pitch"`
}

type DebugUpdateQueueButtons struct {
	Crouch bool `json:"crouch"`
	Jump   bool `json:"jump"`
	Fire   bool `json:"fire"`
}
