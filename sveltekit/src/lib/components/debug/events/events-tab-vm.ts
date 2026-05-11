// View-model for the Events tab: shapes scraperWS.events[name] into the
// frequency-strip buckets and the type-filtered feed. Plain TS so callers
// establish reactivity via $derived.by().

import type { Envelope } from '$lib/types/scraper';
import { eventTypeOf } from '../shared/events-vm';
import type { scraperWS } from '$lib/stores/scraper-ws.svelte';

type ScraperWS = typeof scraperWS;

export type EventTypeBucket = {
	type: string;
	count: number;
};

export type EventsTabVm = {
	filteredEvents: Envelope[];
	typeBuckets: EventTypeBucket[];
	totalCount: number;
	filterActive: boolean;
};

// Fall back to env.type so payload-less events still bucket somewhere
// rather than disappearing from the strip / feed.
function typeKey(env: Envelope): string {
	return eventTypeOf(env) ?? env.type;
}

export function buildEventsTabVm(
	name: string,
	ws: ScraperWS,
	selectedTypes: ReadonlySet<string>
): EventsTabVm {
	const events = ws.events[name] ?? [];

	const counts: Record<string, number> = {};
	for (const env of events) {
		const t = typeKey(env);
		counts[t] = (counts[t] ?? 0) + 1;
	}

	// Sort by count desc; tie-break alphabetically so the strip stays
	// stable when counts collide.
	const typeBuckets: EventTypeBucket[] = Object.keys(counts)
		.map((t) => ({ type: t, count: counts[t] }))
		.sort((a, b) => b.count - a.count || a.type.localeCompare(b.type));

	const filterActive = selectedTypes.size > 0;
	const filteredEvents = filterActive
		? events.filter((env) => selectedTypes.has(typeKey(env)))
		: events;

	return {
		filteredEvents,
		typeBuckets,
		totalCount: events.length,
		filterActive
	};
}
