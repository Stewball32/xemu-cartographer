// View-model for the Runtime tab — the "stuff that doesn't fit Game / Tick /
// Xbox" bucket: WS connection state, runner lifecycle counters, cross-instance
// summary entry, and envelope freshness ages. Pure TS; reactivity is wired by
// $derived.by() at the call site.

import { scraperWS } from '$lib/stores/scraper-ws.svelte';
import type { DebugContext } from '../context';
import type { HostSummary, Phase } from '$lib/types/scraper';

type ScraperWS = typeof scraperWS;

export type RuntimeVm = {
	// Connection.
	wsConnected: boolean;
	lastError: string | null;

	// Lifecycle.
	phase: Phase;
	startedAt: string | undefined;
	runnerUptimeStr: string;
	lastReadStr: string;
	engineTick: number | undefined;
	iterations: number | undefined;

	// Cross-instance summary entry.
	hostSummary: HostSummary | null;

	// Envelope freshness.
	gameDataStr: string;
	tickStr: string;
	lastEventsReplyStr: string;
	lastEventsReplyPhase: Phase | undefined;
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

export function buildRuntimeVm(name: string, ws: ScraperWS, ctx: DebugContext): RuntimeVm {
	const snap = ws.snapshots[name] ?? null;
	const phase: Phase = ws.phases[name] ?? ctx.inspect?.phase ?? 'idle';
	const startedAt = snap?.started_at ?? ctx.inspect?.started_at;
	const startedAtMs = parseIsoMs(startedAt);
	const runnerUptimeStr = startedAtMs !== undefined ? formatUptime(ctx.now - startedAtMs) : '—';
	const lastReadAtMs = parseIsoMs(ws.lastReadAt[name] ?? ctx.inspect?.last_read_at);
	const lastReadStr = ctx.relativeTime(lastReadAtMs);
	const engineTick = snap?.engine_tick ?? ctx.inspect?.tick;
	const iterations = snap?.iterations ?? ctx.inspect?.ticks;

	const hostSummary: HostSummary | null = ws.hostSummaries[name] ?? null;
	const hostSummaryReadMs = parseIsoMs(hostSummary?.last_successful_read_at);
	const hostSummaryReadStr = ctx.relativeTime(hostSummaryReadMs);

	const gameDataStr = ctx.relativeTime(ws.gameDataAt[name]);
	const tickStr = ctx.relativeTime(ws.ticksAt[name]);
	const lastEventsReplyStr = ctx.relativeTime(ws.lastEventsReplyAt[name]);
	const lastEventsReplyPhase = ws.lastEventsReplyPhase[name];

	const connection: Record<string, unknown> = {
		ws_connected: ws.connected ? 'connected' : 'disconnected',
		last_error: ws.lastError ?? '—'
	};

	const lifecycle: Record<string, unknown> = {
		phase,
		started_at: startedAt ?? '—',
		runner_uptime: runnerUptimeStr,
		last_read_at: ws.lastReadAt[name] ?? '—',
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
