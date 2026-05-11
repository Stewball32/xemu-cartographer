<script lang="ts">
	import type { EventTypeBucket } from './events-tab-vm';

	let { typeBuckets }: { typeBuckets: EventTypeBucket[] } = $props();
</script>

{#if typeBuckets.length === 0}
	<div class="text-surface-500-400 text-xs">no events to summarise</div>
{:else}
	<div class="flex flex-wrap gap-2" aria-label="Event type frequency">
		{#each typeBuckets as b (b.type)}
			<span class="chip preset-filled text-xs" title="{b.type}: {b.count}">
				<span class="font-mono">{b.type}</span>
				<span class="ml-1 tabular-nums opacity-80">{b.count}</span>
			</span>
		{/each}
	</div>
{/if}
