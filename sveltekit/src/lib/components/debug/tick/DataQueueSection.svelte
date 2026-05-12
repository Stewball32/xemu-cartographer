<script lang="ts">
	// Renders TickPayload.data_queue — five scalar fields the engine updates
	// every tick. Rendered as a KvCard so all fields appear with their wire
	// names regardless of value.
	import KvCard from '../shared/KvCard.svelte';
	import type { TickDataQueue } from '$lib/types/scraper';

	let {
		dataQueue,
		showHeader = true
	}: {
		dataQueue: TickDataQueue | null | undefined;
		showHeader?: boolean;
	} = $props();

	function hex(n: number | undefined): string {
		if (n === undefined) return '—';
		const u = n >>> 0;
		return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
	}

	const value = $derived.by(() => {
		if (!dataQueue) return null;
		return {
			tick: dataQueue.tick,
			global_random: hex(dataQueue.global_random),
			tick2: dataQueue.tick2,
			unk1: dataQueue.unk1,
			player_count: dataQueue.player_count
		};
	});
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Data queue
		</div>
	{/if}
	<KvCard {value} emptyMessage="no data_queue in this tick" entriesSort="none" />
</section>
