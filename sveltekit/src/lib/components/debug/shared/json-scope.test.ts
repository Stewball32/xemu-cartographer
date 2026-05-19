import { describe, it, expect } from 'vitest';
import {
	buildTreeRoot,
	scopeAtPath,
	V2_ENVELOPE_SHAPE,
	EVENT_SHAPE,
	type XNode
} from './json-scope';

// Tiny fixtures mirroring the actual envelope shape just enough to exercise
// scope/tree behavior. Kept inline so the test file is self-contained.
const v2Env = {
	v: 2,
	type: 'tick',
	instance: 'pod-a',
	seq: 17,
	tick: 12345,
	ts: '2026-05-18T00:00:00Z',
	data: {
		game_globals: { active: 1, time_limit: 600 },
		players: [
			{ index: 0, name: 'Shadow', health: 100, pos: { x: 1, y: 2, z: 3 } },
			{ index: 1, name: 'Whisp', health: 75, pos: { x: 4, y: 5, z: 6 } },
			{ index: 2, name: 'Disco', pos: { x: 7, y: 8, z: 9 } } // missing health
		],
		tags: ['ctf', 'team_slayer'],
		empty: []
	}
};

const v2EnvHetero = {
	v: 2,
	type: 'previous_game',
	instance: 'pod-a',
	seq: 1,
	tick: 0,
	ts: '2026-05-18T00:00:00Z',
	data: {
		events: [
			{ event_type: 'death', victim: { name: 'A' }, killer: { name: 'B' } },
			{ event_type: 'damage', dealer: { name: 'C' }, amount: 25 },
			{ event_type: 'medal', kind: 'killtacular' }
		]
	}
};

const eventEnv = {
	seq: 99,
	tick: 4242,
	at: '2026-05-18T00:00:01Z',
	event_type: 'death',
	victim: { name: 'A', team: 0 },
	killer: { name: 'B' }
};

function names(children: XNode[] | undefined): string[] {
	return (children ?? []).map((c) => c.name);
}

function findChild(root: XNode, name: string): XNode | undefined {
	return root.children?.find((c) => c.name === name || c.name === `${name} [arr]`);
}

describe('buildTreeRoot — V2 shape', () => {
	it('lists data children directly at root (no envelope/data parents)', () => {
		const root = buildTreeRoot(v2Env, V2_ENVELOPE_SHAPE);
		expect(names(root.children)).toEqual(['game_globals', 'players [arr]', 'tags', 'empty']);
	});

	it('recurses into nested objects with dot-path ids', () => {
		const root = buildTreeRoot(v2Env, V2_ENVELOPE_SHAPE);
		const globals = findChild(root, 'game_globals');
		expect(globals?.children?.map((c) => c.id)).toEqual([
			'data.game_globals.active',
			'data.game_globals.time_limit'
		]);
	});

	it('emits [arr] branch for array-of-objects with union-of-keys children', () => {
		const root = buildTreeRoot(v2Env, V2_ENVELOPE_SHAPE);
		const players = findChild(root, 'players');
		expect(players?.name).toBe('players [arr]');
		// Union: index, name, health, pos — even though one player lacks health.
		const ids = players?.children?.map((c) => c.id);
		expect(ids).toEqual([
			'data.players[].index',
			'data.players[].name',
			'data.players[].health',
			'data.players[].pos'
		]);
	});

	it('treats primitive arrays and empty arrays as leaves', () => {
		const root = buildTreeRoot(v2Env, V2_ENVELOPE_SHAPE);
		expect(findChild(root, 'tags')?.children).toBeUndefined();
		expect(findChild(root, 'empty')?.children).toBeUndefined();
	});

	it('descends into nested objects under array elements', () => {
		const root = buildTreeRoot(v2Env, V2_ENVELOPE_SHAPE);
		const pos = findChild(findChild(root, 'players')!, 'pos');
		expect(pos?.children?.map((c) => c.id)).toEqual([
			'data.players[].pos.x',
			'data.players[].pos.y',
			'data.players[].pos.z'
		]);
	});

	it('surfaces every key from heterogeneous arrays (events log)', () => {
		const root = buildTreeRoot(v2EnvHetero, V2_ENVELOPE_SHAPE);
		const events = findChild(root, 'events');
		expect(events?.name).toBe('events [arr]');
		const ids = events?.children?.map((c) => c.id).sort();
		expect(ids).toEqual(
			[
				'data.events[].amount',
				'data.events[].dealer',
				'data.events[].event_type',
				'data.events[].killer',
				'data.events[].kind',
				'data.events[].victim'
			].sort()
		);
	});

	it('returns an empty tree when env is null', () => {
		const root = buildTreeRoot(null, V2_ENVELOPE_SHAPE);
		expect(root.children).toEqual([]);
	});
});

describe('buildTreeRoot — event shape', () => {
	it('places non-scalar event fields directly under the root', () => {
		const root = buildTreeRoot(eventEnv, EVENT_SHAPE);
		// Scalar keys (seq/tick/at/event_type) are excluded — they're always
		// emitted in the output and aren't selectable.
		expect(names(root.children)).toEqual(['victim', 'killer']);
	});

	it('recurses into nested ref objects (victim.name etc.)', () => {
		const root = buildTreeRoot(eventEnv, EVENT_SHAPE);
		const victim = findChild(root, 'victim');
		expect(victim?.children?.map((c) => c.id)).toEqual(['victim.name', 'victim.team']);
	});
});

describe('scopeAtPath — V2 shape', () => {
	it('returns the full envelope when nothing is selected', () => {
		expect(scopeAtPath(v2Env, [], V2_ENVELOPE_SHAPE)).toBe(v2Env);
	});

	it('prefixes the output with envelope scalars for a single payload path', () => {
		const out = scopeAtPath(v2Env, ['data.game_globals.active'], V2_ENVELOPE_SHAPE) as Record<
			string,
			unknown
		>;
		expect(Object.keys(out)).toEqual(['v', 'type', 'instance', 'seq', 'tick', 'ts', 'data']);
		expect(out.data).toEqual({ game_globals: { active: 1 } });
	});

	it('deep-merges multiple disjoint paths under data', () => {
		const out = scopeAtPath(
			v2Env,
			['data.game_globals.active', 'data.game_globals.time_limit'],
			V2_ENVELOPE_SHAPE
		) as Record<string, unknown>;
		expect(out.data).toEqual({ game_globals: { active: 1, time_limit: 600 } });
	});

	it('projects a scalar field across every element of an array', () => {
		const out = scopeAtPath(v2Env, ['data.players[].name'], V2_ENVELOPE_SHAPE) as Record<
			string,
			unknown
		>;
		expect(out.data).toEqual({
			players: [{ name: 'Shadow' }, { name: 'Whisp' }, { name: 'Disco' }]
		});
	});

	it('merges multiple projected fields per index', () => {
		const out = scopeAtPath(
			v2Env,
			['data.players[].name', 'data.players[].health'],
			V2_ENVELOPE_SHAPE
		) as Record<string, unknown>;
		expect(out.data).toEqual({
			players: [
				{ name: 'Shadow', health: 100 },
				{ name: 'Whisp', health: 75 },
				{ name: 'Disco', health: null } // missing field emits null
			]
		});
	});

	it('projects nested object fields at any depth (players[].pos.x)', () => {
		const out = scopeAtPath(v2Env, ['data.players[].pos.x'], V2_ENVELOPE_SHAPE) as Record<
			string,
			unknown
		>;
		expect(out.data).toEqual({
			players: [{ pos: { x: 1 } }, { pos: { x: 4 } }, { pos: { x: 7 } }]
		});
	});

	it('is idempotent when an ancestor and descendant of the same path are both selected', () => {
		const out = scopeAtPath(
			v2Env,
			['data.game_globals', 'data.game_globals.active'],
			V2_ENVELOPE_SHAPE
		) as Record<string, unknown>;
		expect(out.data).toEqual({ game_globals: { active: 1, time_limit: 600 } });
	});

	it('selecting an array branch (no [] segment) returns the whole array', () => {
		const out = scopeAtPath(v2Env, ['data.players'], V2_ENVELOPE_SHAPE) as Record<string, unknown>;
		expect(out.data).toEqual({ players: v2Env.data.players });
	});
});

describe('scopeAtPath — event shape', () => {
	it('returns the full event when nothing is selected', () => {
		expect(scopeAtPath(eventEnv, [], EVENT_SHAPE)).toBe(eventEnv);
	});

	it('emits scalars alongside selected fields (no data wrapper)', () => {
		const out = scopeAtPath(eventEnv, ['victim.name'], EVENT_SHAPE) as Record<string, unknown>;
		expect(out).toEqual({
			seq: 99,
			tick: 4242,
			at: '2026-05-18T00:00:01Z',
			event_type: 'death',
			victim: { name: 'A' }
		});
	});

	it('deep-merges multiple paths flat beside scalars', () => {
		const out = scopeAtPath(eventEnv, ['victim.name', 'killer.name'], EVENT_SHAPE) as Record<
			string,
			unknown
		>;
		expect(out.victim).toEqual({ name: 'A' });
		expect(out.killer).toEqual({ name: 'B' });
	});
});
