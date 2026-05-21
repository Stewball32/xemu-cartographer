<script lang="ts">
	// Interactive chip strip — doubles as the "by type" frequency display and
	// the multi-select filter. Each chip shows the event type + count. With no
	// selection every chip looks filled (showing all); selecting some dims the
	// non-selected chips. Bound to a parent-owned SvelteSet so the reactive
	// identity survives across selection changes (the parent's $derived re-runs
	// on set membership mutations, not on reference swaps).

	import type { SvelteSet } from 'svelte/reactivity';
	import type { EventTypeBucket } from './events-tab-vm';

	let {
		typeBuckets,
		selectedTypes
	}: {
		typeBuckets: EventTypeBucket[];
		selectedTypes: SvelteSet<string>;
	} = $props();

	function toggle(t: string) {
		if (selectedTypes.has(t)) selectedTypes.delete(t);
		else selectedTypes.add(t);
	}
</script>

<div class="flex flex-wrap items-center gap-2" aria-label="Filter events by type">
	{#each typeBuckets as b (b.type)}
		{@const isSelected = selectedTypes.has(b.type)}
		{@const isActive = selectedTypes.size === 0 || isSelected}
		<button
			type="button"
			class="chip text-xs transition-opacity {isActive
				? 'preset-filled'
				: 'preset-tonal opacity-60 hover:opacity-100'}"
			aria-pressed={isSelected}
			title={isSelected
				? `${b.type}: ${b.count} — click to remove from filter`
				: selectedTypes.size === 0
					? `${b.type}: ${b.count} — click to filter to just this type`
					: `${b.type}: ${b.count} — click to add to filter`}
			onclick={() => toggle(b.type)}
		>
			<span class="font-mono">{b.type}</span>
			<span class="ml-1 tabular-nums opacity-80">{b.count}</span>
		</button>
	{/each}
	{#if selectedTypes.size > 0}
		<button
			type="button"
			class="btn preset-tonal btn-sm"
			onclick={() => selectedTypes.clear()}
			title="Clear filter — show all event types"
		>
			Clear
		</button>
	{/if}
</div>
