// Pure per-player render logic shared by the broadcast cards + scoreboard.
//
// Two rules live here:
//   1. TEAM-vs-FFA armor colour (accurate to the game). In a TEAM game every
//      player on a team shares ONE team armor colour — colour does NOT tell
//      teammates apart (the emblem + gamertag do). In FFA colours are per-player
//      and distinct, so colour IS a valid identifier. `resolveArmorIndex`
//      branches on that: team colour in team games, the roster's per-player
//      colour in FFA.
//   2. Profile-avatar merge. A player's identity avatar is their gamertag
//      PROFILE (Spartan + H2 emblem). `cardAppearance` puts the profile's emblem
//      on a game-accurate armor colour (so a team game still reads team-coloured)
//      and returns undefined when there's no profile (→ caller renders a plain
//      tinted Spartan with no emblem, never a generic placeholder emblem).
//
// Pure (no IO, no Svelte) → unit-tested.

import { H2_KEYS, type Appearance } from '$lib/utils/emblem';
import type { BroadcastGame } from './theme';

// Team index → armor-palette index, per game — the colour the engine forces on a
// team. Ordered Red, Blue, Green, Gold, Purple, Orange, Cyan, Pink to match
// overlay-view's TEAM_META; indices beyond the table wrap (defensive).
const CE_TEAM_ARMOR = [2, 3, 6, 5, 8, 11, 9, 7]; // CE_COLORS indices
const H2_TEAM_ARMOR = [2, 11, 6, 4, 13, 3, 8, 14]; // H2_COLORS indices

/** Armor-palette index for a team, per game (accurate shared team colour). */
export function teamArmorIndex(game: BroadcastGame, team: number): number {
	const arr = game === 'ce' ? CE_TEAM_ARMOR : H2_TEAM_ARMOR;
	const len = arr.length;
	return arr[((Math.trunc(team) % len) + len) % len];
}

/** The armor index to tint a player's Spartan: shared team colour in team games,
 * the player's own roster colour in FFA. */
export function resolveArmorIndex(
	game: BroadcastGame,
	isTeamGame: boolean,
	team: number,
	playerArmorColor: number
): number {
	return isTeamGame ? teamArmorIndex(game, team) : playerArmorColor;
}

/** A player's resolved profile appearance (from the public profiles endpoint /
 * the mock). Any field may be absent. */
export interface ResolvedProfile {
	/** The user's PocketBase avatar image (built-in users.avatar upload) as a
	 * ready-to-fetch URL — the cards' identity-avatar spot pulls this straight
	 * from PB instead of rendering anything client-side. */
	avatar?: string;
	ce?: { color: number };
	h2?: { appearance: Appearance };
}

/** True when a profile carries anything renderable. */
export function hasProfileData(p: ResolvedProfile | null | undefined): boolean {
	return !!(p && (p.h2?.appearance || p.ce));
}

/** The H2 appearance to render on a card: the profile's emblem re-coloured to the
 * game-accurate armor index (`colorIndex`), so a team game reads team-coloured
 * while the emblem still identifies the player. Returns undefined for CE (no
 * emblem system) or when there's no H2 profile (→ render a plain tinted Spartan,
 * no emblem). */
export function cardAppearance(
	game: BroadcastGame,
	colorIndex: number,
	profile: ResolvedProfile | null | undefined
): Appearance | undefined {
	if (game !== 'h2') return undefined;
	const base = profile?.h2?.appearance;
	if (!base) return undefined;
	return {
		...base,
		[H2_KEYS.armorPrimary]: colorIndex,
		[H2_KEYS.armorSecondary]: colorIndex
	};
}
