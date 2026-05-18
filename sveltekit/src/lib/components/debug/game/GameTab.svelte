<script lang="ts">
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import { useDebugContext } from '../context.js';
	import GameStatsHeader from './GameStatsHeader.svelte';
	import GamePretty from './GamePretty.svelte';
	import GameJson from './GameJson.svelte';

	let { name }: { name: string } = $props();

	// viewMode lives on the page (debug context) so the choice persists
	// across tabs that opt into the Pretty/JSON toggle. ctx fields are
	// optional so non-tabbed debug pages (probe) can skip wiring them up.
	const ctx = useDebugContext();
	const viewMode = $derived(ctx.viewMode ?? 'pretty');
	const setViewMode = ctx.setViewMode ?? (() => {});

	// Two store reads: payload for the Pretty view, full envelope for the
	// stats header + JSON view. Both reactive via the v2 store's getters.
	const payload = $derived(scraperWSV2.game[name] ?? null);
	const envelope = $derived(scraperWSV2.gameEnvelope[name] ?? null);
</script>

<div class="flex flex-col gap-4">
	<GameStatsHeader {envelope} {viewMode} onViewModeChange={setViewMode} />

	{#if viewMode === 'pretty'}
		<GamePretty {payload} />
	{:else}
		<GameJson {envelope} />
	{/if}
</div>
