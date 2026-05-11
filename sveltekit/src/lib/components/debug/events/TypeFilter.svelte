<script lang="ts">
	// Multi-select Toggle Group bound to a parent-owned SvelteSet so the
	// reactive identity survives across selection changes (the parent's
	// $derived re-runs when set membership mutates, not when the reference
	// swaps). Mirrors OverviewTab's collapsedSections pattern.

	import type { SvelteSet } from 'svelte/reactivity';
	import { ToggleGroup } from '@skeletonlabs/skeleton-svelte';
	import type { EventTypeBucket } from './events-tab-vm';

	let {
		typeBuckets,
		selectedTypes
	}: {
		typeBuckets: EventTypeBucket[];
		selectedTypes: SvelteSet<string>;
	} = $props();

	const selectedArray = $derived([...selectedTypes]);

	function onValueChange(next: string[]) {
		selectedTypes.clear();
		for (const t of next) selectedTypes.add(t);
	}
</script>

<div class="flex flex-wrap items-center gap-2">
	<ToggleGroup
		value={selectedArray}
		onValueChange={(d) => onValueChange(d.value)}
		multiple
		aria-label="Filter events by type"
	>
		{#each typeBuckets as b (b.type)}
			<ToggleGroup.Item value={b.type}>
				<span class="font-mono text-xs">{b.type}</span>
				<span class="ml-1 text-xs tabular-nums opacity-70">({b.count})</span>
			</ToggleGroup.Item>
		{/each}
	</ToggleGroup>
	{#if selectedTypes.size > 0}
		<button
			type="button"
			class="btn preset-tonal btn-sm"
			onclick={() => selectedTypes.clear()}
			title="Clear filter — show all event types"
		>
			Clear filter
		</button>
	{/if}
</div>
