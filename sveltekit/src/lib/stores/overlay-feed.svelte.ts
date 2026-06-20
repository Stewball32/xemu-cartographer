// Per-overlay feed lifecycle. Wraps the shared scraperWSV2 singleton for the
// live path and swaps in animated sample data for the mock path, behind one
// uniform set of reactive getters so an overlay page renders the same way
// regardless of source.
//
// Live path uses the M10 overlay token (read-only): the page passes the token
// straight off the URL into scraperWSV2.connect(). The token is scoped to one
// instance's host:<instance> room, so we may join its per-class rooms
// (host:<instance>:game / :tick / :scenario / ...) but NEVER host:summary and
// may only send join_room/leave_room — so this factory deliberately avoids
// subscribeSummary / requestEvents / requestProbe, all of which the Hub rejects
// for an overlay connection.

import type {
	EnvelopeTypeV2,
	GamePayload,
	ObjectsPayload,
	ScenarioPayload,
	TickPayloadV2
} from '$lib/types/scraper-v2';
import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import { mockGame, mockObjects, mockScenario, mockTick } from '$lib/utils/overlay-mock';

export interface OverlayFeedOptions {
	instance: string;
	token: string;
	mock: boolean;
	/** Per-class rooms to subscribe to on the live path. */
	classes: EnvelopeTypeV2[];
}

/** ~5 fps mock animation cadence — enough to make bars wobble and scores creep
 * without burning cycles in an idle OBS preview. */
const MOCK_TICK_MS = 200;

export function createOverlayFeed() {
	let opts = $state<OverlayFeedOptions | null>(null);
	let frame = $state(0);
	let timer: ReturnType<typeof setInterval> | null = null;

	function start(o: OverlayFeedOptions): void {
		opts = o;
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
		if (opts && !opts.mock) {
			scraperWSV2.unsubscribeInstance(opts.instance, opts.classes);
			scraperWSV2.disconnect();
		}
		opts = null;
	}

	return {
		start,
		stop,
		get mock(): boolean {
			return opts?.mock ?? false;
		},
		/** Mock is always "connected"; live mirrors the socket state. */
		get connected(): boolean {
			if (!opts) return false;
			return opts.mock ? true : scraperWSV2.connected;
		},
		get lastError(): string | null {
			if (!opts || opts.mock) return null;
			return scraperWSV2.lastError;
		},
		/** True when no token was supplied to a live overlay — the page surfaces
		 * a "mint a token" hint instead of an indefinite "Connecting…". */
		get missingToken(): boolean {
			return opts != null && !opts.mock && !opts.token;
		},
		get game(): GamePayload | null {
			if (!opts) return null;
			return opts.mock ? mockGame(frame) : (scraperWSV2.game[opts.instance] ?? null);
		},
		get tick(): TickPayloadV2 | null {
			if (!opts) return null;
			return opts.mock ? mockTick(frame) : (scraperWSV2.tick[opts.instance] ?? null);
		},
		get scenario(): ScenarioPayload | null {
			if (!opts) return null;
			return opts.mock ? mockScenario() : (scraperWSV2.scenario[opts.instance] ?? null);
		},
		get objects(): ObjectsPayload | null {
			if (!opts) return null;
			return opts.mock ? mockObjects() : (scraperWSV2.objects[opts.instance] ?? null);
		}
	};
}

export type OverlayFeed = ReturnType<typeof createOverlayFeed>;
