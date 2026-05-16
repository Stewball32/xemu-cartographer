// View-model for the System tab's runtime sections — WS connection state,
// runner lifecycle counters, cross-instance summary entry, envelope
// freshness ages. Reads from scraperWSV2 with inspect-endpoint fallbacks
// for the first-paint window before v2 envelopes flow.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { DebugContext } from '../context';
import type { GamePayload, HostSummaryV2, PhaseV2 } from '$lib/types/scraper-v2';

type ScraperWSV2 = typeof scraperWSV2;

export type RuntimeVm = {
	// Connection.
	wsConnected: boolean;
	lastError: string | null;

	// Lifecycle.
	phase: PhaseV2;
	startedAt: string | undefined;
	runnerUptimeStr: string;
	lastReadStr: string;
	engineTick: number | undefined;
	iterations: number | undefined;

	// Cross-instance summary entry (populated when the page subscribes
	// to host:summary — SystemTab does so on mount).
	hostSummary: HostSummaryV2 | null;

	// Envelope freshness.
	gameDataStr: string;
	tickStr: string;
	lastEventsReplyStr: string;
	lastEventsReplyPhase: PhaseV2 | undefined;
	hostSummaryReadStr: string;

	connection: Record<string, unknown>;
	lifecycle: Record<string, unknown> | null;
	summary: Record<string, unknown> | null;
	freshness: Record<string, unknown>;
};

function parseIsoMs(iso: string | undefined | null): number | undefined {
	if (!iso) return undefined;
	const t = Date.parse(iso);
	return Number.isFinite(t) ? t : undefined;
}

function formatUptime(ms: number | undefined): string {
	if (ms === undefined || !Number.isFinite(ms) || ms < 0) return '—';
	let s = Math.floor(ms / 1000);
	const days = Math.floor(s / 86400);
	s -= days * 86400;
	const hh = Math.floor(s / 3600)
		.toString()
		.padStart(2, '0');
	s -= Number(hh) * 3600;
	const mm = Math.floor(s / 60)
		.toString()
		.padStart(2, '0');
	const ss = (s - Number(mm) * 60).toString().padStart(2, '0');
	return days > 0 ? `${days}d ${hh}:${mm}:${ss}` : `${hh}:${mm}:${ss}`;
}

export function buildRuntimeVm(name: string, ws: ScraperWSV2, ctx: DebugContext): RuntimeVm {
	const game: GamePayload | null = ws.game[name] ?? null;

	// Phase + started_at + counters come from the game class in v2 (v1's
	// CurrentStateSnapshot was a per-instance bag; v2 splits responsibilities
	// by class).
	const phase: PhaseV2 = game?.phase ?? (ctx.inspect?.phase as PhaseV2 | undefined) ?? 'idle';
	const startedAt = game?.started_at ?? ctx.inspect?.started_at;
	const startedAtMs = parseIsoMs(startedAt);
	const runnerUptimeStr = startedAtMs !== undefined ? formatUptime(ctx.now - startedAtMs) : '—';
	const lastReadAtIso = game?.last_read_at ?? ctx.inspect?.last_read_at;
	const lastReadAtMs = parseIsoMs(lastReadAtIso);
	const lastReadStr = ctx.relativeTime(lastReadAtMs);
	const engineTick = game?.engine_tick ?? ws.engineTick[name] ?? ctx.inspect?.tick;
	const iterations = game?.iterations ?? ctx.inspect?.ticks;

	const hostSummary: HostSummaryV2 | null = ws.hostSummaries[name] ?? null;
	const hostSummaryReadMs = parseIsoMs(hostSummary?.last_successful_read_at);
	const hostSummaryReadStr = ctx.relativeTime(hostSummaryReadMs);

	// Game-class envelopes fire on every poll; tick-class on every poll
	// during Live. The per-class At maps are the freshness source.
	const gameDataStr = ctx.relativeTime(ws.gameAt[name]);
	const tickStr = ctx.relativeTime(ws.tickAt[name]);
	const lastEventsReplyStr = ctx.relativeTime(ws.lastEventsReplyAt[name]);
	const lastEventsReplyPhase = ws.lastEventsReplyPhase[name] as PhaseV2 | undefined;

	const connection: Record<string, unknown> = {
		ws_connected: ws.connected ? 'connected' : 'disconnected',
		last_error: ws.lastError ?? '—'
	};

	const lifecycle: Record<string, unknown> = {
		phase,
		started_at: startedAt ?? '—',
		runner_uptime: runnerUptimeStr,
		last_read_at: lastReadAtIso ?? '—',
		last_read_age: lastReadStr,
		engine_tick: engineTick ?? '—',
		iterations: iterations ?? '—'
	};

	const summary: Record<string, unknown> | null = hostSummary
		? {
				instance: hostSummary.instance,
				phase: hostSummary.phase,
				title: hostSummary.title || '—',
				map: hostSummary.map || '—',
				gametype: hostSummary.gametype || '—',
				score_summary: hostSummary.score_summary || '—',
				last_successful_read_at: hostSummary.last_successful_read_at || '—',
				last_successful_read_age: hostSummaryReadStr
			}
		: null;

	const freshness: Record<string, unknown> = {
		game_data_age: gameDataStr,
		tick_age: tickStr,
		events_reply_age: lastEventsReplyStr,
		events_reply_phase: lastEventsReplyPhase ?? '—'
	};

	return {
		wsConnected: ws.connected,
		lastError: ws.lastError,
		phase,
		startedAt,
		runnerUptimeStr,
		lastReadStr,
		engineTick,
		iterations,
		hostSummary,
		gameDataStr,
		tickStr,
		lastEventsReplyStr,
		lastEventsReplyPhase,
		hostSummaryReadStr,
		connection,
		lifecycle,
		summary,
		freshness
	};
}
