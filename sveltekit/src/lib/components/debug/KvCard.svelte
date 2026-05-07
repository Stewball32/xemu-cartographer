<script lang="ts">
	import JsonTree from '../JsonTree.svelte';
	import AnnotationPill from './AnnotationPill.svelte';

	let {
		title,
		value,
		emptyMessage = 'no data',
		annotationPrefix
	}: {
		title?: string;
		value: Record<string, unknown> | null | undefined;
		emptyMessage?: string;
		annotationPrefix?: string;
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
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			{title}
		</div>
	{/if}
	{#if !value || entries.length === 0}
		<div class="text-surface-500-400 text-sm">{emptyMessage}</div>
	{:else}
		<dl
			class="grid {annotationPrefix
				? 'grid-cols-[auto_max-content_1fr]'
				: 'grid-cols-[max-content_1fr]'} gap-x-4 gap-y-1 text-sm"
		>
			{#each entries as [k, v] (k)}
				{#if annotationPrefix}
					<AnnotationPill key="{annotationPrefix}.{k}" />
				{/if}
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
