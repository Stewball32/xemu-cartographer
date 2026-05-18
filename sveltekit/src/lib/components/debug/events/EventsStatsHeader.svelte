<script lang="ts">
	import {
		BracesIcon,
		ClockIcon,
		HashIcon,
		InboxIcon,
		LayoutGridIcon,
		ListOrderedIcon,
		TagIcon,
		ZapIcon
	} from '@lucide/svelte';
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import type { AnyEvent } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import { useDebugContext } from '../context.js';

	type ViewMode = 'pretty' | 'json';

	// Source data: the entire rolling log so we can surface both the *latest*
	// event's identity (seq/tick/event_type) and the log's overall size.
	let {
		events,
		instance,
		viewMode,
		onViewModeChange
	}: {
		events: AnyEvent[];
		instance: string;
		viewMode: ViewMode;
		onViewModeChange: (next: ViewMode) => void;
	} = $props();

	const ctx = useDebugContext();

	// events[] is newest-first; the latest envelope is index 0. Tiles all
	// render '—' when the log is empty (no event envelopes received yet).
	const latest = $derived(events[0] ?? null);

	// Re-derives every tick of the page's `now` interval (1 s) because
	// ctx.relativeTime reads ctx.now under the hood.
	const receivedDisplay = $derived(latest?.at ? ctx.relativeTime(Date.parse(latest.at)) : '—');
	const receivedRaw = $derived(latest?.at ?? undefined);
</script>

{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}
{#snippet zapIcon()}<ZapIcon class="size-3.5" />{/snippet}
{#snippet clockIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet tagIcon()}<TagIcon class="size-3.5" />{/snippet}
{#snippet countIcon()}<ListOrderedIcon class="size-3.5" />{/snippet}

<div class="flex flex-col gap-2">
	<div class="flex items-start justify-between gap-2">
		<div
			class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
		>
			<InboxIcon class="size-3.5" />
			events log
		</div>
		<Switch
			checked={viewMode === 'json'}
			onCheckedChange={(d) => onViewModeChange(d.checked ? 'json' : 'pretty')}
			aria-label="Toggle pretty / JSON view"
			title="Toggle pretty / JSON view"
		>
			<Switch.Label class="text-xs font-semibold tracking-wide uppercase">
				{viewMode === 'json' ? 'JSON' : 'Pretty'}
			</Switch.Label>
			<Switch.Control
				class="inline-flex cursor-pointer items-center rounded-full bg-surface-300-700 transition-colors data-[state=checked]:bg-primary-500"
			>
				<Switch.Thumb
					class="flex size-5 items-center justify-center rounded-full bg-white shadow-sm transition-transform"
				>
					{#if viewMode === 'json'}
						<BracesIcon class="size-3 text-primary-700" />
					{:else}
						<LayoutGridIcon class="size-3 text-secondary-700" />
					{/if}
				</Switch.Thumb>
			</Switch.Control>
			<Switch.HiddenInput />
		</Switch>
	</div>

	<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-5">
		<StatTile
			label="latest seq"
			display={latest ? String(latest.seq) : '—'}
			statusKind={latest ? 'on' : 'none'}
			title="sequence number of the most recent event envelope"
			icon={hashIcon}
		/>
		<StatTile
			label="latest tick"
			display={latest ? String(latest.tick) : '—'}
			statusKind={latest ? 'on' : 'none'}
			title="engine tick at the most recent event"
			icon={zapIcon}
		/>
		<StatTile
			label="received"
			display={receivedDisplay}
			statusKind={latest ? 'on' : 'none'}
			title={receivedRaw ?? 'no event envelopes received yet'}
			icon={clockIcon}
		/>
		<StatTile
			label="event_type"
			display={latest?.event_type ?? '—'}
			statusKind={latest ? 'on' : 'none'}
			title={latest?.event_type ?? 'no event envelopes received yet'}
			icon={tagIcon}
		/>
		<StatTile
			label="logged"
			display={String(events.length)}
			statusKind={events.length > 0 ? 'on' : 'none'}
			title={`${events.length} event envelope(s) buffered for ${instance}`}
			icon={countIcon}
		/>
	</div>

	<!-- min-h-5 reserves the line height so the layout doesn't shift when the
		first envelope arrives and the note disappears. -->
	<div class="text-surface-500-400 min-h-5 text-xs">
		{#if !latest}no event envelopes received yet{/if}
	</div>
</div>
