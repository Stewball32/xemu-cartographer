// View-model for the Raw tab. Plain-TS — reactivity is set up at the call
// site via `$derived.by()` in `RawTab.svelte`.
//
// Lays out the v2 per-class store contents (one slot per class plus the
// auxiliary timestamp/connection state) so the operator can eyeball the
// raw cache without filtering through pretty-printed sections.

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { ScraperInspect } from '$lib/types/scraper';

type ScraperWSV2 = typeof scraperWSV2;

export type RawSource = 'inspect' | 'live';

export type RawVm = {
	inspectJson: string;
	liveJson: string;
	displayJson: string;
};

function buildLiveSnapshot(name: string, ws: ScraperWSV2): Record<string, unknown> {
	return {
		// Per-class payload slots from PR 20.
		xbox: ws.xbox[name] ?? null,
		scenario: ws.scenario[name] ?? null,
		game: ws.game[name] ?? null,
		tick: ws.tick[name] ?? null,
		objects: ws.objects[name] ?? null,
		debug: ws.debug[name] ?? null,
		previousGame: ws.previousGame[name] ?? null,
		events: ws.events[name] ?? [],

		// Receive timestamps (epoch ms) per class.
		xboxAt: ws.xboxAt[name] ?? null,
		scenarioAt: ws.scenarioAt[name] ?? null,
		gameAt: ws.gameAt[name] ?? null,
		tickAt: ws.tickAt[name] ?? null,
		objectsAt: ws.objectsAt[name] ?? null,
		debugAt: ws.debugAt[name] ?? null,

		// Engine-tick pulse + events-reply round-trip tracking.
		engineTick: ws.engineTick[name] ?? null,
		lastEventsReplyAt: ws.lastEventsReplyAt[name] ?? null,
		lastEventsReplyPhase: ws.lastEventsReplyPhase[name] ?? null,

		// Cross-instance host:summary entry for this runner.
		hostSummary: ws.hostSummaries[name] ?? null,

		// Hello/restart detection state.
		hello: ws.hello,
		instanceStartedAt: ws.instanceStartedAt[name] ?? null,

		// Connection state shared across all instances.
		connection: { connected: ws.connected, lastError: ws.lastError }
	};
}

export function buildRawVm(
	name: string,
	source: RawSource,
	inspect: ScraperInspect | null,
	ws: ScraperWSV2
): RawVm {
	const inspectJson = inspect ? JSON.stringify(inspect, null, 2) : '';
	const liveJson = JSON.stringify(buildLiveSnapshot(name, ws), null, 2);
	return {
		inspectJson,
		liveJson,
		displayJson: source === 'inspect' ? inspectJson : liveJson
	};
}
