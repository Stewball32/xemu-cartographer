<script lang="ts">
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import { useDebugContext } from '../context.js';
	import DebugStatsHeader from './DebugStatsHeader.svelte';
	import DebugPretty from './DebugPretty.svelte';
	import JsonView from '../shared/JsonView.svelte';
	import { V2_ENVELOPE_SHAPE } from '../shared/json-scope';

	let { name }: { name: string } = $props();

	// viewMode lives on the page (debug context) so the choice persists
	// across tabs that opt into the Pretty/JSON toggle. ctx fields are
	// optional so non-tabbed debug pages (probe) can skip wiring them up.
	const ctx = useDebugContext();
	const viewMode = $derived(ctx.viewMode ?? 'pretty');
	const setViewMode = ctx.setViewMode ?? (() => {});

	// Two store reads: payload for the Pretty view, full envelope for the
	// stats header + JSON view. Both reactive via the v2 store's getters.
	const payload = $derived(scraperWSV2.debug[name] ?? null);
	const envelope = $derived(scraperWSV2.debugEnvelope[name] ?? null);
</script>

<div class="flex flex-col gap-4">
	<DebugStatsHeader {envelope} {viewMode} onViewModeChange={setViewMode} />

	{#if viewMode === 'pretty'}
		<DebugPretty {payload} />
	{:else}
		<JsonView {envelope} shape={V2_ENVELOPE_SHAPE} />
	{/if}
</div>
