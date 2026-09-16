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
//     machines[] (or the instance goes silent), re-resolve and re-subscribe to
//     the new instance; a 404 detaches until the console is live again. A
//     join the server refuses or revokes meanwhile is retried by the store
//     itself (scraper-ws-v2: per-room backoff), so the intent is kept here.
//   • spectator (?spectator=<kid.secret>&console=NAME) — the same live path
//     carrying a spectator key minted from Studio (authz design §9). The key
//     goes on the resolver fetch as `Authorization: Bearer <key>` and on the
//     socket as `?spectator=`; its per-instance scopes replace the anonymous
//     console door, which only ever admits the instance's OWN console name.
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
	/** Spectator key (`<kid>.<secret>`) for the live path. When omitted the
	 * feed reads `?spectator=` off the page URL itself, so the overlay routes
	 * don't need to know about it. */
	spectator?: string;
}

/** spectatorFromPageURL reads `?spectator=` off the current document URL —
 * the overlay routes build their options from `nativeOverlayParams`, which
 * predates spectator keys, so the feed picks the key up itself. */
export function spectatorFromPageURL(): string {
	if (typeof window === 'undefined' || !window.location) return '';
	// Hand-parsed rather than URLSearchParams: this is a .svelte.ts module and
	// the lint rule wants the reactive flavour there, which a one-shot read
	// doesn't need. Only the first `spectator=` counts, `+` is a space.
	const m = /[?&]spectator=([^&#]*)/.exec(window.location.search);
	if (!m) return '';
	try {
		return decodeURIComponent(m[1].replace(/\+/g, ' ')).trim();
	} catch {
		return '';
	}
}

const MOCK_TICK_MS = 200;
/** console-ws migration/attach check cadence: cheap local machines[] scan; only
 *  re-hits the resolver when the console isn't present in FRESH lobby data or
 *  isn't attached yet. */
const CONSOLE_RESOLVE_MS = 4000;
/** How long the subscribed instance may go without a game envelope before its
 * cached machines[] stops counting as evidence: the game class is emitted on
 * every runner poll (≤3 s apart while idle), so silence this long means the
 * runner is gone — or the join never took — and the resolver must be asked
 * again. */
const CONSOLE_STALE_MS = 3 * CONSOLE_RESOLVE_MS;

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

/** One resolver round-trip. `gone` is the definitive answer (404: the console
 * is in no live lobby right now) as opposed to a transient failure (network,
 * 5xx, a rejected credential) where `snapshot` is null but the current
 * subscription is worth keeping. */
interface ResolveResult {
	snapshot: ConsoleSnapshot | null;
	gone: boolean;
}

const sanitize = (s: string) => s.trim().toLowerCase();

/** Swap the raw classes for their dummy-filtered counterparts on the console-ws
 * path; other classes pass through. A page asks for what it wants semantically
 * ('game', 'event') and gets the viewer-safe room. */
function toFilteredClasses(classes: EnvelopeTypeV2[]): EnvelopeTypeV2[] {
	return classes.map((c) => {
		if (c === 'game') return 'game_filtered';
		if (c === 'event') return 'event_filtered';
		return c;
	});
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
	// Spectator key for the live path ('' = anonymous console door).
	let spectator = '';
	// One resolver fetch in flight at a time; a slow one must not overlap the
	// next tick's and double-(un)subscribe.
	let resolving = false;

	/** Fetch the resolver once. A null snapshot means network/HTTP failure,
	 * with `gone` flagging the 404 case. A spectator key rides as a Bearer
	 * credential; the anonymous door sends nothing. */
	async function resolve(name: string): Promise<ResolveResult> {
		try {
			const headers: Record<string, string> = {};
			if (spectator) headers.Authorization = `Bearer ${spectator}`;
			const res = await fetch(`${apiBaseURL()}/api/overlay/console/${encodeURIComponent(name)}`, {
				headers
			});
			if (!res.ok) {
				resolveErr = resolveErrorText(name, res.status);
				return { snapshot: null, gone: res.status === 404 };
			}
			resolveErr = null;
			return { snapshot: (await res.json()) as ConsoleSnapshot, gone: false };
		} catch (e) {
			resolveErr = e instanceof Error ? e.message : String(e);
			return { snapshot: null, gone: false };
		}
	}

	function resolveErrorText(name: string, status: number): string {
		if (status === 404) return `console "${name}" not in any live lobby`;
		if (status === 401) {
			return spectator ? 'spectator key rejected (expired or revoked)' : 'console door closed';
		}
		if (status === 403) {
			return spectator
				? `spectator key not scoped for console "${name}"`
				: `console "${name}" is not this box's own console`;
		}
		return `HTTP ${status}`;
	}

	// ---- console-ws: tokenless socket + server-filtered class --------------
	/** True while the subscribed instance is still delivering game envelopes
	 * that list this console — the one state in which the resolver cannot have
	 * changed its answer (a live machines[] match is exactly what it resolves
	 * on first). No envelope yet, a silent instance (the cached slot keeps its
	 * last machines[] forever after the runner stops, or the join was refused)
	 * and a console missing from machines[] all send the caller back to the
	 * resolver. */
	function consolePresent(name: string): boolean {
		const at = scraperWSV2.gameAt[wsInstance];
		if (!at || Date.now() - at > CONSOLE_STALE_MS) return false;
		const machines = scraperWSV2.game[wsInstance]?.machines;
		if (!machines || machines.length === 0) return false;
		return machines.some((m) => sanitize(m.name) === sanitize(name));
	}

	/** Resolve console→instance and (re)subscribe. Skips the resolver only
	 * while fresh lobby data proves the current subscription; otherwise acts
	 * on whatever it answers now: a new instance (migration, or the box came
	 * back under another name) is re-subscribed on the same socket, a 404
	 * detaches so the next 200 subscribes afresh, and a transient failure
	 * keeps things as they are for the next tick. */
	async function ensureConsoleWS(name: string): Promise<void> {
		if (resolving) return;
		if (wsActive && consolePresent(name)) return;
		resolving = true;
		try {
			const r = await resolve(name);
			if (opts?.console !== name) return; // stop()/restart raced the fetch
			if (r.gone) {
				detachConsoleWS();
				return;
			}
			const s = r.snapshot;
			if (!s || !s.instance) return; // transient — retry next tick
			if (s.instance !== wsInstance) {
				if (wsInstance) scraperWSV2.unsubscribeInstance(wsInstance, wsClasses);
				wsInstance = s.instance;
				scraperWSV2.subscribeInstance(wsInstance, wsClasses); // same socket
			}
			wsActive = true;
		} finally {
			resolving = false;
		}
	}

	/** Drop the instance subscription (console not live anywhere). The socket
	 * stays up; the next successful resolve subscribes again. */
	function detachConsoleWS(): void {
		if (wsInstance) scraperWSV2.unsubscribeInstance(wsInstance, wsClasses);
		wsInstance = '';
		wsActive = false;
	}

	function startConsoleWS(name: string, classes: EnvelopeTypeV2[]): void {
		wsClasses = toFilteredClasses(classes);
		if (spectator) {
			scraperWSV2.connectSpectator(spectator, name); // ?spectator=<key>&console=NAME
		} else {
			scraperWSV2.connectConsole(name); // tokenless
		}
		void ensureConsoleWS(name);
		timer = setInterval(() => void ensureConsoleWS(name), CONSOLE_RESOLVE_MS);
	}

	function start(o: OverlayFeedOptions): void {
		opts = o;
		spectator = (o.spectator ?? spectatorFromPageURL()).trim();
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
		// The socket is opened by startConsoleWS whether or not a subscription
		// ever took, so close it on the same condition.
		if (opts?.console) {
			detachConsoleWS();
			scraperWSV2.disconnect();
		}
		wsActive = false;
		wsInstance = '';
		wsClasses = [];
		spectator = '';
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
		/** True when the live path carries a spectator key rather than using
		 * the anonymous console door. */
		get usingSpectator(): boolean {
			return !!opts?.console && spectator !== '';
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
