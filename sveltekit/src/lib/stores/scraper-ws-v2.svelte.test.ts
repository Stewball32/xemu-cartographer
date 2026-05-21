import { describe, it, expect, vi } from 'vitest';

// The v2 store gates WebSocket construction on $app/environment.browser.
// Mock it false so the store imports without trying to instantiate a
// real WebSocket during the test run.
vi.mock('$app/environment', () => ({ browser: false }));

// wsBaseURL is consulted inside buildURL; never called in these tests
// (browser=false short-circuits) but the import must resolve.
vi.mock('$lib/utils/api-base', () => ({ wsBaseURL: () => 'ws://test' }));

const { scraperWSV2 } = await import('./scraper-ws-v2.svelte');

describe('scraperWSV2 initial state', () => {
	it('starts disconnected with empty per-class slots', () => {
		expect(scraperWSV2.connected).toBe(false);
		expect(scraperWSV2.hello).toBeNull();
		expect(scraperWSV2.hostList).toEqual([]);
		expect(scraperWSV2.xbox).toEqual({});
		expect(scraperWSV2.tick).toEqual({});
		expect(scraperWSV2.events).toEqual({});
	});

	it('subscribe/unsubscribe mutate intendedRooms even while disconnected', () => {
		expect(scraperWSV2.intendedRooms.has('host:alpha:tick')).toBe(false);
		scraperWSV2.subscribe('alpha', 'tick');
		// Even with no live socket the intent is recorded so on connect
		// the store can replay the join.
		expect(scraperWSV2.intendedRooms.has('host:alpha:tick')).toBe(true);
		scraperWSV2.unsubscribe('alpha', 'tick');
		expect(scraperWSV2.intendedRooms.has('host:alpha:tick')).toBe(false);
	});

	it('subscribeSummary / unsubscribeSummary track host:summary', () => {
		scraperWSV2.subscribeSummary();
		expect(scraperWSV2.intendedRooms.has('host:summary')).toBe(true);
		scraperWSV2.unsubscribeSummary();
		expect(scraperWSV2.intendedRooms.has('host:summary')).toBe(false);
	});

	it('subscribeInstance bulk-adds all classes', () => {
		scraperWSV2.subscribeInstance('beta', ['xbox', 'game', 'tick']);
		expect(scraperWSV2.intendedRooms.has('host:beta:xbox')).toBe(true);
		expect(scraperWSV2.intendedRooms.has('host:beta:game')).toBe(true);
		expect(scraperWSV2.intendedRooms.has('host:beta:tick')).toBe(true);
		scraperWSV2.unsubscribeInstance('beta', ['xbox', 'game', 'tick']);
		expect(scraperWSV2.intendedRooms.has('host:beta:xbox')).toBe(false);
		expect(scraperWSV2.intendedRooms.has('host:beta:game')).toBe(false);
		expect(scraperWSV2.intendedRooms.has('host:beta:tick')).toBe(false);
	});

	it('connect is a no-op when browser=false', () => {
		// connect builds a WebSocket via the `new WebSocket(...)` constructor;
		// the browser=false guard at the top of open() must skip that.
		scraperWSV2.connect('fake-token');
		expect(scraperWSV2.connected).toBe(false);
	});
});
