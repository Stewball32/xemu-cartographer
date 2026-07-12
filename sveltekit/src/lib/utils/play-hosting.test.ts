import { describe, it, expect } from 'vitest';
import {
	activeInstance,
	derivePlayHostPhase,
	formatCountdown,
	isPlayerControllable,
	resolvedFromCurrent,
	INITIAL_INPUTS,
	type PlayHostInputs,
	type PlayStatus
} from './play-hosting';

function inputs(over: Partial<PlayHostInputs>): PlayHostInputs {
	return { ...INITIAL_INPUTS, ...over };
}

describe('derivePlayHostPhase', () => {
	it('is catalog when there is no box at all', () => {
		expect(derivePlayHostPhase(inputs({}))).toBe('catalog');
	});

	it('is provisioning after a request while the server has not resolved the box', () => {
		expect(derivePlayHostPhase(inputs({ pendingInstance: 'play-abc' }))).toBe('provisioning');
	});

	it('is lobby once resolved and the game feed is idle/ready', () => {
		expect(derivePlayHostPhase(inputs({ resolvedInstance: 'play-abc', gamePhase: 'idle' }))).toBe(
			'lobby'
		);
		expect(derivePlayHostPhase(inputs({ resolvedInstance: 'play-abc', gamePhase: 'ready' }))).toBe(
			'lobby'
		);
	});

	it('is lobby when resolved but no game feed yet (null phase)', () => {
		expect(derivePlayHostPhase(inputs({ resolvedInstance: 'play-abc', gamePhase: null }))).toBe(
			'lobby'
		);
	});

	it('is live when the game feed reports a live match', () => {
		expect(derivePlayHostPhase(inputs({ resolvedInstance: 'play-abc', gamePhase: 'live' }))).toBe(
			'live'
		);
	});

	it('prefers live over a pending previous game', () => {
		expect(
			derivePlayHostPhase(
				inputs({ resolvedInstance: 'play-abc', gamePhase: 'live', hasPreviousGame: true })
			)
		).toBe('live');
	});

	it('is postgame when a match just ended and not dismissed', () => {
		expect(
			derivePlayHostPhase(
				inputs({ resolvedInstance: 'play-abc', gamePhase: 'ready', hasPreviousGame: true })
			)
		).toBe('postgame');
	});

	it('returns to lobby once the post-game card is dismissed', () => {
		expect(
			derivePlayHostPhase(
				inputs({
					resolvedInstance: 'play-abc',
					gamePhase: 'ready',
					hasPreviousGame: true,
					postgameDismissed: true
				})
			)
		).toBe('lobby');
	});

	it('resolved instance wins over a stale pending instance (no longer provisioning)', () => {
		expect(
			derivePlayHostPhase(
				inputs({ resolvedInstance: 'play-abc', pendingInstance: 'play-abc', gamePhase: 'idle' })
			)
		).toBe('lobby');
	});
});

describe('activeInstance', () => {
	it('prefers the resolved instance', () => {
		expect(activeInstance({ resolvedInstance: 'a', pendingInstance: 'b' })).toBe('a');
	});
	it('falls back to the pending instance', () => {
		expect(activeInstance({ resolvedInstance: null, pendingInstance: 'b' })).toBe('b');
	});
	it('is null with neither', () => {
		expect(activeInstance({ resolvedInstance: null, pendingInstance: null })).toBeNull();
	});
});

describe('resolvedFromCurrent', () => {
	it('maps an empty instance to null', () => {
		expect(resolvedFromCurrent({ instance: '', status: null })).toBeNull();
	});
	it('maps a named instance through', () => {
		expect(resolvedFromCurrent({ instance: 'play-x', status: null })).toBe('play-x');
	});
	it('is null for a nullish response', () => {
		expect(resolvedFromCurrent(null)).toBeNull();
		expect(resolvedFromCurrent(undefined)).toBeNull();
	});
});

describe('formatCountdown', () => {
	it('formats sub-minute as seconds', () => {
		expect(formatCountdown(0)).toBe('0s');
		expect(formatCountdown(45)).toBe('45s');
	});
	it('formats whole minutes', () => {
		expect(formatCountdown(120)).toBe('2m');
	});
	it('formats minutes + seconds', () => {
		expect(formatCountdown(150)).toBe('2m 30s');
	});
	it('clamps negatives / non-finite to 0s', () => {
		expect(formatCountdown(-10)).toBe('0s');
		expect(formatCountdown(NaN)).toBe('0s');
	});
});

describe('isPlayerControllable', () => {
	const base: PlayStatus = {
		instance: 'play-x',
		present: true,
		authority: 'runner',
		tick: 0,
		machine_count: 1,
		team_count: 0,
		countdown_active: false,
		ready_to_start: false,
		selected: false,
		ready: false
	};
	it('true when present + runner authority', () => {
		expect(isPlayerControllable(base)).toBe(true);
	});
	it('false when an admin took over', () => {
		expect(isPlayerControllable({ ...base, authority: 'admin' })).toBe(false);
	});
	it('false when hosting disabled', () => {
		expect(isPlayerControllable({ ...base, authority: 'disabled' })).toBe(false);
	});
	it('false when no runner is present', () => {
		expect(isPlayerControllable({ ...base, present: false })).toBe(false);
	});
	it('false for null', () => {
		expect(isPlayerControllable(null)).toBe(false);
	});
});
