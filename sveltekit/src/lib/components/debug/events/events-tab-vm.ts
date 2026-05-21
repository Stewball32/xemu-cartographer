// View-model for the Events tab. Sources scraperWSV2.events[name] (a
// rolling list of AnyEvent payloads in newest-first order) and projects
// each event into the v1 Envelope shape EventFeed + EventTile already
// consume. Bucket counts dispatch on the v2 event_type; when the v1
// section migrates fully the wrapping disappears.
//
// Plain TS so callers establish reactivity via $derived.by().

import type { AnyEvent } from '$lib/types/scraper-v2';
import type { Envelope } from '$lib/types/scraper';
import { eventTypeOf } from '../shared/events-vm';
import type { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';

type ScraperWSV2 = typeof scraperWSV2;

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

// Wrap a v2 AnyEvent in the v1 Envelope shape so EventFeed/EventTile
// keep working without churning their internals. The outer envelope
// carries the instance + tick the v1 EventTile reads; the inner v2
// payload sits in `payload` (where the v1 helpers expect it).
function v2ToV1Envelope(ev: AnyEvent, instance: string): Envelope {
	return {
		v: 2,
		// v1 EnvelopeType doesn't include "event" — it does, see scraper.ts.
		type: 'event',
		instance,
		tick: ev.tick,
		payload: ev as unknown as Record<string, unknown>
	};
}

// Fall back to env.type so payload-less events still bucket somewhere
// rather than disappearing from the strip / feed.
function typeKey(env: Envelope): string {
	return eventTypeOf(env) ?? env.type;
}

export function buildEventsTabVm(
	name: string,
	ws: ScraperWSV2,
	selectedTypes: ReadonlySet<string>
): EventsTabVm {
	const v2Events = ws.events[name] ?? [];
	const events: Envelope[] = v2Events.map((ev) => v2ToV1Envelope(ev, name));

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
