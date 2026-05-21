// View-model for the Probe tab: shapes the v2 probe-envelope payload's
// free-form score_probe and state_inputs maps into sorted entry lists,
// plus a shared formatProbeValue helper used by both views. Plain-TS
// so the file isn't a runes module — reactivity is set up at the call
// site with $derived.by().
//
// Source is `scraperWSV2.probe[name]` — populated by requestProbe()
// (request/reply, never broadcast). score_probe is the bag dumped by
// the GameReader plugin's BuildScoreProbe (every candidate address it
// reads); state_inputs is the per-tick state-classification inputs
// LastStateInputs() returns (booleans / pointers / mode strings).
// Both are intentionally untyped on the wire so a human can spot
// which raw value matches what's on-screen.

import type { ProbePayload } from '$lib/types/scraper-v2';
import type { ScoreProbe, StateInputs } from '$lib/types/scraper';

// Pointer-shaped numbers (Xbox VAs 0x80000000+ or large addresses) render as
// both decimal and hex; the same threshold as StateInputsGrid keeps the two
// views consistent when state_inputs is shown side-by-side.
const POINTER_THRESHOLD = 0x10000;

export type ProbeEntry = [string, unknown];

export type ProbeVm = {
	scoreProbe: ScoreProbe | null;
	stateInputs: StateInputs | null;
	scoreEntries: ProbeEntry[];
	stateInputEntries: ProbeEntry[];
};

function sortedEntries(rec: Record<string, unknown> | null | undefined): ProbeEntry[] {
	if (!rec) return [];
	return Object.entries(rec).sort(([a], [b]) => a.localeCompare(b));
}

export function buildProbeVm(payload: ProbePayload | null): ProbeVm {
	const scoreProbe = (payload?.score_probe ?? null) as ScoreProbe | null;
	const stateInputs = (payload?.state_inputs ?? null) as StateInputs | null;
	return {
		scoreProbe,
		stateInputs,
		scoreEntries: sortedEntries(scoreProbe),
		stateInputEntries: sortedEntries(stateInputs)
	};
}

// Display-friendly representation of a probe value. Pointer-shaped numbers
// render with both decimal and hex so the operator can paste either form
// into a memory viewer; smaller integers stay decimal; floats truncate to
// 4 decimals; booleans become on/off; objects fall back to JSON.
export function formatProbeValue(v: unknown): string {
	if (v === null || v === undefined) return '—';
	if (typeof v === 'boolean') return v ? 'on' : 'off';
	if (typeof v === 'number') {
		if (!Number.isFinite(v)) return String(v);
		if (Number.isInteger(v) && Math.abs(v) >= POINTER_THRESHOLD) {
			return `${v} (0x${v.toString(16)})`;
		}
		return Number.isInteger(v) ? String(v) : v.toFixed(4);
	}
	if (typeof v === 'string') return v.length === 0 ? '—' : v;
	try {
		return JSON.stringify(v);
	} catch {
		return String(v);
	}
}
