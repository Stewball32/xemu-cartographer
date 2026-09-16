import { describe, it, expect } from 'vitest';
import { canonScope, hasScope, matchScope, parseScope } from './scopes';

// The design §3.2 matching table, one case per row — copied verbatim from
// internal/authz/scope_test.go TestMatch so the TS mirror can't drift from
// the Go matcher (§8.1 S7).
const rows: Array<[scope: string, want: string, ok: boolean]> = [
	['overlay.read_state:box1', 'overlay.read_state:box1', true], // 1
	['overlay.read_state:box1', 'overlay.read_state:box2', false], // 2
	['overlay.read_state:*', 'overlay.read_state:box2', true], // 3
	['overlay.*:box1', 'overlay.read_state:box1', true], // 4
	['overlay.*:*', 'overlay.mint', true], // 5
	['lan.*', 'lan.saves.file:gametype/abc', true], // 6
	['lan.saves.*', 'lan.sync.manifest', false], // 7
	['room.join:host:*:game_filtered', 'room.join:host:box1:game_filtered', true], // 8
	['room.join:host:*:game_filtered', 'room.join:host:box1', false], // 9
	['room.join:host:*:game_filtered', 'room.join:host:summary', false], // 10
	['room.join:host:box1:*', 'room.join:host:box1:tick', true], // 11
	['room.join:*', 'room.join:host:summary', true], // 12
	['*', 'token.mint', true], // 13
	['library.manage', 'library.manage', true], // 14
	['library.manage', 'library.manage:iso1', false], // 15
	['Overlay.Read_State:Box1', 'overlay.read_state:box1', false], // 16
	['overlay.read_state:', 'overlay.read_state:box1', false], // 17
	['overlay.read_state:bo*x', 'overlay.read_state:box', false], // 18
	['nosuch.action:*', 'nosuch.action:x', false], // 19
	['room.join:host:*:*', 'room.join:host:box1:tick', true] // 20
];

describe('hasScope — §3.2 matching table', () => {
	rows.forEach(([scope, want, ok], i) => {
		const name = `row${String(i + 1).padStart(2, '0')}`;
		it(`${name}: hasScope([${JSON.stringify(scope)}], ${JSON.stringify(want)}) = ${ok}`, () => {
			expect(hasScope([scope], want)).toBe(ok);
			expect(matchScope(scope, want)).toBe(ok);
		});
	});
});

describe('hasScope — want canonicalisation and edge cases', () => {
	it('canonicalises the want selector but keeps the scope side strict', () => {
		expect(matchScope('overlay.read_state:box 1', 'overlay.read_state:  Box   1 ')).toBe(true);
		expect(matchScope('overlay.read_state:box1', 'Overlay.Read_State:box1')).toBe(false);
	});

	it('never matches an empty scope or want', () => {
		expect(matchScope('overlay.read_state:box1', '')).toBe(false);
		expect(matchScope('', 'overlay.read_state:box1')).toBe(false);
	});

	it('treats "*" in a want as a literal', () => {
		expect(matchScope('overlay.read_state:box1', 'overlay.read_state:*')).toBe(false);
	});

	it('returns the first match across a list and misses on empty lists', () => {
		const scopes = ['admin.*', 'overlay.read_state:box1', 'room.join:host:*:tick'];
		expect(hasScope(scopes, 'overlay.read_state:box1')).toBe(true);
		expect(hasScope(scopes, 'admin.users')).toBe(true);
		expect(hasScope(scopes, 'token.mint')).toBe(false);
		expect(hasScope([], 'token.mint')).toBe(false);
		expect(hasScope(null, 'token.mint')).toBe(false);
		expect(hasScope(undefined, 'token.mint')).toBe(false);
	});

	it('the superuser sentinel ["*"] grants every known action', () => {
		expect(hasScope(['*'], 'token.mint')).toBe(true);
		expect(hasScope(['*'], 'lan.saves.file:gametype/abc')).toBe(true);
		expect(hasScope(['*'], 'nosuch.action')).toBe(false);
	});
});

describe('parseScope', () => {
	it('rejects the Go TestParseScopeRejects set', () => {
		const bad = [
			'',
			':',
			':box1',
			'overlay.read_state:',
			'overlay.read_state::box1',
			'Overlay.Read_State',
			'overlay.read_state:Box1',
			'overlay.read_state:bo*x',
			'overlay.read_state:b*',
			'overlay.read_state:box/1',
			'overlay.read_state:box@1',
			'nosuch.action',
			'nosuch.*',
			'overlay',
			'overlay.*x',
			'*:box1',
			'*.*',
			' overlay.read_state',
			'overlay.read_state ',
			'lan.saves.file:gametype/abc'
		];
		for (const s of bad) {
			expect(parseScope(s), s).toBeNull();
		}
	});

	it('accepts the Go TestParseScopeRejects good set', () => {
		expect(parseScope('*')).toEqual({ action: '', family: false, selector: null, any: true });
		expect(parseScope('lan.*')).toEqual({
			action: 'lan',
			family: true,
			selector: null,
			any: false
		});
		expect(parseScope('lan.saves.*')).toEqual({
			action: 'lan.saves',
			family: true,
			selector: null,
			any: false
		});
		expect(parseScope('overlay.*:box1')).toEqual({
			action: 'overlay',
			family: true,
			selector: ['box1'],
			any: false
		});
		expect(parseScope('library.manage')).toEqual({
			action: 'library.manage',
			family: false,
			selector: null,
			any: false
		});
		expect(parseScope('overlay.read_state:*')).toEqual({
			action: 'overlay.read_state',
			family: false,
			selector: ['*'],
			any: false
		});
		expect(parseScope('room.join:host:*:game_filtered')).toEqual({
			action: 'room.join',
			family: false,
			selector: ['host', '*', 'game_filtered'],
			any: false
		});
		expect(parseScope('overlay.read_state:box 1.a-b_c')).toEqual({
			action: 'overlay.read_state',
			family: false,
			selector: ['box 1.a-b_c'],
			any: false
		});
	});
});

describe('canonScope', () => {
	it('mirrors Go Canon and is idempotent', () => {
		const cases: Record<string, string> = {
			'  Overlay.Read_State : Box   1 ': 'overlay.read_state:box 1',
			'ROOM.JOIN:HOST:*:TICK': 'room.join:host:*:tick',
			'lan.*': 'lan.*',
			'': '',
			'   ': '',
			'a:\tb  c\n': 'a:b c'
		};
		for (const [input, want] of Object.entries(cases)) {
			const got = canonScope(input);
			expect(got, input).toBe(want);
			expect(canonScope(got), input).toBe(got);
		}
	});
});
