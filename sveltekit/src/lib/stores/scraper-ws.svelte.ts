import { browser } from '$app/environment';
import { SvelteSet } from 'svelte/reactivity';
import type {
	CurrentStatePayload,
	CurrentStateSnapshot,
	Envelope,
	EventsResponsePayload,
	GameData,
	HostSummary,
	Phase,
	PreviousGameInfo,
	StateUpdatePayload,
	TickPayload,
	WSMessage
} from '$lib/types/scraper';
import {
	HOST_ALL_ROOM,
	HOST_ROOM_PREFIX,
	isCurrentState,
	isEvent,
	isEventsReply,
	isStateUpdate
} from '$lib/types/scraper';
import { wsBaseURL } from '$lib/utils/api-base';

const reconnectDelays = [1000, 2000, 4000, 8000, 15000, 30000];
const MAX_EVENTS_PER_INSTANCE = 100;

// Reserved instance string carried in the host:all aggregate envelope's
// Instance field — backend marshals it from internal/scraper/manager/aggregator.go.
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
	let gameData = $state<Record<string, GameData | null>>({});
	let ticks = $state<Record<string, TickPayload | null>>({});
	// Most-recent envelope.tick value, updated by every envelope kind. The
	// debug page prefers this over the 3s HTTP-poll value so the engine-tick
	// counter advances at WS cadence (~30Hz in_game) instead of stuttering.
	let tickNumbers = $state<Record<string, number>>({});
	// Receive-timestamps (epoch ms) — used by debug page to surface staleness.
	let gameDataAt = $state<Record<string, number>>({});
	let ticksAt = $state<Record<string, number>>({});
	// Rolling per-instance event log; newest first, capped at
	// MAX_EVENTS_PER_INSTANCE. Replaced wholesale by current_state envelopes
	// (atomic cache read), prepended on live event envelopes, and merged on
	// events-reply envelopes.
	let events = $state<Record<string, Envelope[]>>({});
	// Phase + freshness sourced from current_state / state_update payloads.
	let phases = $state<Record<string, Phase>>({});
	let lastReadAt = $state<Record<string, string>>({});
	// Just-ended match populated by Live → Ready transitions; null when the
	// runner drops it (Ready → Idle).
	let previousGames = $state<Record<string, PreviousGameInfo | null>>({});
	// host:all aggregate cache. Keys are instance names.
	let hostSummaries = $state<Record<string, HostSummary>>({});
	// Instance names from the most recent host:all snapshot, in receive order.
	let hostList = $state<string[]>([]);
	// Last events-reply receipt + the phase the runner reported at reply time
	// (UI shows "synced Xs ago"; phase tells the user why an empty list is
	// empty).
	let lastEventsReplyAt = $state<Record<string, number>>({});
	let lastEventsReplyPhase = $state<Record<string, Phase>>({});
	// Slow-moving snapshot from the most recent current_state envelope — see
	// CurrentStateSnapshot. engine_tick / iterations advance every poll, so
	// applyStateUpdate refreshes those two fields on the existing snapshot
	// without re-issuing identity / EEPROM / XBE / kernel data.
	let snapshots = $state<Record<string, CurrentStateSnapshot>>({});
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
			const room = `${HOST_ROOM_PREFIX}${name}`;
			if (subscribed.has(room)) continue;
			subscribed.add(room);
			ws.send(JSON.stringify({ type: 'join_room', room }));
		}
	}

	function sendJSON(value: unknown): boolean {
		if (!ws || ws.readyState !== WebSocket.OPEN) return false;
		ws.send(JSON.stringify(value));
		return true;
	}

	function requestState(): boolean {
		return sendJSON({ type: 'request_state' });
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

	function handleHostAll(env: Envelope) {
		// host:all "current_state" envelopes carry a HostSummary[] payload —
		// see internal/scraper/manager/aggregator.go marshalEnvelope.
		const payload = env.payload;
		if (!Array.isArray(payload)) return;
		const next: Record<string, HostSummary> = {};
		const order: string[] = [];
		for (const entry of payload) {
			if (
				entry &&
				typeof entry === 'object' &&
				typeof (entry as HostSummary).instance === 'string'
			) {
				const summary = entry as HostSummary;
				next[summary.instance] = summary;
				order.push(summary.instance);
			}
		}
		hostSummaries = next;
		hostList = order;
		ensureSubscribed(order);
	}

	function applyCurrentState(name: string, payload: CurrentStatePayload, now: number) {
		phases = { ...phases, [name]: payload.phase };
		if (payload.last_read_at) {
			lastReadAt = { ...lastReadAt, [name]: payload.last_read_at };
		}
		// Atomic cache-read semantics: assign unconditionally so a Live → Idle
		// transition (which carries game_data: null) clears the stale Live
		// snapshot rather than preserving it.
		const newGameData = payload.game_data ?? null;
		gameData = { ...gameData, [name]: newGameData };
		if (newGameData) gameDataAt = { ...gameDataAt, [name]: now };

		const newTick = payload.latest_tick ?? null;
		ticks = { ...ticks, [name]: newTick };
		if (newTick) ticksAt = { ...ticksAt, [name]: now };

		// The cache stores events newest-first; preserve that order so the
		// debug page's existing render (which expects newest-first) keeps
		// working. Capped at MAX_EVENTS_PER_INSTANCE on the way in.
		const cachedEvents = payload.events ?? [];
		events = {
			...events,
			[name]: cachedEvents.slice(0, MAX_EVENTS_PER_INSTANCE)
		};

		previousGames = { ...previousGames, [name]: payload.previous_game ?? null };

		// Capture the slow-moving portion of the payload — identity, EEPROM,
		// XBE certificate, kernel clock — so the Xbox / Runtime tabs render
		// the whole envelope, not just the parts that have dedicated stores.
		const snapshot: CurrentStateSnapshot = {
			started_at: payload.started_at,
			title_id: payload.title_id,
			title: payload.title,
			xbox_name: payload.xbox_name,
			engine_tick: payload.engine_tick,
			iterations: payload.iterations,
			serial_number: payload.serial_number,
			mac_address: payload.mac_address,
			video_standard: payload.video_standard,
			time_zone_bias: payload.time_zone_bias,
			time_zone_std_name: payload.time_zone_std_name,
			time_zone_dlt_name: payload.time_zone_dlt_name,
			xbe_title_name: payload.xbe_title_name,
			xbe_version: payload.xbe_version,
			xbe_game_region: payload.xbe_game_region,
			xbe_disk_number: payload.xbe_disk_number,
			xbe_allowed_media: payload.xbe_allowed_media,
			kernel_system_time: payload.kernel_system_time,
			kernel_boot_time: payload.kernel_boot_time,
			kernel_uptime_ns: payload.kernel_uptime_ns
		};
		snapshots = { ...snapshots, [name]: snapshot };
	}

	function applyStateUpdate(name: string, payload: StateUpdatePayload, now: number) {
		phases = { ...phases, [name]: payload.phase };
		if (payload.last_read_at) {
			lastReadAt = { ...lastReadAt, [name]: payload.last_read_at };
		}
		if (payload.ready !== undefined) {
			gameData = { ...gameData, [name]: payload.ready };
			if (payload.ready) gameDataAt = { ...gameDataAt, [name]: now };
		}
		if (payload.tick !== undefined) {
			ticks = { ...ticks, [name]: payload.tick };
			if (payload.tick) ticksAt = { ...ticksAt, [name]: now };
		}
		// Advance the snapshot's two volatile counters. Live broadcasts at
		// ~30Hz; the identity / EEPROM / XBE / kernel fields the snapshot also
		// holds are stable, so we preserve them by reading the prior snapshot
		// and only overwriting iterations + engine_tick (and last_read_at
		// effectively via the lastReadAt store above).
		const prior = snapshots[name];
		if (prior) {
			snapshots = {
				...snapshots,
				[name]: {
					...prior,
					iterations: payload.iterations ?? prior.iterations,
					engine_tick: payload.engine_tick ?? prior.engine_tick
				}
			};
		}
	}

	function applyEventsReply(name: string, payload: EventsResponsePayload, now: number) {
		// Reply events are oldest-first (backend OQ7 resolution). Merge into
		// the local newest-first log by tick — duplicates skipped so a resync
		// after a brief disconnect doesn't double-append events received both
		// live and via the reply.
		const existing = events[name] ?? [];
		const seenTicks = new SvelteSet<number>();
		for (const env of existing) seenTicks.add(env.tick);
		const merged = [...existing];
		// Walk newest-first (reverse the oldest-first reply) and prepend so
		// the local store stays newest-first overall.
		for (let i = payload.events.length - 1; i >= 0; i--) {
			const env = payload.events[i];
			if (seenTicks.has(env.tick)) continue;
			seenTicks.add(env.tick);
			// Insert in tick-descending position. Replies are usually small
			// (gap-fill), so a linear find is fine.
			let idx = 0;
			while (idx < merged.length && merged[idx].tick > env.tick) idx++;
			merged.splice(idx, 0, env);
		}
		events = { ...events, [name]: merged.slice(0, MAX_EVENTS_PER_INSTANCE) };
		lastEventsReplyAt = { ...lastEventsReplyAt, [name]: now };
		lastEventsReplyPhase = { ...lastEventsReplyPhase, [name]: payload.phase };
	}

	function handleEnvelope(env: Envelope) {
		// host:all summary feed has env.instance === "all". Distinguishable
		// from per-instance envelopes (which carry the runner's name) without
		// a dedicated wire-type — see backend aggregator.marshalEnvelope.
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

		if (isCurrentState(env)) {
			applyCurrentState(env.instance, env.payload, now);
		} else if (isStateUpdate(env)) {
			applyStateUpdate(env.instance, env.payload, now);
		} else if (isEvent(env)) {
			const prev = events[env.instance] ?? [];
			// Defensive dedup against replay races / backend retransmission:
			// when a current_state replay (whose cached events list already
			// contains the in-flight live event) reaches the client just before
			// the matching live broadcast, prepending blindly would produce two
			// identical tiles in the feed. The log is newest-first, so events
			// sharing a tick cluster at the front — walk them and skip the
			// prepend if any has an identical payload.
			let envPayloadJSON: string | null;
			try {
				envPayloadJSON = JSON.stringify(env.payload);
			} catch {
				envPayloadJSON = null;
			}
			if (envPayloadJSON !== null) {
				for (let i = 0; i < prev.length && prev[i].tick === env.tick; i++) {
					try {
						if (JSON.stringify(prev[i].payload) === envPayloadJSON) {
							return;
						}
					} catch {
						// fall through — accept the prepend rather than drop a
						// legitimate event if stringify ever throws.
					}
				}
			}
			const next = [env, ...prev].slice(0, MAX_EVENTS_PER_INSTANCE);
			events = { ...events, [env.instance]: next };
		} else if (isEventsReply(env)) {
			applyEventsReply(env.instance, env.payload, now);
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
			subscribed.add(HOST_ALL_ROOM);
			ws?.send(JSON.stringify({ type: 'join_room', room: HOST_ALL_ROOM }));
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
		get phases() {
			return phases;
		},
		get lastReadAt() {
			return lastReadAt;
		},
		get previousGames() {
			return previousGames;
		},
		get hostSummaries() {
			return hostSummaries;
		},
		get hostList() {
			return hostList;
		},
		get lastEventsReplyAt() {
			return lastEventsReplyAt;
		},
		get lastEventsReplyPhase() {
			return lastEventsReplyPhase;
		},
		get snapshots() {
			return snapshots;
		},
		get lastError() {
			return lastError;
		},
		// Single-instance convenience accessors used by /overlays/players/.
		// Not a shim — the overlay genuinely renders one host at a time.
		get firstGameData(): GameData | null {
			const keys = Object.keys(gameData);
			for (const k of keys) {
				const gd = gameData[k];
				if (gd) return gd;
			}
			return null;
		},
		get firstTick(): TickPayload | null {
			const keys = Object.keys(ticks);
			for (const k of keys) {
				const t = ticks[k];
				if (t) return t;
			}
			return null;
		},
		connect,
		disconnect,
		ensureSubscribed,
		requestState,
		requestEvents
	};
}

export const scraperWS = createScraperWS();
