// View-model helpers for rendering "event" envelopes — used by the Events
// tab + the Overview tab's RecentEvents / EventTile. Updated for v2 where
// the 24 v1 event types collapsed into 5 typed payloads
// (death/damage/medal/player_update/game_update) each carrying a
// kind/cause discriminator on the inner payload. See
// atlas/new_json/04-ground-up-rebuild.md §6 (event stream) and
// internal/scraper/event_payloads.go for the kind catalogs.

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
// in EventTile. Dispatches on v2 event_type first, then on the inner
// kind/cause discriminator where the event_type itself is too coarse to
// pick a bucket (player_update spans player + pickup + powerup buckets;
// game_update spans match + player buckets).
export function eventBucket(env: Envelope): EventBucket {
	const t = eventTypeOf(env);
	const p = (env.payload ?? {}) as Record<string, unknown>;
	const kind = typeof p.kind === 'string' ? p.kind : '';
	switch (t) {
		case 'death':
		case 'damage':
		case 'medal':
			return 'combat';
		case 'player_update':
			switch (kind) {
				case 'powerup_picked_up':
				case 'powerup_expired':
					return 'powerup';
				case 'item_picked_up':
				case 'item_dropped':
				case 'item_depleted':
					return 'pickup';
				default:
					return 'player';
			}
		case 'game_update':
			switch (kind) {
				case 'team_score':
				case 'game_start':
				case 'game_end':
					return 'match';
				case 'item_spawned':
					return 'pickup';
				default:
					return 'player';
			}
		default:
			return 'other';
	}
}

// Inner event type lives in payload.event_type; the outer envelope.type is
// always the literal "event" wire-type for live broadcasts and "events"
// for backfill batches (see internal/scraper/manager/events.go).
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

// In v2, every PlayerRef carries its own identity (name + armor_color)
// inline so the event log is analytics-complete without a roster join.
// We still try the roster first so the rendered name matches whatever the
// game tab is showing (cross-event consistency in mid-game renames);
// fall through to the ref's denormalised fields when the roster doesn't
// have the index.
type PlayerLike = { index?: number; name?: string; armor_color?: number };

function playerPartFromRef(players: EventRosterRef[], ref: unknown): SummaryPart {
	if (ref && typeof ref === 'object' && 'index' in ref) {
		const r = ref as PlayerLike;
		const pl = findPlayer(players, r.index);
		if (pl) return { kind: 'player', name: pl.name, armorColor: pl.armor_color };
		const name = r.name && r.name.length > 0 ? r.name : `P${r.index}`;
		return { kind: 'player', name, armorColor: r.armor_color ?? 0 };
	}
	return { kind: 'text', text: '?' };
}

function txt(s: string): SummaryPart {
	return { kind: 'text', text: s };
}

// Compose an event summary for the Events tab feed + Overview RecentEvents.
// Dispatches on event_type → kind/cause for v2 events.
export function summarizeEvent(env: Envelope, players: EventRosterRef[]): SummaryPart[] {
	const t = eventTypeOf(env) ?? env.type;
	const p = (env.payload ?? {}) as Record<string, unknown>;
	const kind = typeof p.kind === 'string' ? p.kind : '';
	const cause = typeof p.cause === 'string' ? p.cause : '';

	switch (t) {
		case 'death': {
			// Cause discriminates: kill / betrayal carry a killer; suicide /
			// fall / environment do not. See DeathEvent in
			// internal/scraper/event_payloads.go.
			if (cause === 'betrayal') {
				return [
					playerPartFromRef(players, p.killer),
					txt(' betrayed '),
					playerPartFromRef(players, p.victim)
				];
			}
			if (cause === 'kill' && p.killer) {
				return [
					playerPartFromRef(players, p.killer),
					txt(' killed '),
					playerPartFromRef(players, p.victim)
				];
			}
			if (cause === 'suicide') return [playerPartFromRef(players, p.victim), txt(' suicided')];
			if (cause === 'fall') return [playerPartFromRef(players, p.victim), txt(' fell to death')];
			if (cause === 'environment')
				return [playerPartFromRef(players, p.victim), txt(' died (env)')];
			return [playerPartFromRef(players, p.victim), txt(' died')];
		}
		case 'damage': {
			if (kind === 'melee') {
				return [
					playerPartFromRef(players, p.dealer),
					txt(' meleed '),
					playerPartFromRef(players, p.victim)
				];
			}
			return [
				playerPartFromRef(players, p.dealer),
				txt(' damaged '),
				playerPartFromRef(players, p.victim)
			];
		}
		case 'medal': {
			const count = typeof p.count === 'number' ? `×${p.count}` : '';
			if (kind === 'multikill') {
				return [playerPartFromRef(players, p.player), txt(` ${count} multikill`)];
			}
			if (kind === 'kill_streak') {
				return [playerPartFromRef(players, p.player), txt(` kill streak ${count}`)];
			}
			return [playerPartFromRef(players, p.player), txt(` ${kind} ${count}`)];
		}
		case 'player_update': {
			switch (kind) {
				case 'spawn':
					return [playerPartFromRef(players, p.player), txt(' spawned')];
				case 'score':
					return [playerPartFromRef(players, p.player), txt(' scored')];
				case 'powerup_picked_up':
					return [
						playerPartFromRef(players, p.player),
						txt(` grabbed ${typeof p.powerup === 'string' ? p.powerup : 'powerup'}`)
					];
				case 'powerup_expired':
					return [
						playerPartFromRef(players, p.player),
						txt(` lost ${typeof p.powerup === 'string' ? p.powerup : 'powerup'}`)
					];
				case 'vehicle_entered':
					return [
						playerPartFromRef(players, p.player),
						txt(` entered vehicle${typeof p.seat === 'string' ? ` (${p.seat})` : ''}`)
					];
				case 'vehicle_exited':
					return [playerPartFromRef(players, p.player), txt(' exited vehicle')];
				case 'item_picked_up':
				case 'item_dropped':
				case 'item_depleted': {
					const verb =
						kind === 'item_picked_up'
							? 'picked up'
							: kind === 'item_dropped'
								? 'dropped'
								: 'depleted';
					const item = p.item as { tag?: string } | undefined;
					return [playerPartFromRef(players, p.player), txt(` ${verb} ${item?.tag ?? 'item'}`)];
				}
				case 'grenade_thrown':
					return [
						playerPartFromRef(players, p.player),
						txt(
							` threw ${typeof p.grenade_type === 'string' ? p.grenade_type : ''} grenade`.replace(
								/\s+/g,
								' '
							)
						)
					];
				case 'player_quit':
					return [playerPartFromRef(players, p.player), txt(' quit')];
				default:
					return [playerPartFromRef(players, p.player), txt(` ${kind}`)];
			}
		}
		case 'game_update': {
			switch (kind) {
				case 'player_joined':
					return [playerPartFromRef(players, p.player), txt(' joined')];
				case 'player_left':
					return [playerPartFromRef(players, p.player), txt(' left')];
				case 'player_team_changed':
					return [
						playerPartFromRef(players, p.player),
						txt(` → team ${typeof p.team === 'number' ? p.team : '?'}`)
					];
				case 'team_score':
					return [txt(`team ${typeof p.team === 'number' ? p.team : '?'} → ${p.score ?? '?'}`)];
				case 'game_start':
					return [txt(`Game started${typeof p.gametype === 'string' ? ` (${p.gametype})` : ''}`)];
				case 'game_end':
					return [txt(`Game ended${typeof p.gametype === 'string' ? ` (${p.gametype})` : ''}`)];
				case 'item_spawned': {
					const item = p.item as { tag?: string } | undefined;
					return [txt(`${item?.tag ?? 'item'} spawned`)];
				}
				default:
					return [txt(`game: ${kind}`)];
			}
		}
		default:
			return [txt(t)];
	}
}
