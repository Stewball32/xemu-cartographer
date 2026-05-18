// Pretty-view VM for the Previous-Game debug tab — formats the
// PreviousGamePayload into three section records mirroring the envelope's
// JSON shape (top-level scalars + game summary + events table). Every
// expected field is always present; payloads missing a field render as
// '—' so the Pretty view exposes the envelope structure even before the
// first read.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type {
	AnyEvent,
	EnvelopeV2,
	GamePayload,
	GameTeamScore,
	PreviousGamePayload
} from '$lib/types/scraper-v2';

// One tile's display state + the raw value (passed through StatTile's
// `title` prop so hovering a formatted tile reveals the underlying
// wire value). Mirrors the FieldDisplay contract in xbox-vm.ts.
export interface FieldDisplay {
	display: string;
	raw: string;
	present: boolean;
}

// One row in the events table — every column derived from a full v2
// envelope so the operator can correlate UI rows to wire entries.
export interface EventRow {
	seq: number;
	tick: number;
	ts: string;
	type: string;
	event_type: string;
}

export interface PreviousGamePrettyVm {
	topLevel: {
		ended_at: FieldDisplay;
	};
	game: {
		present: boolean;
		phase: FieldDisplay;
		started_at: FieldDisplay;
		last_read_at: FieldDisplay;
		engine_tick: FieldDisplay;
		iterations: FieldDisplay;
		gametype: FieldDisplay;
		variant_name: FieldDisplay;
		is_team_game: FieldDisplay;
		score_limit: FieldDisplay;
		time_limit_ticks: FieldDisplay;
		players_count: FieldDisplay;
		machines_count: FieldDisplay;
		team_scores_summary: FieldDisplay;
	};
	events: {
		count: number;
		rows: EventRow[];
	};
}

function field(display: string, raw: string | undefined, present: boolean): FieldDisplay {
	return { display, raw: raw ?? '—', present };
}

function fieldStr(value: string | undefined | null): FieldDisplay {
	const v = value ?? '';
	return field(v || '—', v || undefined, v.length > 0);
}

function fieldNum(value: number | undefined | null): FieldDisplay {
	if (value === undefined || value === null) return field('—', undefined, false);
	return field(String(value), String(value), true);
}

function fieldBool(value: boolean | undefined | null): FieldDisplay {
	if (value === undefined || value === null) return field('—', undefined, false);
	return field(value ? 'true' : 'false', String(value), true);
}

function formatIsoLocal(iso: string | undefined | null): string {
	if (!iso) return '—';
	const t = Date.parse(iso);
	if (!Number.isFinite(t)) return iso;
	return `${new Date(t).toLocaleString()} (${iso})`;
}

function fieldIso(value: string | undefined | null): FieldDisplay {
	const v = value ?? '';
	return field(v ? formatIsoLocal(v) : '—', v || undefined, v.length > 0);
}

function summariseTeamScores(scores: GameTeamScore[] | undefined | null): FieldDisplay {
	if (!scores || scores.length === 0) {
		return field('—', undefined, false);
	}
	const sorted = [...scores].sort((a, b) => a.team - b.team);
	const display = sorted.map((s) => `${s.team}:${s.score}`).join(' / ');
	return field(display, JSON.stringify(scores), true);
}

function buildGameSection(game: GamePayload | null): PreviousGamePrettyVm['game'] {
	const cfg = game?.config ?? null;
	return {
		present: game !== null,
		phase: fieldStr(game?.phase),
		started_at: fieldIso(game?.started_at),
		last_read_at: fieldIso(game?.last_read_at),
		engine_tick: fieldNum(game?.engine_tick),
		iterations: fieldNum(game?.iterations),
		gametype: fieldStr(cfg?.gametype),
		variant_name: fieldStr(cfg?.variant_name),
		is_team_game: fieldBool(cfg?.is_team_game),
		score_limit: fieldNum(cfg?.score_limit),
		time_limit_ticks: fieldNum(cfg?.time_limit_ticks),
		players_count: fieldNum(game?.players?.length),
		machines_count: fieldNum(game?.machines?.length),
		team_scores_summary: summariseTeamScores(game?.team_scores)
	};
}

function buildEventRow(env: EnvelopeV2): EventRow {
	const inner = env.data as AnyEvent | undefined;
	return {
		seq: env.seq,
		tick: env.tick,
		ts: env.ts,
		type: env.type,
		event_type: inner?.event_type ?? '—'
	};
}

export function buildPreviousGamePrettyVm(
	payload: PreviousGamePayload | null
): PreviousGamePrettyVm {
	const events = payload?.events ?? [];
	return {
		topLevel: {
			ended_at: fieldIso(payload?.ended_at)
		},
		game: buildGameSection(payload?.game ?? null),
		events: {
			count: events.length,
			rows: events.map(buildEventRow)
		}
	};
}
