import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { GamePayload } from '$lib/types/scraper-v2';

// The feed drives scraperWSV2 (connect / subscribe / unsubscribe) and reads
// its per-instance slots to decide when to re-hit the console resolver. A
// hand-rolled fake stands in for both so the tests can script the lobby
// data and count the calls.
const fakeWS = {
	connected: true,
	lastError: null as string | null,
	game: {} as Record<string, GamePayload | null>,
	gameAt: {} as Record<string, number>,
	tick: {} as Record<string, unknown>,
	scenario: {} as Record<string, unknown>,
	objects: {} as Record<string, unknown>,
	events: {} as Record<string, unknown[]>,
	connectConsole: vi.fn(),
	connectSpectator: vi.fn(),
	disconnect: vi.fn(),
	subscribeInstance: vi.fn(),
	unsubscribeInstance: vi.fn()
};

vi.mock('$lib/stores/scraper-ws-v2.svelte', () => ({ scraperWSV2: fakeWS }));
vi.mock('$lib/utils/api-base', () => ({ apiBaseURL: () => 'http://test' }));

const { createOverlayFeed } = await import('./overlay-feed.svelte');

const CLASSES = ['game_filtered', 'tick', 'scenario'];
const RESOLVE_MS = 4000;

type Answer = { status: number; instance?: string } | 'network';
let answers: Answer[] = [];
let fetches = 0;
/** Pending fetch resolvers, so a test can hold a resolve in flight. */
let release: Array<() => void> = [];

function answerNext(): Answer {
	return answers.length > 1 ? answers.shift()! : (answers[0] ?? { status: 500 });
}

const fetchMock = vi.fn(() => {
	fetches++;
	const a = answerNext();
	return new Promise<Response>((resolve, reject) => {
		release.push(() => {
			if (a === 'network') {
				reject(new Error('fetch failed'));
				return;
			}
			resolve({
				ok: a.status >= 200 && a.status < 300,
				status: a.status,
				json: async () => ({ instance: a.instance ?? '', machine_index: -1, machine_name: '' })
			} as Response);
		});
	});
});

/** Let every held fetch answer and the feed act on it. */
async function flush() {
	const r = release;
	release = [];
	for (const f of r) f();
	await vi.advanceTimersByTimeAsync(0);
}

/** Tick the resolver interval once and settle. */
async function tick() {
	await vi.advanceTimersByTimeAsync(RESOLVE_MS);
	await flush();
}

/** Fresh game data on `instance` whose lobby lists the given consoles. */
function lobby(instance: string, consoles: string[]) {
	fakeWS.game[instance] = {
		machines: consoles.map((name, index) => ({ index, name }))
	} as unknown as GamePayload;
	fakeWS.gameAt[instance] = Date.now();
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.stubGlobal('fetch', fetchMock);
	answers = [];
	fetches = 0;
	release = [];
	fakeWS.game = {};
	fakeWS.gameAt = {};
	fakeWS.connectConsole.mockClear();
	fakeWS.connectSpectator.mockClear();
	fakeWS.disconnect.mockClear();
	fakeWS.subscribeInstance.mockClear();
	fakeWS.unsubscribeInstance.mockClear();
});

afterEach(() => {
	vi.unstubAllGlobals();
	vi.clearAllTimers();
	vi.useRealTimers();
});

describe('overlay feed — console resolver follow-up', () => {
	it('subscribes the resolved instance with the filtered classes', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game', 'tick', 'scenario'], console: 'box1' });
		expect(fakeWS.connectConsole).toHaveBeenCalledWith('box1');
		expect(feed.resolvedInstance).toBeNull();
		await flush();
		expect(fakeWS.subscribeInstance).toHaveBeenCalledWith('box1', CLASSES);
		expect(feed.resolvedInstance).toBe('box1');
		expect(feed.connected).toBe(true);
		feed.stop();
	});

	it('skips the resolver only while fresh lobby data lists the console', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		await flush();
		expect(fetches).toBe(1);

		// Live data with the console present: the tick is a local scan only.
		lobby('box1', ['box1', 'peer']);
		await tick();
		expect(fetches).toBe(1);

		// Same data, but stale — the runner has gone quiet — so ask again.
		fakeWS.gameAt.box1 = Date.now() - 3 * RESOLVE_MS - 1;
		await tick();
		expect(fetches).toBe(2);

		// Fresh again, console gone from the lobby (migration): ask again.
		lobby('box1', ['peer']);
		await tick();
		expect(fetches).toBe(3);
		feed.stop();
	});

	it('re-subscribes when the resolver names another instance', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		await flush();
		expect(feed.resolvedInstance).toBe('box1');

		// box1 went silent; the console now lives on box2.
		fakeWS.gameAt.box1 = Date.now() - 3 * RESOLVE_MS - 1;
		answers = [{ status: 200, instance: 'box2' }];
		await tick();
		expect(fakeWS.unsubscribeInstance).toHaveBeenCalledWith('box1', ['game_filtered']);
		expect(fakeWS.subscribeInstance).toHaveBeenLastCalledWith('box2', ['game_filtered']);
		expect(feed.resolvedInstance).toBe('box2');
		feed.stop();
	});

	it('a 404 detaches; the next 200 subscribes afresh', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		await flush();
		expect(feed.resolvedInstance).toBe('box1');

		answers = [{ status: 404 }];
		await tick();
		expect(fakeWS.unsubscribeInstance).toHaveBeenCalledWith('box1', ['game_filtered']);
		expect(feed.resolvedInstance).toBeNull();
		expect(feed.connected).toBe(false);
		expect(feed.lastError).toBe('console "box1" not in any live lobby');

		// Still gone: no churn.
		await tick();
		expect(fakeWS.unsubscribeInstance).toHaveBeenCalledTimes(1);
		expect(fakeWS.subscribeInstance).toHaveBeenCalledTimes(1);

		// Back (same instance name): subscribe again — the earlier unsubscribe
		// withdrew the intent, so this is what puts the rooms back.
		answers = [{ status: 200, instance: 'box1' }];
		await tick();
		expect(fakeWS.subscribeInstance).toHaveBeenCalledTimes(2);
		expect(fakeWS.subscribeInstance).toHaveBeenLastCalledWith('box1', ['game_filtered']);
		expect(feed.resolvedInstance).toBe('box1');
		expect(feed.lastError).toBeNull();
		feed.stop();
	});

	it('a transient failure keeps the current subscription', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		await flush();

		answers = [{ status: 503 }];
		await tick();
		answers = ['network'];
		await tick();
		expect(fakeWS.unsubscribeInstance).not.toHaveBeenCalled();
		expect(feed.resolvedInstance).toBe('box1');
		feed.stop();
	});

	it('does not overlap resolver fetches', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		// Hold the first fetch open across two ticks.
		await vi.advanceTimersByTimeAsync(2 * RESOLVE_MS);
		expect(fetches).toBe(1);
		await flush();
		expect(fakeWS.subscribeInstance).toHaveBeenCalledTimes(1);
		feed.stop();
	});

	it('stop() closes the socket even when no subscription ever took', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 404 }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		await flush();
		expect(feed.resolvedInstance).toBeNull();
		feed.stop();
		expect(fakeWS.unsubscribeInstance).not.toHaveBeenCalled();
		expect(fakeWS.disconnect).toHaveBeenCalledTimes(1);
	});

	it('a resolve that lands after stop() subscribes nothing', async () => {
		const feed = createOverlayFeed();
		answers = [{ status: 200, instance: 'box1' }];
		feed.start({ mock: false, classes: ['game'], console: 'box1' });
		feed.stop();
		await flush();
		expect(fakeWS.subscribeInstance).not.toHaveBeenCalled();
		expect(feed.resolvedInstance).toBeNull();
	});
});
