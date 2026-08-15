// Per-overlay feed lifecycle. One uniform set of reactive getters over two
// sources so an overlay page renders the same regardless of where the data
// comes from:
//   • console-ws (?console=NAME) — the live path. Resolves the console name →
//     instance (/api/overlay/console/{name}), opens a TOKENLESS WS
//     (?console=NAME — the server admits it to that instance's rooms via
//     Membership()), and subscribes to the SERVER-FILTERED
//     host:<instance>:game_filtered class (+ tick/scenario) — live PUSH, dummy
//     filtering stays server-side. machine_index is derived client-side from
//     each envelope's machines[] (indices shift live). Migration is followed on
//     the SAME socket: if the console drops out of the current instance's
//     machines[], re-resolve and re-subscribe to the new instance.
//   • mock (?mock=1) — animated sample data, no token.
//
// The original PoC's HTTP-poll transport (?transport=poll against the resolver
// @700ms) is GONE — WS push superseded it and nothing minted the param. The
// resolver endpoint itself stays: the WS path still uses it to map console name
// → instance and to follow migrations.
//
// The WS path deliberately avoids subscribeSummary / requestEvents / requestProbe
// (the Hub rejects those for an overlay connection).

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
import { apiBaseURL } from '$lib/utils/api-base';

export interface OverlayFeedOptions {
	mock: boolean;
	/** Per-class rooms to subscribe to on the live (WS) path. */
	classes: EnvelopeTypeV2[];
	/** Target purely by console name (the only live mode). */
	console?: string;
}

const MOCK_TICK_MS = 200;
/** console-ws migration/attach check cadence: cheap local machines[] scan; only
 *  re-hits the resolver when the console isn't present or isn't attached yet. */
const CONSOLE_RESOLVE_MS = 4000;

/** The console resolver's reply. Only `instance` is consumed — the live payload
 * arrives over the socket — but the endpoint returns a full snapshot, so the
 * unused fields are typed for clarity. */
interface ConsoleSnapshot {
	instance: string;
	machine_index: number;
	machine_name: string;
	game: GamePayload | null;
	tick: TickPayloadV2 | null;
	scenario: ScenarioPayload | null;
}

const sanitize = (s: string) => s.trim().toLowerCase();

/** Swap the raw `game` class for the dummy-filtered `game_filtered` class on the
 * console-ws path; other classes pass through. */
function toFilteredClasses(classes: EnvelopeTypeV2[]): EnvelopeTypeV2[] {
	return classes.map((c) => (c === 'game' ? 'game_filtered' : c));
}

export function createOverlayFeed() {
	let opts = $state<OverlayFeedOptions | null>(null);
	let frame = $state(0);
	let timer: ReturnType<typeof setInterval> | null = null;

	// console-ws state
	let wsActive = $state(false); // true once subscribed over WS
	let wsInstance = $state<string>(''); // instance currently subscribed to
	let wsClasses: EnvelopeTypeV2[] = [];
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

	// ---- console-ws: tokenless socket + server-filtered class --------------
	/** True while the subscribed instance's live game still shows this console. */
	function consolePresent(name: string): boolean {
		const machines = scraperWSV2.game[wsInstance]?.machines;
		if (!machines || machines.length === 0) return true; // no lobby data yet — don't thrash
		return machines.some((m) => sanitize(m.name) === sanitize(name));
	}

	/** Resolve console→instance and (re)subscribe. Only re-resolves when not yet
	 * attached or the console left the current instance (migration). */
	async function ensureConsoleWS(name: string): Promise<void> {
		if (wsInstance && consolePresent(name)) return;
		const s = await resolve(name);
		if (!s || !s.instance) return; // not live yet / transient — retry next tick
		if (s.instance !== wsInstance) {
			if (wsInstance) scraperWSV2.unsubscribeInstance(wsInstance, wsClasses);
			wsInstance = s.instance;
			scraperWSV2.subscribeInstance(wsInstance, wsClasses); // same socket
		}
		wsActive = true;
	}

	function startConsoleWS(name: string, classes: EnvelopeTypeV2[]): void {
		wsClasses = toFilteredClasses(classes);
		scraperWSV2.connectConsole(name); // tokenless
		void ensureConsoleWS(name);
		timer = setInterval(() => void ensureConsoleWS(name), CONSOLE_RESOLVE_MS);
	}

	function start(o: OverlayFeedOptions): void {
		opts = o;
		if (o.console) {
			startConsoleWS(o.console, o.classes);
			return;
		}
		if (o.mock) {
			timer = setInterval(() => {
				frame += 1;
			}, MOCK_TICK_MS);
			return;
		}
		// No console and not mock: nothing to feed (the M10 token mode is gone).
	}

	function stop(): void {
		if (timer !== null) {
			clearInterval(timer);
			timer = null;
		}
		if (wsActive) {
			scraperWSV2.unsubscribeInstance(wsInstance, wsClasses);
			scraperWSV2.disconnect();
		}
		wsActive = false;
		wsInstance = '';
		wsClasses = [];
		opts = null;
	}

	const isConsole = () => !!opts?.console;
	/** Live console data is only readable once the socket is subscribed. */
	const wsLive = () => isConsole() && wsActive;

	return {
		start,
		stop,
		get mock(): boolean {
			return opts?.mock ?? false;
		},
		get connected(): boolean {
			if (!opts) return false;
			if (opts.console) return wsActive && scraperWSV2.connected;
			return opts.mock ? true : scraperWSV2.connected;
		},
		get lastError(): string | null {
			if (!opts) return null;
			if (opts.console) return wsActive ? scraperWSV2.lastError : resolveErr;
			return opts.mock ? null : scraperWSV2.lastError;
		},
		/** Console mode: which machine (system-link console) the name resolves to,
		 * for per-console filtering. -1 = the instance's own console / not in the
		 * live lobby. Derived from the live envelope each read, since indices shift
		 * as the lobby changes. */
		get machineIndex(): number | null {
			if (!isConsole()) return null;
			if (!wsActive) return null;
			const machines = scraperWSV2.game[wsInstance]?.machines;
			const name = opts?.console ?? '';
			const m = machines?.find((mm) => sanitize(mm.name) === sanitize(name));
			return m ? m.index : -1;
		},
		get resolvedInstance(): string | null {
			if (!isConsole()) return null;
			return wsActive ? wsInstance : null;
		},
		get game(): GamePayload | null {
			if (!opts) return null;
			// console-ws reads the SERVER-FILTERED game slot (fed by game_filtered).
			if (opts.console) return wsLive() ? (scraperWSV2.game[wsInstance] ?? null) : null;
			return opts.mock ? mockGame(frame) : null;
		},
		get tick(): TickPayloadV2 | null {
			if (!opts) return null;
			if (opts.console) return wsLive() ? (scraperWSV2.tick[wsInstance] ?? null) : null;
			return opts.mock ? mockTick(frame) : null;
		},
		get scenario(): ScenarioPayload | null {
			if (!opts) return null;
			if (opts.console) return wsLive() ? (scraperWSV2.scenario[wsInstance] ?? null) : null;
			return opts.mock ? mockScenario() : null;
		},
		get objects(): ObjectsPayload | null {
			if (!opts) return null;
			if (opts.console) return wsLive() ? (scraperWSV2.objects[wsInstance] ?? null) : null;
			return opts.mock ? mockObjects() : null;
		},
		get events(): AnyEvent[] {
			if (!opts) return [];
			if (opts.console) return wsLive() ? (scraperWSV2.events[wsInstance] ?? []) : [];
			return opts.mock ? mockEvents(frame) : [];
		}
	};
}

export type OverlayFeed = ReturnType<typeof createOverlayFeed>;
