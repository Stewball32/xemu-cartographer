// View-model for the Tick tab: surfaces the live TickPayload streamed via
// scraperWS, falling back to the REST inspect snapshot when WS data isn't
// available yet. Plain-TS so the file isn't a runes module — the reactivity
// is established at the call site by wrapping in $derived.by().
//
// The Tick tab is the most volatile of the debug surfaces: every field of
// TickPayload should have a home so the operator can confirm that a struct
// is being read correctly. Sub-sections (engine flags / locals / network /
// players) each pull one slice of the payload.
//
// See `internal/scraper/types.go` Tick* types for the source of truth.

import { scraperWS } from '$lib/stores/scraper-ws.svelte';
import type { DebugContext } from '../context';
import type { GamePlayer, Phase, TickPayload } from '$lib/types/scraper';
import type { KvField } from '../shared/KvGrid.svelte';

type ScraperWS = typeof scraperWS;

// Render a plain Record into KvField[] for KvGrid. Annotation keys are
// derived from the prefix so every field in a sub-struct gets a stable
// pill identity (debug.tick.<section>.<field>). Accepts any object —
// the generated TS structs (TickNetworkClient, TickPlayerExtended, ...)
// don't carry a string index signature, so we widen via `object` rather
// than force every caller to cast.
export function recordToFields(
	value: object | null | undefined,
	annotationPrefix: string
): KvField[] {
	if (!value) return [];
	return Object.entries(value).map(([k, v]) => ({
		key: k,
		value: v,
		annotationKey: `${annotationPrefix}.${k}`
	}));
}

export type TickVm = {
	tick: TickPayload | null;
	phase: Phase;
	tickValue: number | undefined;
	tickStr: string;
	isTeamGame: boolean;
	playersByIndex: Map<number, GamePlayer>;
};

export function buildTickVm(name: string, ws: ScraperWS, ctx: DebugContext): TickVm {
	const tick = ws.ticks[name] ?? ctx.inspect?.latest_tick ?? null;
	const gameData = ws.gameData[name] ?? ctx.inspect?.game_data ?? null;
	const phase: Phase = ws.phases[name] ?? ctx.inspect?.phase ?? 'idle';
	const tickValue = ws.tickNumbers[name] ?? ctx.inspect?.tick;

	// Index roster players by tick-player index so a TickPlayer row can pull
	// its identity (name/team) without iterating the full roster each time.
	const playersByIndex = new Map<number, GamePlayer>();
	for (const p of gameData?.players ?? []) playersByIndex.set(p.index, p);

	return {
		tick,
		phase,
		tickValue,
		tickStr: ctx.relativeTime(ws.ticksAt[name]),
		isTeamGame: gameData?.is_team_game === true,
		playersByIndex
	};
}
