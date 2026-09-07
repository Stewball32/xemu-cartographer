import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Unlike scraper-ws-v2.svelte.test.ts (browser=false, no socket), these
// tests need the store to open sockets so a fake WebSocket can play the
// server: refuse joins, evict rooms, deliver frames. browser=true +
// vi.stubGlobal('WebSocket', FakeWebSocket) below.
vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$lib/utils/api-base', () => ({ wsBaseURL: () => 'ws://test' }));

/** Minimal stand-in for the browser WebSocket: records what the store sent,
 * lets a test flip it open / closed and push frames through onmessage. */
class FakeWebSocket {
	static readonly CONNECTING = 0;
	static readonly OPEN = 1;
	static readonly CLOSING = 2;
	static readonly CLOSED = 3;
	static instances: FakeWebSocket[] = [];

	readyState = FakeWebSocket.CONNECTING;
	sent: string[] = [];
	onopen: (() => void) | null = null;
	onmessage: ((e: { data: string }) => void) | null = null;
	onerror: (() => void) | null = null;
	onclose: (() => void) | null = null;

	constructor(public url: string) {
		FakeWebSocket.instances.push(this);
	}

	send(data: string) {
		this.sent.push(data);
	}

	close() {
		this.readyState = FakeWebSocket.CLOSED;
	}

	// ---- server side ----
	open() {
		this.readyState = FakeWebSocket.OPEN;
		this.onopen?.();
	}

	serverClose() {
		this.readyState = FakeWebSocket.CLOSED;
		this.onclose?.();
	}

	receive(frame: unknown) {
		this.onmessage?.({ data: JSON.stringify(frame) });
	}

	/** join_room frames sent so far, in order, as room names. */
	joins(): string[] {
		return this.sent
			.map((s) => JSON.parse(s) as { type: string; room?: string })
			.filter((m) => m.type === 'join_room')
			.map((m) => m.room ?? '');
	}

	leaves(): string[] {
		return this.sent
			.map((s) => JSON.parse(s) as { type: string; room?: string })
			.filter((m) => m.type === 'leave_room')
			.map((m) => m.room ?? '');
	}
}

vi.stubGlobal('WebSocket', FakeWebSocket);

const { scraperWSV2 } = await import('./scraper-ws-v2.svelte');

const ROOM = 'host:box1:game_filtered';
const TICK = 'host:box1:tick';

function refuse(sock: FakeWebSocket, room: string, code = 'forbidden') {
	sock.receive({
		type: 'error',
		room,
		payload: { code, message: 'not allowed to join this room' }
	});
}

function evict(sock: FakeWebSocket, room: string) {
	sock.receive({ type: 'room_left', room, payload: { reason: 'forbidden' } });
}

/** A scraper frame for `room` — the join-replay / live push that proves the
 * join took. The envelope body is the minimum handleEnvelope tolerates. */
function deliver(sock: FakeWebSocket, room: string) {
	sock.receive({
		type: 'scraper',
		room,
		payload: {
			v: 2,
			type: 'tick',
			instance: 'box1',
			seq: 1,
			tick: 1,
			ts: '',
			data: { players: [] }
		}
	});
}

/** Open a console socket, let it connect, subscribe to ROOM and return the
 * socket with its initial join already sent. */
function connectAndJoin(): FakeWebSocket {
	scraperWSV2.connectConsole('box1');
	const sock = FakeWebSocket.instances.at(-1)!;
	sock.open();
	scraperWSV2.subscribe('box1', 'game_filtered');
	expect(sock.joins()).toEqual([ROOM]);
	return sock;
}

beforeEach(() => {
	vi.useFakeTimers();
	FakeWebSocket.instances = [];
});

afterEach(() => {
	// The store is a module singleton: withdraw every intent and close the
	// socket so no test inherits rooms or timers from the previous one.
	scraperWSV2.unsubscribeInstance('box1', ['game_filtered', 'tick']);
	scraperWSV2.disconnect();
	vi.clearAllTimers();
	vi.useRealTimers();
});

describe('scraperWSV2 re-join backoff', () => {
	it('re-sends a refused join after 2s, then 4s, 8s, 16s, capped at 30s', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		expect(scraperWSV2.lastErrorCode).toBe('forbidden');

		// 2s → second join
		vi.advanceTimersByTime(1999);
		expect(sock.joins()).toHaveLength(1);
		vi.advanceTimersByTime(1);
		expect(sock.joins()).toHaveLength(2);

		// 4s → third
		refuse(sock, ROOM);
		vi.advanceTimersByTime(3999);
		expect(sock.joins()).toHaveLength(2);
		vi.advanceTimersByTime(1);
		expect(sock.joins()).toHaveLength(3);

		// 8s → fourth
		refuse(sock, ROOM);
		vi.advanceTimersByTime(8000);
		expect(sock.joins()).toHaveLength(4);

		// 16s → fifth
		refuse(sock, ROOM);
		vi.advanceTimersByTime(15999);
		expect(sock.joins()).toHaveLength(4);
		vi.advanceTimersByTime(1);
		expect(sock.joins()).toHaveLength(5);

		// 32s would be next; capped at 30s.
		refuse(sock, ROOM);
		vi.advanceTimersByTime(29999);
		expect(sock.joins()).toHaveLength(5);
		vi.advanceTimersByTime(1);
		expect(sock.joins()).toHaveLength(6);

		// Stays at the cap.
		refuse(sock, ROOM);
		vi.advanceTimersByTime(30000);
		expect(sock.joins()).toHaveLength(7);
		expect(sock.joins().every((r) => r === ROOM)).toBe(true);
	});

	it('does not double-schedule while a retry is pending', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		refuse(sock, ROOM); // a second refusal before the timer fires
		vi.advanceTimersByTime(2000);
		expect(sock.joins()).toHaveLength(2);
		// The duplicate refusal did not bump the attempt count either: the
		// next refusal waits 4s, not 8s.
		refuse(sock, ROOM);
		vi.advanceTimersByTime(4000);
		expect(sock.joins()).toHaveLength(3);
	});

	it('a scraper frame for the room resets its backoff and clears the error', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		vi.advanceTimersByTime(2000);
		refuse(sock, ROOM);
		vi.advanceTimersByTime(4000);
		expect(sock.joins()).toHaveLength(3);
		expect(scraperWSV2.lastError).toMatch(/forbidden/);

		// The third join took: the server replays a frame for the room.
		deliver(sock, ROOM);
		expect(scraperWSV2.lastError).toBeNull();
		expect(scraperWSV2.lastErrorCode).toBeNull();

		// A later refusal starts over at 2s rather than continuing at 8s.
		refuse(sock, ROOM);
		vi.advanceTimersByTime(2000);
		expect(sock.joins()).toHaveLength(4);
	});

	it('a frame for a different room leaves another room’s error alone', () => {
		const sock = connectAndJoin();
		scraperWSV2.subscribe('box1', 'tick');
		refuse(sock, ROOM);
		deliver(sock, TICK);
		expect(scraperWSV2.lastErrorCode).toBe('forbidden');
	});

	it('room_left re-joins with the same backoff and surfaces the reason', () => {
		const sock = connectAndJoin();
		deliver(sock, ROOM);
		evict(sock, ROOM);
		expect(scraperWSV2.lastErrorCode).toBe('forbidden');
		expect(scraperWSV2.lastError).toContain(ROOM);

		vi.advanceTimersByTime(1999);
		expect(sock.joins()).toHaveLength(1);
		vi.advanceTimersByTime(1);
		expect(sock.joins()).toEqual([ROOM, ROOM]);

		// Admitted again: error gone, backoff reset.
		deliver(sock, ROOM);
		expect(scraperWSV2.lastError).toBeNull();
		evict(sock, ROOM);
		vi.advanceTimersByTime(2000);
		expect(sock.joins()).toHaveLength(3);
	});

	it('an error frame without a room does not trigger a re-join', () => {
		const sock = connectAndJoin();
		sock.receive({
			type: 'error',
			payload: { code: 'forbidden', message: 'message type not allowed for this connection' }
		});
		expect(scraperWSV2.lastErrorCode).toBe('forbidden');
		vi.advanceTimersByTime(60000);
		expect(sock.joins()).toHaveLength(1);
	});

	it('unsubscribe cancels the pending retry and its error', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		scraperWSV2.unsubscribe('box1', 'game_filtered');
		expect(scraperWSV2.lastError).toBeNull();
		vi.advanceTimersByTime(60000);
		expect(sock.joins()).toHaveLength(1);
		// The refused room was already out of liveJoins, so no leave_room
		// went out for it either.
		expect(sock.leaves()).toEqual([]);
	});

	it('a retry that fires after unsubscribe + resubscribe does not double-join', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		scraperWSV2.unsubscribe('box1', 'game_filtered');
		scraperWSV2.subscribe('box1', 'game_filtered'); // immediate re-join
		expect(sock.joins()).toEqual([ROOM, ROOM]);
		vi.advanceTimersByTime(60000);
		expect(sock.joins()).toHaveLength(2);
	});

	it('a fresh socket replays the intent with a clean backoff', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		vi.advanceTimersByTime(2000);
		refuse(sock, ROOM);
		vi.advanceTimersByTime(4000);
		refuse(sock, ROOM); // next retry would be 8s out

		// Server drops the connection; the store reconnects (1s) and replays.
		sock.serverClose();
		expect(scraperWSV2.connected).toBe(false);
		vi.advanceTimersByTime(1000);
		const next = FakeWebSocket.instances.at(-1)!;
		expect(next).not.toBe(sock);
		next.open();
		expect(next.joins()).toEqual([ROOM]);
		expect(scraperWSV2.lastError).toBeNull();

		// The old socket's pending retry did not leak onto the new one …
		vi.advanceTimersByTime(8000);
		expect(next.joins()).toHaveLength(1);
		// … and a refusal here starts at 2s again.
		refuse(next, ROOM);
		vi.advanceTimersByTime(2000);
		expect(next.joins()).toHaveLength(2);
	});

	it('disconnect cancels pending retries; the next socket replays the intent', () => {
		const sock = connectAndJoin();
		refuse(sock, ROOM);
		scraperWSV2.disconnect();
		vi.advanceTimersByTime(60000);
		expect(sock.joins()).toHaveLength(1);
		expect(sock.readyState).toBe(FakeWebSocket.CLOSED);

		scraperWSV2.connectConsole('box1');
		const next = FakeWebSocket.instances.at(-1)!;
		next.open();
		expect(next.joins()).toEqual([ROOM]);
	});

	it('session_revoked stops everything: no re-join, no reconnect', () => {
		const sock = connectAndJoin();
		sock.receive({
			type: 'error',
			room: ROOM,
			payload: { code: 'session_revoked', message: 'credential no longer valid' }
		});
		expect(scraperWSV2.revoked).toBe(true);
		sock.serverClose();
		vi.advanceTimersByTime(120000);
		expect(sock.joins()).toHaveLength(1);
		expect(FakeWebSocket.instances).toHaveLength(1);
	});

	it('events from a socket disconnect() let go of do not touch the next one', () => {
		const sock = connectAndJoin();
		scraperWSV2.disconnect();
		scraperWSV2.connectConsole('box1');
		const next = FakeWebSocket.instances.at(-1)!;
		next.open();
		expect(scraperWSV2.connected).toBe(true);
		expect(next.joins()).toEqual([ROOM]);

		// The old socket's close lands late, as it does in a browser.
		sock.serverClose();
		expect(scraperWSV2.connected).toBe(true);
		scraperWSV2.subscribe('box1', 'tick');
		expect(next.joins()).toEqual([ROOM, TICK]);
	});
});
