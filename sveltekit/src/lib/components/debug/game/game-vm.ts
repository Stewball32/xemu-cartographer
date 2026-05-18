// Pretty-view VM for the Game debug tab — formats the GamePayload into
// section records that mirror the envelope's JSON shape (top-level scalars
// + config + team_scores + players + machines + network). Every field is
// always present; payloads missing a field render as '—' so the Pretty
// view exposes the expected envelope structure even before the first
// read.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type { GamePayload } from '$lib/types/scraper-v2';

// One tile's display state + the raw value (passed through StatTile's
// `title` prop so hovering a formatted tile reveals the underlying
// wire value).
export interface FieldDisplay {
	display: string;
	raw: string;
	present: boolean;
}

export interface GamePrettyVm {
	topLevel: {
		phase: FieldDisplay;
		started_at: FieldDisplay;
		last_read_at: FieldDisplay;
		engine_tick: FieldDisplay;
		iterations: FieldDisplay;
	};
	config: {
		gametype: FieldDisplay;
		variant_name: FieldDisplay;
		is_team_game: FieldDisplay;
		score_limit: FieldDisplay;
		time_limit_ticks: FieldDisplay;
	};
	teamScores: GameTeamScoreRow[];
	players: GamePlayerRow[];
	machines: GameMachineRow[];
	network: {
		'countdown.active': FieldDisplay;
		'countdown.paused': FieldDisplay;
		'countdown.seconds_to_start': FieldDisplay;
		'client.machine_index': FieldDisplay;
		'client.average_ping': FieldDisplay;
		'client.packets_sent': FieldDisplay;
	};
}

// Row shapes for the DataTable sections. Keep them flat strings/booleans
// so the table's default formatter handles missing values uniformly.
export interface GameTeamScoreRow {
	team: number;
	score: number;
}

export interface GamePlayerRow {
	index: number;
	name: string;
	team: number;
	armor_color: number;
	score: number;
	kills: number;
	deaths: number;
	assists: number;
	ctf_score: number;
	team_kills: number;
	suicides: number;
	kill_streak: number;
	multikill: number;
	shots_fired: number;
	shots_hit: number;
	is_local: boolean | null;
	local_index: number | null;
	machine_index: number | null;
	controller_index: number | null;
}

export interface GameMachineRow {
	index: number;
	name: string;
	is_local: boolean | null;
}

function field(display: string, raw: string | undefined, present: boolean): FieldDisplay {
	return { display, raw: raw ?? '—', present };
}

function fieldStr(value: string | undefined): FieldDisplay {
	const v = value ?? '';
	return field(v || '—', v || undefined, v.length > 0);
}

function fieldBool(value: boolean | undefined | null): FieldDisplay {
	if (value === undefined || value === null) return field('—', undefined, false);
	return field(value ? 'true' : 'false', String(value), true);
}

function fieldInt(value: number | undefined | null): FieldDisplay {
	if (value === undefined || value === null) return field('—', undefined, false);
	const s = String(value);
	return field(s, s, true);
}

function formatIsoLocal(iso: string | undefined): FieldDisplay {
	if (!iso) return field('—', undefined, false);
	const t = Date.parse(iso);
	if (!Number.isFinite(t)) return field(iso, iso, true);
	return field(`${new Date(t).toLocaleString()} (${iso})`, iso, true);
}

export function buildGamePrettyVm(payload: GamePayload | null): GamePrettyVm {
	const cfg = payload?.config ?? null;
	const net = payload?.network ?? null;
	const countdown = net?.countdown ?? null;
	const client = net?.client ?? null;

	return {
		topLevel: {
			phase: fieldStr(payload?.phase),
			started_at: formatIsoLocal(payload?.started_at),
			last_read_at: formatIsoLocal(payload?.last_read_at),
			engine_tick: fieldInt(payload?.engine_tick),
			iterations: fieldInt(payload?.iterations)
		},
		config: {
			gametype: fieldStr(cfg?.gametype),
			variant_name: fieldStr(cfg?.variant_name),
			is_team_game: fieldBool(cfg?.is_team_game),
			score_limit: fieldInt(cfg?.score_limit),
			time_limit_ticks: fieldInt(cfg?.time_limit_ticks)
		},
		teamScores: payload?.team_scores ?? [],
		players: payload?.players ?? [],
		machines: payload?.machines ?? [],
		network: {
			'countdown.active': fieldBool(countdown?.active),
			'countdown.paused': fieldBool(countdown?.paused),
			'countdown.seconds_to_start': fieldInt(countdown?.seconds_to_start),
			'client.machine_index': fieldInt(client?.machine_index),
			'client.average_ping': fieldInt(client?.average_ping),
			'client.packets_sent': fieldInt(client?.packets_sent)
		}
	};
}
