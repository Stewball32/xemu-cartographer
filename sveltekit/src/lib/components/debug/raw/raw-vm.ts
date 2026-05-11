// View-model for the Raw tab. Plain-TS — reactivity is set up at the call
// site via `$derived.by()` in `RawTab.svelte`.

import { scraperWS } from '$lib/stores/scraper-ws.svelte';
import type { ScraperInspect } from '$lib/types/scraper';

type ScraperWS = typeof scraperWS;

export type RawSource = 'inspect' | 'live';

export type RawVm = {
	inspectJson: string;
	liveJson: string;
	displayJson: string;
};

function buildLiveSnapshot(name: string, ws: ScraperWS): Record<string, unknown> {
	return {
		gameData: ws.gameData[name] ?? null,
		latestTick: ws.ticks[name] ?? null,
		events: ws.events[name] ?? [],
		phase: ws.phases[name] ?? null,
		tickNumber: ws.tickNumbers[name] ?? null,
		previousGame: ws.previousGames[name] ?? null,
		lastReadAt: ws.lastReadAt[name] ?? null,
		gameDataAt: ws.gameDataAt[name] ?? null,
		ticksAt: ws.ticksAt[name] ?? null,
		lastEventsReplyAt: ws.lastEventsReplyAt[name] ?? null,
		lastEventsReplyPhase: ws.lastEventsReplyPhase[name] ?? null
	};
}

export function buildRawVm(
	name: string,
	source: RawSource,
	inspect: ScraperInspect | null,
	ws: ScraperWS
): RawVm {
	const inspectJson = inspect ? JSON.stringify(inspect, null, 2) : '';
	const liveJson = JSON.stringify(buildLiveSnapshot(name, ws), null, 2);
	return {
		inspectJson,
		liveJson,
		displayJson: source === 'inspect' ? inspectJson : liveJson
	};
}
