// Pure logic for the player-hosting play tab. Keeps the phase decision + the
// /api/play/* response shapes out of the Svelte components so they unit-test
// without a socket or a running box.
//
// The play tab is phase-aware off the LIVE scraper WS `game` feed (phase =
// idle | ready | live) combined with two REST reads: GET /api/play/current
// (which box the caller's gamertag resolves to server-side + host-runner
// status) and the locally-remembered POST /api/play/request result (so the UI
// can show "booting your box" before the roster resolves). See
// internal/pocketbase/routes/play.

import type { PhaseV2 } from '$lib/types/scraper-v2';

// =============================================================================
// /api/play/* response shapes (mirror internal/pocketbase/routes/play)
// =============================================================================

/** hostrunner.Status — the arbitration + selection view GET /current returns. */
export interface PlayStatus {
	instance: string;
	present: boolean;
	authority: string; // "runner" | "admin" | "disabled"
	screen?: string;
	last_kind?: string;
	last_intent?: string;
	last_keys?: string[];
	tick: number;
	machine_count: number;
	team_count: number;
	countdown_active: boolean;
	ready_to_start: boolean;
	map?: string;
	gametype?: string;
	selected_map?: string;
	selected_gametype?: string;
	selected: boolean;
	ready: boolean;
}

/** The idle-out reaper's per-instance countdown, present on /current only while
 * the box is accruing idle time (empty lobby, no live match). */
export interface ReapView {
	reap_at: string;
	seconds_remaining: number;
	warning: boolean;
}

export interface CurrentResponse {
	/** "" when the caller isn't in any live roster (idle / box still booting). */
	instance: string;
	status: PlayStatus | null;
	reap?: ReapView | null;
}

/** One selectable map or gametype, enumerated LIVE from the instance's disc. */
export interface MapOption {
	name: string;
	steps: number;
}

export interface OptionsResponse {
	instance: string;
	/** false when the box can't be enumerated yet — the UI must NOT substitute a
	 * stock list (modded discs make any hardcoded set wrong); fall back to
	 * free-form entry. */
	available: boolean;
	maps: MapOption[];
	gametypes: MapOption[];
	selected_map: string;
	selected_gametype: string;
}

/** A game the player may request an instance for (GET /api/play/isos). */
export interface IsoOption {
	id: string;
	name: string;
	title_id: string;
	description: string;
}

/** POST /api/play/request (201) — the freshly-provisioned box. */
export interface RequestResponse {
	instance: string;
	index: number;
	iso: string;
	title_id: string;
	game_iso: string;
}

// =============================================================================
// Phase model
// =============================================================================

/** The five player-hosting phases the tab renders.
 *  - catalog:      no box yet → pick a game + request one
 *  - provisioning: requested a box, waiting for it to boot + resolve
 *  - lobby:        box up, in menus → pick map/gametype, ready up, see joiners
 *  - live:         a match is running → live scoreboard
 *  - postgame:     a match just ended → final scores, back to lobby */
export type PlayHostPhase = 'catalog' | 'provisioning' | 'lobby' | 'live' | 'postgame';

export interface PlayHostInputs {
	/** Instance GET /current resolved for the caller ("" / null = unresolved). */
	resolvedInstance: string | null;
	/** Instance name from the last successful POST /request (local hint). */
	pendingInstance: string | null;
	/** WS `game` feed phase for the active instance (null = no feed yet). */
	gamePhase: PhaseV2 | null;
	/** WS `previous_game` present for the active instance (a match just ended). */
	hasPreviousGame: boolean;
	/** Player dismissed the post-game card (→ back to the lobby). */
	postgameDismissed: boolean;
}

/** The instance the tab should drive (WS subscription + control POSTs): the
 * server-resolved box when known, else the locally-remembered requested box so
 * the provisioning UI has something to show. */
export function activeInstance(
	i: Pick<PlayHostInputs, 'resolvedInstance' | 'pendingInstance'>
): string | null {
	return i.resolvedInstance || i.pendingInstance || null;
}

/** Reduce the current inputs to a single phase. Pure + total. */
export function derivePlayHostPhase(i: PlayHostInputs): PlayHostPhase {
	const active = activeInstance(i);
	if (!active) return 'catalog';
	// Requested but the server hasn't resolved the caller into its roster yet —
	// the box is still booting / the host isn't recognised on it.
	if (!i.resolvedInstance && i.pendingInstance) return 'provisioning';
	// Resolved box:
	if (i.gamePhase === 'live') return 'live';
	if (i.hasPreviousGame && !i.postgameDismissed) return 'postgame';
	return 'lobby';
}

export const INITIAL_INPUTS: PlayHostInputs = {
	resolvedInstance: null,
	pendingInstance: null,
	gamePhase: null,
	hasPreviousGame: false,
	postgameDismissed: false
};

// =============================================================================
// Helpers
// =============================================================================

/** Normalise a /current response's instance ("" → null). */
export function resolvedFromCurrent(resp: CurrentResponse | null | undefined): string | null {
	const inst = resp?.instance;
	return typeof inst === 'string' && inst.length > 0 ? inst : null;
}

/** Format a countdown of whole seconds as "Ns", "Nm", or "Nm Ss" for the reap
 * heads-up. Negative / non-finite clamps to "0s". */
export function formatCountdown(totalSeconds: number): string {
	const s = Number.isFinite(totalSeconds) ? Math.max(0, Math.floor(totalSeconds)) : 0;
	if (s < 60) return `${s}s`;
	const m = Math.floor(s / 60);
	const rem = s % 60;
	return rem === 0 ? `${m}m` : `${m}m ${rem}s`;
}

/** Whether the caller can currently drive the box (the control POSTs only work
 * when the host-runner is the authority — an admin take-over or disabled
 * hosting locks the player out). */
export function isPlayerControllable(status: PlayStatus | null | undefined): boolean {
	return !!status && status.present && status.authority === 'runner';
}
