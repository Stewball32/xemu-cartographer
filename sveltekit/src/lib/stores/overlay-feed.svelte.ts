// Per-overlay feed lifecycle. One uniform set of reactive getters over four
// sources so an overlay page renders the same regardless of where the data
// comes from:
//   • WS (token) — the M10 overlay-token path: subscribe to ONE instance's
//     host:<instance> per-class rooms (game/tick/scenario/...).
//   • console-ws (?console=NAME) — DEFAULT for console targeting: resolve the
//     console name ONCE (/api/overlay/console/{name}) to {instance, token,
//     filter}, then ride the SAME host:<instance> WS rooms via scraperWSV2 —
//     live PUSH, not polling. machine_index is derived client-side from each
//     game envelope's machines[] (indices shift live). A low-frequency check
//     follows console migration: if the name drops out of the current
//     instance's machines[] (or the socket drops), re-resolve and re-subscribe
//     to the new host. The neutral-host dummy is re-filtered client-side (the
//     WS broadcast is unfiltered; see overlay-filter).
//   • console-poll (fallback) — the original PoC: poll /api/overlay/console/{name}
//     @700ms. Used when the resolver returns no token (older backend / mint
//     failure) or when ?transport=poll forces it. Kept intact on purpose.
//   • mock (?mock=1) — animated sample data, no token.
//
// The WS path deliberately avoids subscribeSummary / requestEvents / requestProbe
// (the Hub rejects those for an overlay connection). SECURITY DEFERRED — the
// console resolver is unauthenticated and mints the scoped token for now.

import type {
	AnyEvent,
	EnvelopeTypeV2,
	GamePayload,
	ObjectsPayload,
	ScenarioPayload,
	TickPayloadV2
} from '$lib/types/scraper-v2';
import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import { mockEvents, mockGame, mockObjects, mockScenario, mockTick } from '$lib/utils/overlay-mock';
import { filterRoster, type DummyFilterConfig } from '$lib/utils/overlay-filter';
import { apiBaseURL } from '$lib/utils/api-base';

export interface OverlayFeedOptions {
	instance: string;
	token: string;
	mock: boolean;
	/** Per-class rooms to subscribe to on the live (WS) path. */
	classes: EnvelopeTypeV2[];
	/** PoC: target purely by console name. Wins over instance/token. */
	console?: string;
	/** Force the HTTP-poll console fallback instead of WS push (?transport=poll). */
	consolePoll?: boolean;
}

const MOCK_TICK_MS = 200;
/** Console poll cadence (fallback path) — a PoC read loop; snappy for scores. */
const CONSOLE_POLL_MS = 700;
/** console-ws migration check cadence: cheap local machines[] scan; only
 *  re-hits the resolver when the console isn't present or the socket dropped. */
const CONSOLE_RESOLVE_MS = 4000;

/** The console resolver's reply (a v2-shaped snapshot + WS handoff fields). */
interface ConsoleSnapshot {
	instance: string;
	machine_index: number;
	machine_name: string;
	/** WS handoff: read-only token scoped to host:<instance> ("" → poll). */
	token?: string;
	/** Dummy-filter config to re-apply on the (unfiltered) WS broadcast. */
	filter?: DummyFilterConfig;
	game: GamePayload | null;
	tick: TickPayloadV2 | null;
	scenario: ScenarioPayload | null;
}

const sanitize = (s: string) => s.trim().toLowerCase();

export function createOverlayFeed() {
	let opts = $state<OverlayFeedOptions | null>(null);
	let frame = $state(0);
	let timer: ReturnType<typeof setInterval> | null = null;

	// console-poll (fallback) state
	let snap = $state<ConsoleSnapshot | null>(null);
	let pollOk = $state(false);
	let pollErr = $state<string | null>(null);

	// console-ws state
	let wsActive = $state(false); // true once resolved + subscribed over WS
	let wsInstance = $state<string>(''); // instance currently subscribed to
	let filterCfg = $state<DummyFilterConfig | null>(null);
	let resolveErr = $state<string | null>(null);

	/** Fetch the resolver once. Returns null on network/HTTP failure. */
	async function resolve(name: string): Promise<ConsoleSnapshot | null> {
		try {
			const res = await fetch(`${apiBaseURL()}/api/overlay/console/${encodeURIComponent(name)}`);
			if (!res.ok) {
				resolveErr =
					res.status === 404 ? `console "${name}" not in any live lobby` : `HTTP ${res.status}`;
				return null;
			}
			resolveErr = null;
			return (await res.json()) as ConsoleSnapshot;
		} catch (e) {
			resolveErr = e instanceof Error ? e.message : String(e);
			return null;
		}
	}

	// ---- console-poll fallback (original PoC path) -------------------------
	async function pollConsole(name: string): Promise<void> {
		const s = await resolve(name);
		if (!s) {
			pollOk = false;
			pollErr = resolveErr;
			return;
		}
		snap = s;
		pollOk = true;
		pollErr = null;
	}

	function startConsolePoll(name: string): void {
		void pollConsole(name);
		timer = setInterval(() => void pollConsole(name), CONSOLE_POLL_MS);
	}

	// ---- console-ws (default) ---------------------------------------------
	/** Connect scraperWSV2 to the resolved instance's rooms and record state. */
	function subscribeResolved(s: ConsoleSnapshot, classes: EnvelopeTypeV2[]): void {
		wsInstance = s.instance;
		filterCfg = s.filter ?? null;
		wsActive = true;
		scraperWSV2.connect(s.token as string);
		scraperWSV2.subscribeInstance(s.instance, classes);
	}

	async function startConsoleWS(name: string, classes: EnvelopeTypeV2[]): Promise<void> {
		const s = await resolve(name);
		if (!s || !s.token) {
			// No token (older backend / mint failure) → fall back to polling.
			startConsolePoll(name);
			return;
		}
		subscribeResolved(s, classes);
		// Migration watcher: only re-resolves when the console is no longer in the
		// live roster or the socket is down — so mint happens at start + genuine
		// migration, not every tick.
		timer = setInterval(() => void checkMigration(name, classes), CONSOLE_RESOLVE_MS);
	}

	/** True while the subscribed instance's live game still shows this console. */
	function consolePresent(name: string): boolean {
		const g = scraperWSV2.game[wsInstance];
		const machines = g?.machines;
		if (!machines || machines.length === 0) return true; // no lobby data yet — don't thrash
		return machines.some((m) => sanitize(m.name) === sanitize(name));
	}

	async function checkMigration(name: string, classes: EnvelopeTypeV2[]): Promise<void> {
		if (scraperWSV2.connected && consolePresent(name)) return; // still here — nothing to do
		const s = await resolve(name);
		if (!s || !s.token) return; // transient; keep the current subscription
		if (s.instance !== wsInstance) {
			// Console migrated to a different host — move our subscription.
			scraperWSV2.unsubscribeInstance(wsInstance, classes);
			scraperWSV2.disconnect();
			subscribeResolved(s, classes);
		} else {
			// Same host (e.g. reconnect after a drop); refresh filter config.
			filterCfg = s.filter ?? null;
		}
	}

	function start(o: OverlayFeedOptions): void {
		opts = o;
		if (o.console) {
			if (o.consolePoll) startConsolePoll(o.console);
			else void startConsoleWS(o.console, o.classes);
			return;
		}
		if (o.mock) {
			timer = setInterval(() => {
				frame += 1;
			}, MOCK_TICK_MS);
			return;
		}
		if (!o.token) return; // no token → stay disconnected; page shows a hint
		scraperWSV2.connect(o.token);
		scraperWSV2.subscribeInstance(o.instance, o.classes);
	}

	function stop(): void {
		if (timer !== null) {
			clearInterval(timer);
			timer = null;
		}
		if (opts) {
			if (wsActive) {
				scraperWSV2.unsubscribeInstance(wsInstance, opts.classes);
				scraperWSV2.disconnect();
			} else if (!opts.mock && !opts.console) {
				scraperWSV2.unsubscribeInstance(opts.instance, opts.classes);
				scraperWSV2.disconnect();
			}
			// console-poll: only the interval to clear (done above).
		}
		wsActive = false;
		wsInstance = '';
		opts = null;
	}

	const isConsole = () => !!opts?.console;
	/** console-ws game with the neutral-host dummy re-filtered (WS is raw). */
	function wsGame(): GamePayload | null {
		const g = scraperWSV2.game[wsInstance] ?? null;
		if (!g) return null;
		return { ...g, players: filterRoster(g.players, filterCfg) };
	}

	return {
		start,
		stop,
		get mock(): boolean {
			return opts?.mock ?? false;
		},
		get connected(): boolean {
			if (!opts) return false;
			if (opts.console) return wsActive ? scraperWSV2.connected : pollOk;
			return opts.mock ? true : scraperWSV2.connected;
		},
		get lastError(): string | null {
			if (!opts) return null;
			if (opts.console) return wsActive ? scraperWSV2.lastError : pollErr;
			return opts.mock ? null : scraperWSV2.lastError;
		},
		get missingToken(): boolean {
			return opts != null && !opts.mock && !opts.console && !opts.token;
		},
		/** Console mode: which machine (system-link console) the name resolves to,
		 * for per-console filtering. -1 = the instance's own console / not in the
		 * live lobby. On the WS path it is derived from the live envelope each read
		 * (indices shift); on the poll path it is the resolver's snapshot value. */
		get machineIndex(): number | null {
			if (!isConsole()) return null;
			if (wsActive) {
				const machines = scraperWSV2.game[wsInstance]?.machines;
				const name = opts?.console ?? '';
				const m = machines?.find((mm) => sanitize(mm.name) === sanitize(name));
				return m ? m.index : -1;
			}
			return snap?.machine_index ?? null;
		},
		get resolvedInstance(): string | null {
			if (!isConsole()) return null;
			return wsActive ? wsInstance : (snap?.instance ?? null);
		},
		get game(): GamePayload | null {
			if (!opts) return null;
			if (opts.console) return wsActive ? wsGame() : (snap?.game ?? null);
			return opts.mock ? mockGame(frame) : (scraperWSV2.game[opts.instance] ?? null);
		},
		get tick(): TickPayloadV2 | null {
			if (!opts) return null;
			if (opts.console)
				return wsActive ? (scraperWSV2.tick[wsInstance] ?? null) : (snap?.tick ?? null);
			return opts.mock ? mockTick(frame) : (scraperWSV2.tick[opts.instance] ?? null);
		},
		get scenario(): ScenarioPayload | null {
			if (!opts) return null;
			if (opts.console)
				return wsActive ? (scraperWSV2.scenario[wsInstance] ?? null) : (snap?.scenario ?? null);
			return opts.mock ? mockScenario() : (scraperWSV2.scenario[opts.instance] ?? null);
		},
		get objects(): ObjectsPayload | null {
			if (!opts) return null;
			if (opts.console) return wsActive ? (scraperWSV2.objects[wsInstance] ?? null) : null;
			return opts.mock ? mockObjects() : (scraperWSV2.objects[opts.instance] ?? null);
		},
		get events(): AnyEvent[] {
			if (!opts) return [];
			if (opts.console) return wsActive ? (scraperWSV2.events[wsInstance] ?? []) : [];
			return opts.mock ? mockEvents(frame) : (scraperWSV2.events[opts.instance] ?? []);
		}
	};
}

export type OverlayFeed = ReturnType<typeof createOverlayFeed>;
