<script lang="ts">
	// game_globals: engine-wide flags read every tick.
	// Fields: map_loaded, active, players_are_double_speed,
	// game_loading_in_progress, precache_map_status, game_difficulty_level,
	// stored_global_random.

	import type { TickGameGlobals } from '$lib/types/scraper';
	import KvGrid, { type KvField } from '../shared/KvGrid.svelte';

	let {
		gameGlobals,
		annotationPrefix = 'tick.game_globals',
		showHeader = true
	}: {
		gameGlobals: TickGameGlobals | null | undefined;
		annotationPrefix?: string;
		showHeader?: boolean;
	} = $props();

	// game_globals booleans are int-encoded on the wire (0 / 1). Render as
	// ✓/· so flag-asserted-this-tick is scannable; leave non-flag ints alone.
	function fmtFlagOrInt(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'number') {
			if (v === 0) return '·';
			if (v === 1) return '✓';
			return String(v);
		}
		if (typeof v === 'boolean') return v ? '✓' : '·';
		return String(v);
	}

	const fields = $derived.by<KvField[]>(() => {
		if (!gameGlobals) return [];
		return Object.entries(gameGlobals).map(([k, v]) => ({
			key: k,
			value: v,
			annotationKey: `${annotationPrefix}.${k}`,
			format: fmtFlagOrInt
		}));
	});
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Engine flags
		</div>
	{/if}
	<div class="card preset-tonal p-3">
		<KvGrid {fields} emptyMessage="no game_globals on current tick" />
	</div>
</section>
