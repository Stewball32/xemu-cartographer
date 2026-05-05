import { browser } from '$app/environment';
import { SvelteSet } from 'svelte/reactivity';
import type { Envelope, GameData, TickPayload, WSMessage } from '$lib/types/scraper';
import { isSnapshot, isTick } from '$lib/types/scraper';
import { wsBaseURL } from '$lib/utils/api-base';

const reconnectDelays = [1000, 2000, 4000, 8000, 15000, 30000];
const MAX_EVENTS_PER_INSTANCE = 100;

// Reserved instance string used by the M5 stage 5b host:all aggregator.
// Backend matches: internal/scraper/manager/aggregator.go marshalEnvelope.
const HOST_ALL_INSTANCE = 'all';

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
	// Per-instance latest game-data / tick. Single-instance is the common
	// case; the map keys let a future multi-instance overlay disambiguate.
	let gameData = $state<Record<string, GameData>>({});
	let ticks = $state<Record<string, TickPayload>>({});
	// Most-recent envelope.tick value, updated by every envelope kind. The
	// debug page prefers this over the 3s HTTP-poll value so the engine-tick
	// counter advances at WS cadence (~30Hz in_game) instead of stuttering.
	let tickNumbers = $state<Record<string, number>>({});
	// Receive-timestamps (epoch ms) — used by debug page to surface staleness.
	let gameDataAt = $state<Record<string, number>>({});
	let ticksAt = $state<Record<string, number>>({});
	// Rolling per-instance event log; newest first, capped at
	// MAX_EVENTS_PER_INSTANCE. Consumed by the debug page only.
	let events = $state<Record<string, Envelope[]>>({});
	// Names of running scrapers as reported by the host:all summary feed.
	// Drives the per-instance auto-subscribe path so an overlay can render
	// game data without the page knowing instance names up front.
	let hostList = $state<string[]>([]);
	let lastError = $state<string | null>(null);

	// Per-connection set of host:* rooms we've already sent join_room for.
	// Cleared on reconnect so the same set is re-established against the
	// fresh WebSocket.
	let subscribed = new SvelteSet<string>();

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

	function ensureSubscribed(names: string[]) {
		if (!ws || ws.readyState !== WebSocket.OPEN) return;
		for (const name of names) {
			const room = `host:${name}`;
			if (subscribed.has(room)) continue;
			subscribed.add(room);
			ws.send(JSON.stringify({ type: 'join_room', room }));
		}
	}

	function handleHostAll(env: Envelope) {
		// host:all rides the legacy "snapshot" wire type; payload is an array
		// of summary records. Type as unknown[] until M5 stage 5c lands a
		// dedicated host-summary envelope.
		const payload = env.payload;
		if (!Array.isArray(payload)) return;
		const next: string[] = [];
		for (const entry of payload) {
			if (
				entry &&
				typeof entry === 'object' &&
				typeof (entry as { instance?: unknown }).instance === 'string'
			) {
				next.push((entry as { instance: string }).instance);
			}
		}
		hostList = next;
		ensureSubscribed(next);
	}

	function handleEnvelope(env: Envelope) {
		// host:all summary feed has env.instance === "all". Distinguishable
		// from per-instance envelopes (which carry the runner's name) without
		// a wire-type change — see M5 stage 5b plan.
		if (env.instance === HOST_ALL_INSTANCE) {
			handleHostAll(env);
			return;
		}

		const now = Date.now();
		// Every envelope carries the engine tick at broadcast time — keep the
		// most recent so the debug page's tick counter updates at WS cadence.
		if (typeof env.tick === 'number') {
			tickNumbers = { ...tickNumbers, [env.instance]: env.tick };
		}
		// isSnapshot() narrows on the legacy "snapshot" wire-type string —
		// the payload is GameData. (Stage 5c will replace the wire string
		// with "current_state".)
		if (isSnapshot(env)) {
			gameData = { ...gameData, [env.instance]: env.payload };
			gameDataAt = { ...gameDataAt, [env.instance]: now };
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
			subscribed = new SvelteSet<string>();
			// Subscribe to the cross-instance summary feed first; the
			// payload's instance list drives ensureSubscribed for per-
			// instance host:<name> rooms.
			subscribed.add('host:all');
			ws?.send(JSON.stringify({ type: 'join_room', room: 'host:all' }));
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
			subscribed = new SvelteSet<string>();
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
		get gameData() {
			return gameData;
		},
		get ticks() {
			return ticks;
		},
		get tickNumbers() {
			return tickNumbers;
		},
		get gameDataAt() {
			return gameDataAt;
		},
		get ticksAt() {
			return ticksAt;
		},
		get events() {
			return events;
		},
		get hostList() {
			return hostList;
		},
		get lastError() {
			return lastError;
		},
		// First-instance convenience accessors for single-instance overlay v1.
		get firstGameData(): GameData | null {
			const keys = Object.keys(gameData);
			return keys.length > 0 ? gameData[keys[0]] : null;
		},
		get firstTick(): TickPayload | null {
			const keys = Object.keys(ticks);
			return keys.length > 0 ? ticks[keys[0]] : null;
		},
		connect,
		disconnect
	};
}

export const scraperWS = createScraperWS();
