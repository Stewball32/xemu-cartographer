import { describe, it, expect } from 'vitest';
import { killedByMap, withKilledBy } from './overlay-deaths';
import type { OverlayPlayer } from './overlay-split';
import type { AnyEvent } from '$lib/types/scraper-v2';

/** Feed order is newest-first, matching the WS store's event log. */
function death(victim: string, killer: string | null, tick: number): AnyEvent {
	return {
		seq: 0,
		tick,
		at: '',
		event_type: 'death',
		victim: { index: 0, name: victim, team: 0, armor_color: 0 },
		killer: killer === null ? null : { index: 1, name: killer, team: 1, armor_color: 0 },
		cause: killer === null ? 'suicide' : 'kill',
		weapon: '',
		team_kill: false,
		respawn_in_ticks: 90
	} as AnyEvent;
}

function player(name: string): OverlayPlayer {
	return { name, display: name } as OverlayPlayer;
}

describe('killedByMap', () => {
	it('newest death wins per victim', () => {
		const map = killedByMap([
			death('gravemind', 'Stewball', 200),
			death('gravemind', 'Keyes', 100)
		]);
		expect(map.gravemind).toBe('Stewball');
	});

	it('a later unattributed death clears an earlier killer', () => {
		// Without this the plate would keep naming Keyes through a suicide the
		// player took seconds later.
		const map = killedByMap([death('gravemind', null, 200), death('gravemind', 'Keyes', 100)]);
		expect(map.gravemind).toBeNull();
	});

	it('maps a killerless death to null, not to an empty name', () => {
		expect(killedByMap([death('gravemind', null, 10)]).gravemind).toBeNull();
	});

	it('trims both sides — CE space-pads profile names', () => {
		const map = killedByMap([death('  gravemind ', ' Stewball  ', 10)]);
		expect(map.gravemind).toBe('Stewball');
	});

	it('ignores non-death events', () => {
		const medal = {
			seq: 0,
			tick: 5,
			at: '',
			event_type: 'medal',
			kind: 'multikill',
			player: { index: 0, name: 'gravemind', team: 0, armor_color: 0 },
			count: 2
		} as AnyEvent;
		expect(killedByMap([medal])).toEqual({});
	});

	it('is empty for an absent feed', () => {
		expect(killedByMap(null)).toEqual({});
		expect(killedByMap([])).toEqual({});
	});
});

describe('withKilledBy', () => {
	it('attaches the killer to the matching player', () => {
		const [a, b] = withKilledBy(
			[player('gravemind'), player('Stewball')],
			[death('gravemind', 'Stewball', 10)]
		);
		expect(a.killedBy).toBe('Stewball');
		// The killer themselves didn't die — null, so their ring never shows a
		// plate from someone else's death.
		expect(b.killedBy).toBeNull();
	});

	it('is null (not undefined) for players with no death on record', () => {
		expect(withKilledBy([player('gravemind')], [])[0].killedBy).toBeNull();
	});

	it('does not mutate the input players', () => {
		const input = [player('gravemind')];
		withKilledBy(input, [death('gravemind', 'Stewball', 10)]);
		expect(input[0].killedBy).toBeUndefined();
	});
});
