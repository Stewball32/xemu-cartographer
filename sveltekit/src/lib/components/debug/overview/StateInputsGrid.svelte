<script lang="ts">
	import type { StateInputs } from '$lib/types/scraper';
	import StatTile from '../shared/StatTile.svelte';

	let {
		stateInputs,
		showHeader = true
	}: {
		stateInputs: StateInputs | null;
		showHeader?: boolean;
	} = $props();

	type StatusKind = 'on' | 'off' | 'neutral' | 'pointer' | 'none';

	// Memory-pointer-shaped values are usually 32-bit Xbox VAs (0x80000000+)
	// or large unsigned ints; render them with the pointer accent so they
	// don't get confused with score-counter values that happen to be large.
	const POINTER_THRESHOLD = 65536;

	function interpret(v: unknown): { display: string; statusKind: StatusKind } {
		if (v === null || v === undefined) return { display: '—', statusKind: 'none' };
		if (typeof v === 'boolean') {
			return v ? { display: 'true', statusKind: 'on' } : { display: 'false', statusKind: 'off' };
		}
		if (typeof v === 'number') {
			if (!Number.isFinite(v)) return { display: String(v), statusKind: 'none' };
			if (v === 0) return { display: '0', statusKind: 'neutral' };
			if (Math.abs(v) >= POINTER_THRESHOLD) {
				return { display: `0x${v.toString(16)}`, statusKind: 'pointer' };
			}
			return { display: Number.isInteger(v) ? String(v) : v.toFixed(4), statusKind: 'on' };
		}
		if (typeof v === 'string') {
			return v.length === 0
				? { display: '—', statusKind: 'none' }
				: { display: v, statusKind: 'none' };
		}
		return { display: String(v), statusKind: 'none' };
	}

	const entries = $derived.by(() => {
		if (!stateInputs) return [] as Array<[string, unknown]>;
		return Object.entries(stateInputs).sort(([a], [b]) => a.localeCompare(b));
	});
</script>

<section>
	{#if showHeader}
		<div
			class="text-surface-700-200 mb-2 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
		>
			State inputs (diagnostic)
			<span class="text-surface-500-400 font-normal normal-case">
				plugin-reported scraper inputs
			</span>
		</div>
	{/if}
	{#if entries.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			no state inputs reported by plugin
		</div>
	{:else}
		<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
			{#each entries as [k, v] (k)}
				{@const r = interpret(v)}
				<StatTile
					label={k}
					display={r.display}
					statusKind={r.statusKind}
					title="{k} = {r.display}"
				/>
			{/each}
		</div>
	{/if}
</section>
