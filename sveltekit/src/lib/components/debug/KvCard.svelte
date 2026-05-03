<script lang="ts">
	import JsonTree from '../JsonTree.svelte';

	let {
		title,
		value,
		emptyMessage = 'no data'
	}: {
		title?: string;
		value: Record<string, unknown> | null | undefined;
		emptyMessage?: string;
	} = $props();

	function isScalar(v: unknown): boolean {
		return v === null || v === undefined || typeof v !== 'object';
	}

	function fmt(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'number') return Number.isInteger(v) ? String(v) : v.toFixed(4);
		if (typeof v === 'boolean') return v ? 'true' : 'false';
		return String(v);
	}

	const entries = $derived(value ? Object.entries(value) : []);
</script>

<div class="card preset-tonal p-3">
	{#if title}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold uppercase tracking-wide">
			{title}
		</div>
	{/if}
	{#if !value || entries.length === 0}
		<div class="text-surface-500-400 text-sm">{emptyMessage}</div>
	{:else}
		<dl class="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
			{#each entries as [k, v]}
				<dt class="text-surface-700-200 font-mono text-xs">{k}</dt>
				<dd>
					{#if isScalar(v)}
						<span class="font-mono tabular-nums">{fmt(v)}</span>
					{:else}
						<JsonTree value={v} depth={1} />
					{/if}
				</dd>
			{/each}
		</dl>
	{/if}
</div>
