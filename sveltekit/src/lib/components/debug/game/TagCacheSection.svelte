<script lang="ts">
	// Renders GameData.tag_cache — the four cache base/size pairs (game state,
	// tag cache, texture cache, sound cache). Useful for verifying the scraper
	// resolved the right base pointers without reaching for the raw JSON.
	import KvCard from '../shared/KvCard.svelte';
	import type { StaticCachePtrs } from '$lib/types/scraper';

	let {
		tagCache,
		showHeader = true
	}: {
		tagCache: StaticCachePtrs | null;
		showHeader?: boolean;
	} = $props();

	function hex(n: number | undefined): string {
		if (n === undefined) return '—';
		// Cache base pointers are 32-bit GVAs — force unsigned width so the
		// high-bit-set kernel addresses render as 0x80xxxxxx, not negative.
		const u = n >>> 0;
		return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
	}

	function bytes(n: number | undefined): string {
		if (n === undefined) return '—';
		return `${n.toLocaleString()} B`;
	}

	const value = $derived.by(() => {
		if (!tagCache) return null;
		return {
			game_state_base: hex(tagCache.game_state_base),
			game_state_size: bytes(tagCache.game_state_size),
			tag_cache_base: hex(tagCache.tag_cache_base),
			tag_cache_size: bytes(tagCache.tag_cache_size),
			texture_cache_base: hex(tagCache.texture_cache_base),
			texture_cache_size: bytes(tagCache.texture_cache_size),
			sound_cache_base: hex(tagCache.sound_cache_base),
			sound_cache_size: bytes(tagCache.sound_cache_size)
		};
	});
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Tag cache
		</div>
	{/if}
	<KvCard {value} emptyMessage="no tag cache pointers in current match data" entriesSort="none" />
</section>
