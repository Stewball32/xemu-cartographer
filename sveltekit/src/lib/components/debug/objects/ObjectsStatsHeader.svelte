<script lang="ts">
	import {
		BracesIcon,
		ClockIcon,
		HashIcon,
		InboxIcon,
		LayersIcon,
		LayoutGridIcon,
		TagIcon,
		ZapIcon
	} from '@lucide/svelte';
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import type { EnvelopeV2, ObjectsPayload } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import { useDebugContext } from '../context.js';

	type ViewMode = 'pretty' | 'json';

	let {
		envelope,
		viewMode,
		onViewModeChange
	}: {
		envelope: EnvelopeV2<ObjectsPayload> | null;
		viewMode: ViewMode;
		onViewModeChange: (next: ViewMode) => void;
	} = $props();

	const ctx = useDebugContext();

	// Re-derives every tick of the page's `now` interval (1 s) because
	// ctx.relativeTime reads ctx.now under the hood.
	const receivedDisplay = $derived(envelope?.ts ? ctx.relativeTime(Date.parse(envelope.ts)) : '—');
	const receivedRaw = $derived(envelope?.ts ?? undefined);
</script>

{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}
{#snippet zapIcon()}<ZapIcon class="size-3.5" />{/snippet}
{#snippet clockIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet layersIcon()}<LayersIcon class="size-3.5" />{/snippet}
{#snippet tagIcon()}<TagIcon class="size-3.5" />{/snippet}

<div class="flex flex-col gap-2">
	<div class="flex items-start justify-between gap-2">
		<div
			class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
		>
			<InboxIcon class="size-3.5" />
			objects envelope
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
			label="seq"
			display={envelope ? String(envelope.seq) : '—'}
			statusKind={envelope ? 'on' : 'none'}
			title="envelope sequence number"
			icon={hashIcon}
		/>
		<StatTile
			label="tick"
			display={envelope ? String(envelope.tick) : '—'}
			statusKind={envelope ? 'on' : 'none'}
			title="engine tick at emit time"
			icon={zapIcon}
		/>
		<StatTile
			label="received"
			display={receivedDisplay}
			statusKind={envelope ? 'on' : 'none'}
			title={receivedRaw ?? 'no envelope received yet'}
			icon={clockIcon}
		/>
		<StatTile
			label="v"
			display={envelope ? String(envelope.v) : '—'}
			statusKind={envelope ? 'on' : 'none'}
			title="protocol version"
			icon={layersIcon}
		/>
		<StatTile
			label="instance"
			display={envelope?.instance ?? '—'}
			statusKind={envelope ? 'on' : 'none'}
			title={envelope?.instance ?? 'no envelope received yet'}
			icon={tagIcon}
		/>
	</div>

	<!-- min-h-5 reserves the line height so the layout doesn't shift when the
		first envelope arrives and the note disappears. -->
	<div class="text-surface-500-400 min-h-5 text-xs">
		{#if !envelope}no objects envelope received yet{/if}
	</div>
</div>
