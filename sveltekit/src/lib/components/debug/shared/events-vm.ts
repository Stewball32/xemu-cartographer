// View-model helpers for rendering "event" envelopes — used by the Overview
// tab's RecentEvents / EventTile. Reconstructed after the original
// events/events-vm.ts was lost during a tab restructure; behavior is
// inferred from EventTile.svelte's consumer code and the backend event
// emitters under internal/scraper/haloce/events/.

import type { Envelope } from '$lib/types/scraper';

// Minimal player ref needed to render names + armor-tinted callouts.
// Compatible with GamePlayer (which has index/name/armor_color and more).
export interface EventRosterRef {
	index: number;
	name: string;
	armor_color: number;
}

export type EventBucket = 'combat' | 'pickup' | 'powerup' | 'match' | 'player' | 'other';

// Categorize an event into a coarse bucket for icon + status-color routing
// in EventTile. The mapping mirrors the implicit grouping the original VM
// used, drawn from the backend's event_type constants (internal/scraper/
// types.go and the haloce event emitters).
export function eventBucket(env: Envelope): EventBucket {
	const t = eventTypeOf(env);
	switch (t) {
		case 'kill':
		case 'death':
		case 'damage':
		case 'melee':
		case 'team_kill':
		case 'multikill':
		case 'kill_streak':
		case 'grenade_thrown':
			return 'combat';
		case 'item_picked_up':
		case 'item_dropped':
		case 'item_spawned':
		case 'item_depleted':
			return 'pickup';
		case 'powerup_picked_up':
		case 'powerup_expired':
			return 'powerup';
		case 'game_start':
		case 'game_end':
		case 'team_score':
			return 'match';
		case 'spawn':
		case 'score':
		case 'vehicle_entered':
		case 'vehicle_exited':
		case 'player_joined':
		case 'player_left':
		case 'player_quit':
		case 'player_team_changed':
			return 'player';
		default:
			return 'other';
	}
}

// Inner event type lives in payload.event_type; the outer envelope.type is
// the literal "event" wire-type (see internal/scraper/manager/events.go).
// Returns undefined when payload is missing event_type so callers can fall
// back to env.type.
export function eventTypeOf(env: Envelope): string | undefined {
	const p = (env.payload ?? {}) as Record<string, unknown>;
	const t = p.event_type;
	return typeof t === 'string' && t.length > 0 ? t : undefined;
}

export type SummaryPart =
	| { kind: 'text'; text: string }
	| { kind: 'player'; name: string; armorColor: number };

function findPlayer(players: EventRosterRef[], idx: unknown): EventRosterRef | undefined {
	if (typeof idx !== 'number' || idx < 0) return undefined;
	return players.find((p) => p.index === idx);
}

function playerPart(players: EventRosterRef[], idx: unknown): SummaryPart {
	const pl = findPlayer(players, idx);
	if (pl) return { kind: 'player', name: pl.name, armorColor: pl.armor_color };
	return { kind: 'text', text: typeof idx === 'number' ? `P${idx}` : '?' };
}

function txt(s: string): SummaryPart {
	return { kind: 'text', text: s };
}

// Render an event as an array of summary parts — text runs interleaved with
// player chips. EventTile renders player parts with armor-tinted color (FFA)
// or plain bold (team games).
export function summarizeEvent(env: Envelope, players: EventRosterRef[]): SummaryPart[] {
	const t = eventTypeOf(env) ?? env.type;
	const p = (env.payload ?? {}) as Record<string, unknown>;

	switch (t) {
		case 'kill':
			return [playerPart(players, p.killer), txt(' killed '), playerPart(players, p.victim)];
		case 'team_kill':
			return [playerPart(players, p.killer), txt(' betrayed '), playerPart(players, p.victim)];
		case 'death':
			return [playerPart(players, p.player), txt(' died')];
		case 'spawn':
			return [playerPart(players, p.player), txt(' spawned')];
		case 'damage':
			return [playerPart(players, p.attacker ?? p.player), txt(' damaged ')];
		case 'melee':
			return [playerPart(players, p.attacker ?? p.player), txt(' meleed')];
		case 'multikill':
			return [playerPart(players, p.player), txt(` ×${p.count ?? '?'} multikill`)];
		case 'kill_streak':
			return [playerPart(players, p.player), txt(` kill streak ×${p.count ?? '?'}`)];
		case 'score':
			return [playerPart(players, p.player), txt(' scored')];
		case 'grenade_thrown':
			return [
				playerPart(players, p.player),
				txt(` threw ${typeof p.kind === 'string' ? p.kind : ''} grenade`)
			];
		case 'item_picked_up':
			return [
				playerPart(players, p.player),
				txt(` picked up ${typeof p.tag === 'string' ? p.tag : 'item'}`)
			];
		case 'item_dropped':
			return [
				playerPart(players, p.player),
				txt(` dropped ${typeof p.tag === 'string' ? p.tag : 'item'}`)
			];
		case 'item_spawned':
			return [txt(`${typeof p.tag === 'string' ? p.tag : 'item'} spawned`)];
		case 'item_depleted':
			return [txt(`${typeof p.tag === 'string' ? p.tag : 'item'} depleted`)];
		case 'powerup_picked_up':
			return [
				playerPart(players, p.player),
				txt(` grabbed ${typeof p.tag === 'string' ? p.tag : (p.kind ?? 'powerup')}`)
			];
		case 'powerup_expired':
			return [txt(`${typeof p.tag === 'string' ? p.tag : (p.kind ?? 'powerup')} expired`)];
		case 'vehicle_entered':
			return [playerPart(players, p.player), txt(' entered vehicle')];
		case 'vehicle_exited':
			return [playerPart(players, p.player), txt(' exited vehicle')];
		case 'player_joined':
			return [playerPart(players, p.player), txt(' joined')];
		case 'player_left':
			return [playerPart(players, p.player), txt(' left')];
		case 'player_quit':
			return [playerPart(players, p.player), txt(' quit')];
		case 'player_team_changed':
			return [
				playerPart(players, p.player),
				txt(` → team ${typeof p.team === 'number' ? p.team : '?'}`)
			];
		case 'team_score':
			return [
				txt(`team ${typeof p.team === 'number' ? p.team : '?'} → ${p.score ?? '?'}`)
			];
		case 'game_start':
			return [txt(`Match started${typeof p.gametype === 'string' ? ` (${p.gametype})` : ''}`)];
		case 'game_end':
			return [txt(`Match ended${typeof p.gametype === 'string' ? ` (${p.gametype})` : ''}`)];
		default:
			return [txt(t)];
	}
}
