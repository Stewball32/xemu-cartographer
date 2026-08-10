import { browser } from '$app/environment';
import { SvelteSet } from 'svelte/reactivity';

import type {
	AnyEvent,
	DebugPayload,
	EnvelopeTypeV2,
	EnvelopeV2,
	GamePayload,
	HelloPayloadV2,
	HostSummaryV2,
	ObjectsPayload,
	PreviousGamePayload,
	ProbePayload,
	ScenarioPayload,
	SummaryPayload,
	TickPayloadV2,
	XboxPayload
} from '$lib/types/scraper-v2';
import {
	PROTOCOL_VERSION_V2,
	SUMMARY_ROOM,
	isDebugEnv,
	isEventEnv,
	isEventsReplyEnv,
	isGameEnv,
	isGameFilteredEnv,
	isHelloEnv,
	isObjectsEnv,
	isPreviousGameEnv,
	isProbeEnv,
	isScenarioEnv,
	isSummaryEnv,
	isTickEnv,
	isXboxEnv,
	roomForInstanceClass
} from '$lib/types/scraper-v2';
import { wsBaseURL } from '$lib/utils/api-base';

const reconnectDelays = [1000, 2000, 4000, 8000, 15000, 30000];
const MAX_EVENTS_PER_INSTANCE = 100;

/** Outer WS message wrapper. Server sets type="scraper" and stuffs the
 * inner v2 envelope into payload as raw JSON; SvelteKit's JSON.parse
 * already lifts it to an object. */
interface WSMessage {
	type: string;
	payload?: unknown;
}

function buildURL(token: string): string {
	return `${wsBaseURL()}/api/ws?token=${encodeURIComponent(token)}`;
}

/** Tokenless console-overlay connection: the server admits it to whichever
 * host:<instance> currently rosters this console (join_room via Membership()). */
function buildConsoleURL(console: string): string {
	return `${wsBaseURL()}/api/ws?console=${encodeURIComponent(console)}`;
}

function createScraperWSV2() {
	let ws: WebSocket | null = null;
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let attempt = 0;
	let manuallyClosed = false;
	let currentToken = '';
	// Set for a tokenless console-overlay connection; wins over currentToken when
	// building the socket URL (incl. on reconnect). Cleared on disconnect.
	let currentConsole = '';

	let connected = $state(false);
	let lastError = $state<string | null>(null);

	// Per-class per-instance latest payload. The runner emits at most one
	// envelope per (instance, class) per poll, and the join-replay path
	// re-delivers the latest cached envelope, so each slot holds the
	// freshest snapshot the client has seen.
	let xbox = $state<Record<string, XboxPayload | null>>({});
	// Full envelope (not just payload) for the xbox class — the Xbox debug
	// tab's envelope-stats header surfaces seq/tick/ts/v straight from the
	// wire. Kept as a parallel slot so existing consumers of `xbox` (which
	// only need the payload) stay unchanged.
	let xboxEnvelope = $state<Record<string, EnvelopeV2<XboxPayload> | null>>({});
	let scenario = $state<Record<string, ScenarioPayload | null>>({});
	// Full envelope (not just payload) for the scenario class — the Scenario
	// debug tab's envelope-stats header surfaces seq/tick/ts/v straight from
	// the wire. Kept as a parallel slot so existing consumers of `scenario`
	// (which only need the payload) stay unchanged.
	let scenarioEnvelope = $state<Record<string, EnvelopeV2<ScenarioPayload> | null>>({});
	let game = $state<Record<string, GamePayload | null>>({});
	// Full envelope (not just payload) for the game class — same pattern as
	// xboxEnvelope: the Game debug tab's envelope-stats header surfaces
	// seq/tick/ts/v straight from the wire while the existing `game` slot
	// stays the payload-only convenience for consumers that don't care.
	let gameEnvelope = $state<Record<string, EnvelopeV2<GamePayload> | null>>({});
	let tick = $state<Record<string, TickPayloadV2 | null>>({});
	// Full envelope (not just payload) for the tick class — the Tick debug
	// tab's envelope-stats header surfaces seq/tick/ts/v straight from the
	// wire. Kept as a parallel slot so existing consumers of `tick` (which
	// only need the payload) stay unchanged.
	let tickEnvelope = $state<Record<string, EnvelopeV2<TickPayloadV2> | null>>({});
	let objects = $state<Record<string, ObjectsPayload | null>>({});
	let debug = $state<Record<string, DebugPayload | null>>({});
	// Full envelope (not just payload) for the debug class — the Debug tab's
	// envelope-stats header surfaces seq/tick/ts/v straight from the wire and
	// the JSON view walks the full envelope shape. Kept as a parallel slot so
	// existing consumers of `debug` (which only need the payload) stay
	// unchanged.
	let debugEnvelope = $state<Record<string, EnvelopeV2<DebugPayload> | null>>({});
	let previousGame = $state<Record<string, PreviousGamePayload | null>>({});

	// Probe envelopes are request/reply only — populated by requestProbe()
	// + the matching `probe` envelope from the server. Never broadcast
	// alongside the per-tick classes. Mirror the EventsReply pattern:
	// payload slot + timestamp for "fetched Xs ago" UI.
	let probe = $state<Record<string, ProbePayload | null>>({});
	let lastProbeReplyAt = $state<Record<string, number>>({});
	// Full envelope (not just payload) for the objects class — the Objects
	// debug tab's envelope-stats header surfaces seq/tick/ts/v straight
	// from the wire. Kept as a parallel slot so existing consumers of
	// `objects` (which only need the payload) stay unchanged.
	let objectsEnvelope = $state<Record<string, EnvelopeV2<ObjectsPayload> | null>>({});
	// Full envelope (not just payload) for the previous_game class — the
	// Previous-Game debug tab's envelope-stats header surfaces seq/tick/ts/v
	// straight from the wire, and its JSON view walks the whole envelope.
	// Kept as a parallel slot so existing consumers of `previousGame`
	// (payload-only) stay unchanged.
	let previousGameEnvelope = $state<Record<string, EnvelopeV2<PreviousGamePayload> | null>>({});

	// Per-class receive timestamps (epoch ms) for staleness UI.
	let xboxAt = $state<Record<string, number>>({});
	let scenarioAt = $state<Record<string, number>>({});
	let gameAt = $state<Record<string, number>>({});
	let tickAt = $state<Record<string, number>>({});
	let objectsAt = $state<Record<string, number>>({});
	let debugAt = $state<Record<string, number>>({});

	// Rolling per-instance event log; newest first, capped at
	// MAX_EVENTS_PER_INSTANCE. Backfilled by request_events (events_reply
	// envelope). The events class join-replay path does NOT redeliver past
	// events, so the live stream is the primary source.
	let events = $state<Record<string, AnyEvent[]>>({});

	// Cross-instance summary cache from host:summary. hostList preserves
	// receive order from the most recent SummaryPayload so the dashboard
	// renders deterministically.
	let hostSummaries = $state<Record<string, HostSummaryV2>>({});
	let hostList = $state<string[]>([]);

	// Hello + per-instance started_at for runner-restart detection.
	let hello = $state<HelloPayloadV2 | null>(null);
	let instanceStartedAt = $state<Record<string, string>>({});

	// Most recent engine tick observed per instance, updated by every
	// envelope kind that carries one. UIs prefer this over per-class tickAt
	// timestamps when they want a live "engine pulse" counter.
	let engineTick = $state<Record<string, number>>({});

	// Tracking for the request_events backfill round-trip. UIs surface
	// "events reply received Xs ago" + the runner's phase at reply time
	// so an empty event list can be attributed correctly (no events vs.
	// not Live yet vs. filtered out).
	let lastEventsReplyAt = $state<Record<string, number>>({});
	let lastEventsReplyPhase = $state<Record<string, string>>({});

	// Rooms the user/component has asked to be in. Persisted across
	// reconnects: on a fresh socket we replay every intent to the server.
	// SvelteSet so consumers can reactively watch what's joined.
	const intendedRooms = new SvelteSet<string>();

	// Per-connection set of rooms we've actually sent join_room for. Empty
	// on reconnect; ensureSubscribed walks intendedRooms to re-establish.
	let liveJoins = new SvelteSet<string>();

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

	function sendJSON(value: unknown): boolean {
		if (!ws || ws.readyState !== WebSocket.OPEN) return false;
		ws.send(JSON.stringify(value));
		return true;
	}

	function sendJoin(room: string) {
		if (liveJoins.has(room)) return;
		if (!sendJSON({ type: 'join_room', room })) return;
		liveJoins.add(room);
	}

	function sendLeave(room: string) {
		if (!liveJoins.has(room)) return;
		if (!sendJSON({ type: 'leave_room', room })) return;
		liveJoins.delete(room);
	}

	/** Subscribe to one (instance, class) room. Idempotent. Re-joined on
	 * reconnect via the intendedRooms set. Pass class === undefined to
	 * subscribe to host:summary (the multi-instance dashboard feed). */
	function subscribe(instance: string, cls: EnvelopeTypeV2): void {
		const room = roomForInstanceClass(instance, cls);
		intendedRooms.add(room);
		sendJoin(room);
	}

	function subscribeSummary(): void {
		intendedRooms.add(SUMMARY_ROOM);
		sendJoin(SUMMARY_ROOM);
	}

	function unsubscribe(instance: string, cls: EnvelopeTypeV2): void {
		const room = roomForInstanceClass(instance, cls);
		intendedRooms.delete(room);
		sendLeave(room);
	}

	function unsubscribeSummary(): void {
		intendedRooms.delete(SUMMARY_ROOM);
		sendLeave(SUMMARY_ROOM);
	}

	/** Bulk helper — subscribe to several classes for one instance in one
	 * pass. Typical use from a debug page: classes = ['xbox', 'scenario',
	 * 'game', 'tick', 'objects', 'debug', 'previous_game']. */
	function subscribeInstance(instance: string, classes: EnvelopeTypeV2[]): void {
		for (const cls of classes) subscribe(instance, cls);
	}

	function unsubscribeInstance(instance: string, classes: EnvelopeTypeV2[]): void {
		for (const cls of classes) unsubscribe(instance, cls);
	}

	function requestEvents(opts?: { sinceTick?: number; types?: string[] }): boolean {
		const payload: { since_tick?: number; types?: string[] } = {};
		if (opts?.sinceTick !== undefined && opts.sinceTick > 0) {
			payload.since_tick = opts.sinceTick;
		}
		if (opts?.types && opts.types.length > 0) {
			payload.types = opts.types;
		}
		const msg: { type: string; payload?: typeof payload } = { type: 'request_events' };
		if (Object.keys(payload).length > 0) msg.payload = payload;
		return sendJSON(msg);
	}

	/** requestProbe asks the server to run the active GameReader's
	 * LastStateInputs + BuildScoreProbe for the named instance and reply
	 * with a `probe`-class envelope. Probe values are intentionally
	 * never broadcast — running BuildScoreProbe per-tick (its old home
	 * on the debug envelope) was wasted memory-read work whenever the
	 * probe page wasn't open. The reply lands in `probe[instance]` and
	 * `lastProbeReplyAt[instance]`. */
	function requestProbe(instance: string): boolean {
		return sendJSON({ type: 'request_probe', payload: { instance } });
	}

	function setSlot<T>(
		store: Record<string, T | null>,
		atStore: Record<string, number>,
		key: string,
		value: T,
		now: number
	): [Record<string, T | null>, Record<string, number>] {
		return [
			{ ...store, [key]: value },
			{ ...atStore, [key]: now }
		];
	}

	function applyHello(payload: HelloPayloadV2) {
		if (payload.protocol_version !== PROTOCOL_VERSION_V2) {
			console.warn(
				`scraper protocol version mismatch: server=${payload.protocol_version}, client=${PROTOCOL_VERSION_V2}`
			);
		}
		// Restart detection: started_at advances on runner restart, so
		// previously-cached per-(instance, class) state is stale and the
		// component should clear/refetch. Today we log; future PRs may
		// invalidate cached seq tracking via this hook.
		for (const inst of payload.instances) {
			const prev = instanceStartedAt[inst.name];
			if (prev && prev !== inst.started_at) {
				console.warn(
					`scraper runner ${inst.name} restarted (started_at: ${prev} → ${inst.started_at})`
				);
			}
		}
		const next: Record<string, string> = {};
		for (const inst of payload.instances) next[inst.name] = inst.started_at;
		instanceStartedAt = next;
		hello = payload;
	}

	function applySummary(payload: SummaryPayload) {
		const next: Record<string, HostSummaryV2> = {};
		const order: string[] = [];
		for (const h of payload.hosts) {
			next[h.instance] = h;
			order.push(h.instance);
		}
		hostSummaries = next;
		hostList = order;
	}

	function handleEnvelope(env: EnvelopeV2) {
		if (isHelloEnv(env)) {
			applyHello(env.data);
			return;
		}
		if (isSummaryEnv(env)) {
			applySummary(env.data);
			return;
		}

		const now = Date.now();
		if (env.instance && typeof env.tick === 'number' && env.tick > 0) {
			engineTick = { ...engineTick, [env.instance]: env.tick };
		}

		if (isXboxEnv(env)) {
			[xbox, xboxAt] = setSlot(xbox, xboxAt, env.instance, env.data, now);
			xboxEnvelope = { ...xboxEnvelope, [env.instance]: env };
		} else if (isScenarioEnv(env)) {
			[scenario, scenarioAt] = setSlot(scenario, scenarioAt, env.instance, env.data, now);
			scenarioEnvelope = { ...scenarioEnvelope, [env.instance]: env };
		} else if (isGameEnv(env) || isGameFilteredEnv(env)) {
			// game_filtered is the dummy-filtered game class (viewer overlays);
			// stored in the same game slot — a page subscribes to one or the other,
			// never both, so there's no clobber.
			[game, gameAt] = setSlot(game, gameAt, env.instance, env.data, now);
			gameEnvelope = { ...gameEnvelope, [env.instance]: env };
		} else if (isTickEnv(env)) {
			[tick, tickAt] = setSlot(tick, tickAt, env.instance, env.data, now);
			tickEnvelope = { ...tickEnvelope, [env.instance]: env };
		} else if (isObjectsEnv(env)) {
			[objects, objectsAt] = setSlot(objects, objectsAt, env.instance, env.data, now);
			objectsEnvelope = { ...objectsEnvelope, [env.instance]: env };
		} else if (isDebugEnv(env)) {
			[debug, debugAt] = setSlot(debug, debugAt, env.instance, env.data, now);
			debugEnvelope = { ...debugEnvelope, [env.instance]: env };
		} else if (isPreviousGameEnv(env)) {
			previousGame = { ...previousGame, [env.instance]: env.data };
			previousGameEnvelope = { ...previousGameEnvelope, [env.instance]: env };
		} else if (isProbeEnv(env)) {
			// Probe replies are one-shot answers to requestProbe — store
			// the latest payload + timestamp so the probe page can show
			// "fetched Xs ago" and re-render when a new reply arrives.
			probe = { ...probe, [env.instance]: env.data };
			lastProbeReplyAt = { ...lastProbeReplyAt, [env.instance]: now };
		} else if (isEventEnv(env)) {
			const prev = events[env.instance] ?? [];
			const next = [env.data, ...prev].slice(0, MAX_EVENTS_PER_INSTANCE);
			events = { ...events, [env.instance]: next };
		} else if (isEventsReplyEnv(env)) {
			// Backfill: events_reply.data.events is oldest-first (backend
			// OQ7). Merge into the local newest-first log keyed by (seq,
			// tick, event_type) so a resync after a brief disconnect doesn't
			// double-append events received both live and via the reply.
			const existing = events[env.instance] ?? [];
			const seen = new SvelteSet<string>();
			for (const e of existing) seen.add(`${e.seq}|${e.tick}|${e.event_type}`);
			const merged = [...existing];
			for (let i = env.data.events.length - 1; i >= 0; i--) {
				const inner = env.data.events[i].data as AnyEvent | undefined;
				if (!inner) continue;
				const key = `${inner.seq}|${inner.tick}|${inner.event_type}`;
				if (seen.has(key)) continue;
				seen.add(key);
				let idx = 0;
				while (idx < merged.length && merged[idx].tick > inner.tick) idx++;
				merged.splice(idx, 0, inner);
			}
			events = { ...events, [env.instance]: merged.slice(0, MAX_EVENTS_PER_INSTANCE) };
			lastEventsReplyAt = { ...lastEventsReplyAt, [env.instance]: now };
			lastEventsReplyPhase = { ...lastEventsReplyPhase, [env.instance]: env.data.phase };
		}
	}

	function open(token: string) {
		if (!browser) return;
		currentToken = token;
		manuallyClosed = false;
		try {
			ws = new WebSocket(currentConsole ? buildConsoleURL(currentConsole) : buildURL(token));
		} catch (err) {
			lastError = err instanceof Error ? err.message : String(err);
			scheduleReconnect();
			return;
		}

		ws.onopen = () => {
			connected = true;
			attempt = 0;
			lastError = null;
			liveJoins = new SvelteSet<string>();
			// Re-establish every intended room on the fresh socket. Order
			// doesn't matter — the backend ignores duplicates and the
			// join-replay path is per-room idempotent.
			for (const room of intendedRooms) sendJoin(room);
		};

		ws.onmessage = (e) => {
			try {
				const msg = JSON.parse(e.data) as WSMessage;
				if (msg.type === 'scraper' && msg.payload) {
					handleEnvelope(msg.payload as EnvelopeV2);
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
			liveJoins = new SvelteSet<string>();
			if (!manuallyClosed) scheduleReconnect();
		};
	}

	function connect(token: string) {
		if (ws) return;
		currentConsole = '';
		open(token);
	}

	/** Open a TOKENLESS console-overlay socket (?console=NAME). The server admits
	 * it to whichever host:<instance> currently rosters that console; migration is
	 * handled by re-subscribing to the new instance on the same socket. */
	function connectConsole(console: string) {
		if (ws) return;
		currentConsole = console;
		open('');
	}

	function disconnect() {
		manuallyClosed = true;
		clearReconnect();
		currentConsole = '';
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
		get lastError() {
			return lastError;
		},
		get hello() {
			return hello;
		},
		get instanceStartedAt() {
			return instanceStartedAt;
		},
		get engineTick() {
			return engineTick;
		},
		get intendedRooms() {
			return intendedRooms;
		},

		// Per-class payload slots.
		get xbox() {
			return xbox;
		},
		get xboxEnvelope() {
			return xboxEnvelope;
		},
		get scenario() {
			return scenario;
		},
		get scenarioEnvelope() {
			return scenarioEnvelope;
		},
		get game() {
			return game;
		},
		get gameEnvelope() {
			return gameEnvelope;
		},
		get tick() {
			return tick;
		},
		get tickEnvelope() {
			return tickEnvelope;
		},
		get objects() {
			return objects;
		},
		get debug() {
			return debug;
		},
		get debugEnvelope() {
			return debugEnvelope;
		},
		get previousGame() {
			return previousGame;
		},
		get previousGameEnvelope() {
			return previousGameEnvelope;
		},
		get probe() {
			return probe;
		},
		get lastProbeReplyAt() {
			return lastProbeReplyAt;
		},
		get events() {
			return events;
		},
		get objectsEnvelope() {
			return objectsEnvelope;
		},

		// Per-class receive timestamps.
		get xboxAt() {
			return xboxAt;
		},
		get scenarioAt() {
			return scenarioAt;
		},
		get gameAt() {
			return gameAt;
		},
		get tickAt() {
			return tickAt;
		},
		get objectsAt() {
			return objectsAt;
		},
		get debugAt() {
			return debugAt;
		},

		// host:summary feed.
		get hostSummaries() {
			return hostSummaries;
		},
		get hostList() {
			return hostList;
		},

		// request_events backfill timestamps/phase.
		get lastEventsReplyAt() {
			return lastEventsReplyAt;
		},
		get lastEventsReplyPhase() {
			return lastEventsReplyPhase;
		},

		connect,
		connectConsole,
		disconnect,
		subscribe,
		subscribeSummary,
		subscribeInstance,
		unsubscribe,
		unsubscribeSummary,
		unsubscribeInstance,
		requestEvents,
		requestProbe
	};
}

export const scraperWSV2 = createScraperWSV2();
