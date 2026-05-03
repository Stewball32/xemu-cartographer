import { browser } from '$app/environment';
import type { Envelope, SnapshotPayload, TickPayload, WSMessage } from '$lib/types/scraper';
import { isSnapshot, isTick } from '$lib/types/scraper';
import { wsBaseURL } from '$lib/utils/api-base';

const reconnectDelays = [1000, 2000, 4000, 8000, 15000, 30000];
const MAX_EVENTS_PER_INSTANCE = 100;

function buildURL(token: string): string {
	return `${wsBaseURL()}/api/ws?token=${encodeURIComponent(token)}`;
}

function createScraperWS() {
	let ws: WebSocket | null = null;
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let attempt = 0;
	let manuallyClosed = false;
	let currentToken = '';

	let connected = $state(false);
	// Per-instance latest snapshot/tick. Single-instance is the common case;
	// the map keys lets a future multi-instance overlay disambiguate.
	let snapshots = $state<Record<string, SnapshotPayload>>({});
	let ticks = $state<Record<string, TickPayload>>({});
	// Most-recent envelope.tick value, updated by every envelope kind. The
	// debug page prefers this over the 3s HTTP-poll value so the engine-tick
	// counter advances at WS cadence (~30Hz in_game) instead of stuttering.
	let tickNumbers = $state<Record<string, number>>({});
	// Receive-timestamps (epoch ms) — used by debug page to surface staleness.
	let snapshotsAt = $state<Record<string, number>>({});
	let ticksAt = $state<Record<string, number>>({});
	// Rolling per-instance event log; newest first, capped at
	// MAX_EVENTS_PER_INSTANCE. Consumed by the debug page only.
	let events = $state<Record<string, Envelope[]>>({});
	let lastError = $state<string | null>(null);

	function clearReconnect() {
		if (reconnectTimer !== null) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
	}

	function scheduleReconnect() {
		if (manuallyClosed) return;
		const delay = reconnectDelays[Math.min(attempt, reconnectDelays.length - 1)];
		attempt++;
		clearReconnect();
		reconnectTimer = setTimeout(() => {
			reconnectTimer = null;
			open(currentToken);
		}, delay);
	}

	function handleEnvelope(env: Envelope) {
		const now = Date.now();
		// Every envelope carries the engine tick at broadcast time — keep the
		// most recent so the debug page's tick counter updates at WS cadence.
		if (typeof env.tick === 'number') {
			tickNumbers = { ...tickNumbers, [env.instance]: env.tick };
		}
		if (isSnapshot(env)) {
			snapshots = { ...snapshots, [env.instance]: env.payload };
			snapshotsAt = { ...snapshotsAt, [env.instance]: now };
		} else if (isTick(env)) {
			ticks = { ...ticks, [env.instance]: env.payload };
			ticksAt = { ...ticksAt, [env.instance]: now };
		} else if (env.type === 'event') {
			const prev = events[env.instance] ?? [];
			const next = [env, ...prev].slice(0, MAX_EVENTS_PER_INSTANCE);
			events = { ...events, [env.instance]: next };
		}
	}

	function open(token: string) {
		if (!browser) return;
		currentToken = token;
		manuallyClosed = false;
		try {
			ws = new WebSocket(buildURL(token));
		} catch (err) {
			lastError = err instanceof Error ? err.message : String(err);
			scheduleReconnect();
			return;
		}

		ws.onopen = () => {
			connected = true;
			attempt = 0;
			lastError = null;
			ws?.send(JSON.stringify({ type: 'join_room', room: 'overlay' }));
		};

		ws.onmessage = (e) => {
			try {
				const msg = JSON.parse(e.data) as WSMessage;
				if (msg.type === 'scraper' && msg.payload) {
					// Outer payload may arrive as parsed object (server-side json.RawMessage
					// is a JSON value; SvelteKit's JSON.parse already lifts it).
					const env = msg.payload as Envelope;
					handleEnvelope(env);
				} else if (msg.type === 'error') {
					const errPayload = msg.payload as { code?: string; message?: string } | undefined;
					lastError = errPayload?.message ?? 'websocket error';
				}
			} catch (err) {
				lastError = err instanceof Error ? err.message : String(err);
			}
		};

		ws.onerror = () => {
			lastError = 'websocket error';
		};

		ws.onclose = () => {
			connected = false;
			ws = null;
			if (!manuallyClosed) {
				scheduleReconnect();
			}
		};
	}

	function connect(token: string) {
		if (ws) return;
		open(token);
	}

	function disconnect() {
		manuallyClosed = true;
		clearReconnect();
		if (ws) {
			ws.close();
			ws = null;
		}
		connected = false;
	}

	return {
		get connected() {
			return connected;
		},
		get snapshots() {
			return snapshots;
		},
		get ticks() {
			return ticks;
		},
		get tickNumbers() {
			return tickNumbers;
		},
		get snapshotsAt() {
			return snapshotsAt;
		},
		get ticksAt() {
			return ticksAt;
		},
		get events() {
			return events;
		},
		get lastError() {
			return lastError;
		},
		// First-instance convenience accessors for single-instance overlay v1.
		get snapshot(): SnapshotPayload | null {
			const keys = Object.keys(snapshots);
			return keys.length > 0 ? snapshots[keys[0]] : null;
		},
		get tick(): TickPayload | null {
			const keys = Object.keys(ticks);
			return keys.length > 0 ? ticks[keys[0]] : null;
		},
		connect,
		disconnect
	};
}

export const scraperWS = createScraperWS();
