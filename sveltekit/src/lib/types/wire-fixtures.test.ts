import { describe, it, expect } from 'vitest';
import {
	PROTOCOL_VERSION_V2,
	SCHEMA_FINISHED_GAME,
	type AnyEvent,
	type DamageEvent,
	type DeathEvent,
	type EnvelopeTypeV2,
	type EnvelopeV2,
	type EventsReplyPayload,
	type FinishedGame,
	type GamePayload,
	type GameUpdateEvent,
	type HelloPayloadV2,
	type MedalEvent,
	type PlayerUpdateEvent,
	type PreviousGamePayload,
	type ScenarioPayload,
	type SummaryPayload,
	type TickPayloadV2
} from './scraper-v2';

// Golden wire fixtures, vendored byte-for-byte from the sibling xc-scraper
// module (../xc-scraper/wire/testdata — regenerate there with
// `go test ./wire -update`, then `task sync-wire` here; `task sync-wire:check`
// fails on drift). The Go side asserts each fixture marshals and round-trips
// exactly; this test is the TS side of the same contract — every fixture must
// satisfy the vendored TS mirror (scraper-v2.ts) at compile time
// (`pnpm check`) and be a well-formed v2 envelope at runtime.
import eventDamage from './wire-fixtures/event_damage.json';
import eventDeath from './wire-fixtures/event_death.json';
import eventFiltered from './wire-fixtures/event_filtered.json';
import eventGameUpdate from './wire-fixtures/event_game_update.json';
import eventMedal from './wire-fixtures/event_medal.json';
import eventPlayerUpdate from './wire-fixtures/event_player_update.json';
import events from './wire-fixtures/events.json';
import finishedGame from './wire-fixtures/finished_game.json';
import game from './wire-fixtures/game.json';
import gameFiltered from './wire-fixtures/game_filtered.json';
import hello from './wire-fixtures/hello.json';
import previousGame from './wire-fixtures/previous_game.json';
import scenario from './wire-fixtures/scenario.json';
import summary from './wire-fixtures/summary.json';
import tick from './wire-fixtures/tick.json';

/**
 * Structural mirror of T with every literal widened to its primitive.
 *
 * `resolveJsonModule` types a JSON import like a plain `const` — `"live"` is
 * `string`, not `'live'` — so a fixture can never satisfy a union such as
 * `PhaseV2` or `EnvelopeTypeV2` directly. Loose<T> keeps the full shape
 * (required vs optional keys, nesting, arrays, `| null`) and only relaxes the
 * literal unions, which is exactly what the JSON type system can express.
 * A JSON import is not a fresh object literal, so extra keys are tolerated
 * (additive wire fields), while a missing required key or a wrong primitive
 * is a `pnpm check` error.
 */
type Loose<T> = T extends string
	? string
	: T extends number
		? number
		: T extends boolean
			? boolean
			: T extends readonly (infer U)[]
				? Loose<U>[]
				: T extends object
					? { [K in keyof T]: Loose<T[K]> }
					: T;

type LooseEnvelope<P> = Loose<EnvelopeV2<P>>;

/** Every envelope class the TS mirror knows. Typed against EnvelopeTypeV2 so
 * a typo here (or a class dropped from the mirror) is a compile error. */
const KNOWN_CLASSES: readonly EnvelopeTypeV2[] = [
	'xbox',
	'scenario',
	'game',
	'game_filtered',
	'tick',
	'objects',
	'debug',
	'probe',
	'summary',
	'previous_game',
	'event',
	'event_filtered',
	'events',
	'hello',
	'error'
];

/** Envelope fixtures, each `satisfies` its class payload type — the
 * compile-time half of the contract. `type` is the class the fixture's file
 * name promises (event_*.json are all class 'event'). */
const ENVELOPE_FIXTURES: ReadonlyArray<{
	file: string;
	type: EnvelopeTypeV2;
	env: LooseEnvelope<unknown>;
}> = [
	{
		file: 'event_damage.json',
		type: 'event',
		env: eventDamage satisfies LooseEnvelope<DamageEvent>
	},
	{ file: 'event_death.json', type: 'event', env: eventDeath satisfies LooseEnvelope<DeathEvent> },
	{
		file: 'event_filtered.json',
		type: 'event_filtered',
		env: eventFiltered satisfies LooseEnvelope<DeathEvent>
	},
	{
		file: 'event_game_update.json',
		type: 'event',
		env: eventGameUpdate satisfies LooseEnvelope<GameUpdateEvent>
	},
	{ file: 'event_medal.json', type: 'event', env: eventMedal satisfies LooseEnvelope<MedalEvent> },
	{
		file: 'event_player_update.json',
		type: 'event',
		env: eventPlayerUpdate satisfies LooseEnvelope<PlayerUpdateEvent>
	},
	{ file: 'events.json', type: 'events', env: events satisfies LooseEnvelope<EventsReplyPayload> },
	{ file: 'game.json', type: 'game', env: game satisfies LooseEnvelope<GamePayload> },
	{
		file: 'game_filtered.json',
		type: 'game_filtered',
		env: gameFiltered satisfies LooseEnvelope<GamePayload>
	},
	{ file: 'hello.json', type: 'hello', env: hello satisfies LooseEnvelope<HelloPayloadV2> },
	{
		file: 'previous_game.json',
		type: 'previous_game',
		env: previousGame satisfies LooseEnvelope<PreviousGamePayload>
	},
	{
		file: 'scenario.json',
		type: 'scenario',
		env: scenario satisfies LooseEnvelope<ScenarioPayload>
	},
	{ file: 'summary.json', type: 'summary', env: summary satisfies LooseEnvelope<SummaryPayload> },
	{ file: 'tick.json', type: 'tick', env: tick satisfies LooseEnvelope<TickPayloadV2> }
];

/** The one bare (non-envelope) artifact: the finished_game record. */
const FINISHED_GAME: Loose<FinishedGame> = finishedGame satisfies Loose<FinishedGame>;

/** Runtime key list for the envelope frame itself (mirrors wire.Envelope). */
const ENVELOPE_KEYS = ['v', 'type', 'instance', 'seq', 'tick', 'ts', 'data'] as const;

/** Every vendored fixture, discovered rather than listed, so a fixture added
 * upstream + synced here without a typed entry above fails the test. */
const ALL_FIXTURES = import.meta.glob('./wire-fixtures/*.json', {
	eager: true,
	import: 'default'
}) as Record<string, unknown>;

function basename(path: string): string {
	return path.slice(path.lastIndexOf('/') + 1);
}

describe('vendored wire fixtures', () => {
	it('every fixture on disk has a typed entry in this test', () => {
		const onDisk = Object.keys(ALL_FIXTURES).map(basename).sort();
		const typed = [...ENVELOPE_FIXTURES.map((f) => f.file), 'finished_game.json'].sort();
		expect(onDisk).toEqual(typed);
		expect(onDisk.length).toBeGreaterThan(0);
	});

	it('the glob and the static imports see the same JSON', () => {
		for (const f of ENVELOPE_FIXTURES) {
			expect(ALL_FIXTURES[`./wire-fixtures/${f.file}`]).toEqual(f.env);
		}
		expect(ALL_FIXTURES['./wire-fixtures/finished_game.json']).toEqual(FINISHED_GAME);
	});

	describe.each(ENVELOPE_FIXTURES)('$file', ({ type, env }) => {
		it('is a v2 envelope of a known class', () => {
			for (const key of ENVELOPE_KEYS) expect(env).toHaveProperty(key);
			expect(env.v).toBe(PROTOCOL_VERSION_V2);
			expect(KNOWN_CLASSES).toContain(env.type);
			expect(env.type).toBe(type);
			expect(typeof env.instance).toBe('string');
			expect(Number.isInteger(env.seq)).toBe(true);
			expect(Number.isInteger(env.tick)).toBe(true);
			expect(Number.isNaN(Date.parse(env.ts))).toBe(false);
			expect(env.data).toBeTypeOf('object');
			expect(env.data).not.toBeNull();
		});
	});

	it('hello advertises only known classes and a matching protocol version', () => {
		expect(hello.data.protocol_version).toBe(PROTOCOL_VERSION_V2);
		for (const cls of hello.data.classes) expect(KNOWN_CLASSES).toContain(cls);
		expect(hello.instance).toBe('');
	});

	it('summary is the cross-instance envelope (empty instance)', () => {
		expect(summary.instance).toBe('');
		expect(summary.data.hosts.length).toBeGreaterThan(0);
	});

	it('event fixtures carry the event_type their file name promises', () => {
		const byKind: ReadonlyArray<[Loose<AnyEvent>, AnyEvent['event_type']]> = [
			[eventDamage.data, 'damage'],
			[eventDeath.data, 'death'],
			[eventFiltered.data, 'death'],
			[eventGameUpdate.data, 'game_update'],
			[eventMedal.data, 'medal'],
			[eventPlayerUpdate.data, 'player_update']
		];
		for (const [data, kind] of byKind) expect(data.event_type).toBe(kind);
	});

	it('events reply wraps full v2 event envelopes, oldest first', () => {
		expect(events.data.events.length).toBeGreaterThan(0);
		let lastTick = -1;
		for (const ev of events.data.events) {
			for (const key of ENVELOPE_KEYS) expect(ev).toHaveProperty(key);
			expect(ev.v).toBe(PROTOCOL_VERSION_V2);
			expect(ev.type).toBe('event');
			expect(ev.instance).toBe(events.instance);
			expect(ev.tick).toBeGreaterThanOrEqual(lastTick);
			lastTick = ev.tick;
		}
	});

	it('finished_game.json is the bare xc.finished_game/1 artifact', () => {
		expect(FINISHED_GAME.schema).toBe(SCHEMA_FINISHED_GAME);
		expect(FINISHED_GAME).not.toHaveProperty('v');
		expect(FINISHED_GAME.game_uid).not.toBe('');
		expect(FINISHED_GAME.players.length).toBeGreaterThan(0);
		expect(Number.isNaN(Date.parse(FINISHED_GAME.ended_at))).toBe(false);
	});

	it('previous_game embeds the same finished_game artifact', () => {
		const embedded = previousGame.data.finished_game;
		expect(embedded).toBeDefined();
		expect(embedded.schema).toBe(SCHEMA_FINISHED_GAME);
		expect(embedded.game_uid).toBe(previousGame.data.game_uid);
		expect(embedded.end_reason).toBe(previousGame.data.end_reason);
		expect(embedded.instance).toBe(previousGame.instance);
	});
});
