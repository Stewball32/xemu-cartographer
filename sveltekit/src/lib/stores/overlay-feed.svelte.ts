// Per-overlay feed lifecycle. One uniform set of reactive getters over three
// sources so an overlay page renders the same regardless of where the data
// comes from:
//   • WS (token) — the M10 overlay-token path: subscribe to ONE instance's
//     host:<instance> per-class rooms (game/tick/scenario/...).
//   • mock (?mock=1) — animated sample data, no token.
//   • console (?console=NAME) — PROOF OF CONCEPT: poll /api/overlay/console/{name},
//     which resolves the console name to whichever host currently sees it (own
//     xbox_name or a System Link lobby peer) and returns that host's snapshot.
//     No instance, no token; re-resolves every poll so it survives the box being
//     recreated. SECURITY DEFERRED — the endpoint is unauthenticated for now.
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
	instance: string;
	token: string;
	mock: boolean;
	/** Per-class rooms to subscribe to on the live (WS) path. */
	classes: EnvelopeTypeV2[];
	/** PoC: target purely by console name (poll mode). Wins over instance/token. */
	console?: string;
}

const MOCK_TICK_MS = 200;
/** Console poll cadence — a PoC read loop; snappy enough for scores/roster. */
const CONSOLE_POLL_MS = 700;

/** The console resolver's reply (a v2-shaped snapshot + which machine matched). */
interface ConsoleSnapshot {
	instance: string;
	machine_index: number;
	machine_name: string;
	game: GamePayload | null;
	tick: TickPayloadV2 | null;
	scenario: ScenarioPayload | null;
}

export function createOverlayFeed() {
	let opts = $state<OverlayFeedOptions | null>(null);
	let frame = $state(0);
	let timer: ReturnType<typeof setInterval> | null = null;

	// console poll-mode state
	let snap = $state<ConsoleSnapshot | null>(null);
	let pollOk = $state(false);
	let pollErr = $state<string | null>(null);

	async function pollConsole(name: string): Promise<void> {
		try {
			const res = await fetch(`${apiBaseURL()}/api/overlay/console/${encodeURIComponent(name)}`);
			if (!res.ok) {
				pollOk = false;
				pollErr =
					res.status === 404 ? `console "${name}" not in any live lobby` : `HTTP ${res.status}`;
				return;
			}
			snap = (await res.json()) as ConsoleSnapshot;
			pollOk = true;
			pollErr = null;
		} catch (e) {
			pollOk = false;
			pollErr = e instanceof Error ? e.message : String(e);
		}
	}

	function start(o: OverlayFeedOptions): void {
		opts = o;
		if (o.console) {
			void pollConsole(o.console);
			timer = setInterval(() => void pollConsole(o.console as string), CONSOLE_POLL_MS);
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
		if (opts && !opts.mock && !opts.console) {
			scraperWSV2.unsubscribeInstance(opts.instance, opts.classes);
			scraperWSV2.disconnect();
		}
		opts = null;
	}

	const isConsole = () => !!opts?.console;

	return {
		start,
		stop,
		get mock(): boolean {
			return opts?.mock ?? false;
		},
		get connected(): boolean {
			if (!opts) return false;
			if (opts.console) return pollOk;
			return opts.mock ? true : scraperWSV2.connected;
		},
		get lastError(): string | null {
			if (!opts) return null;
			if (opts.console) return pollErr;
			return opts.mock ? null : scraperWSV2.lastError;
		},
		get missingToken(): boolean {
			return opts != null && !opts.mock && !opts.console && !opts.token;
		},
		/** Console poll mode: which machine (system-link console) the name resolved
		 * to, for per-console filtering. -1 = the instance's own console. */
		get machineIndex(): number | null {
			return isConsole() ? (snap?.machine_index ?? null) : null;
		},
		get resolvedInstance(): string | null {
			return isConsole() ? (snap?.instance ?? null) : null;
		},
		get game(): GamePayload | null {
			if (!opts) return null;
			if (opts.console) return snap?.game ?? null;
			return opts.mock ? mockGame(frame) : (scraperWSV2.game[opts.instance] ?? null);
		},
		get tick(): TickPayloadV2 | null {
			if (!opts) return null;
			if (opts.console) return snap?.tick ?? null;
			return opts.mock ? mockTick(frame) : (scraperWSV2.tick[opts.instance] ?? null);
		},
		get scenario(): ScenarioPayload | null {
			if (!opts) return null;
			if (opts.console) return snap?.scenario ?? null;
			return opts.mock ? mockScenario() : (scraperWSV2.scenario[opts.instance] ?? null);
		},
		get objects(): ObjectsPayload | null {
			if (!opts || opts.console) return null;
			return opts.mock ? mockObjects() : (scraperWSV2.objects[opts.instance] ?? null);
		},
		get events(): AnyEvent[] {
			if (!opts || opts.console) return [];
			return opts.mock ? mockEvents(frame) : (scraperWSV2.events[opts.instance] ?? []);
		}
	};
}

export type OverlayFeed = ReturnType<typeof createOverlayFeed>;
