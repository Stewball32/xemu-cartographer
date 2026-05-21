<script lang="ts" module>
	export type KvField = {
		key: string;
		value: unknown;
		annotationKey?: string;
		format?: (v: unknown) => string;
		span?: 1 | 2;
	};
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import AnnotationPill from './AnnotationPill.svelte';

	let {
		fields,
		emptyMessage = 'no data',
		nested,
		class: extraClass = ''
	}: {
		fields: KvField[] | null | undefined;
		emptyMessage?: string;
		nested?: Snippet<[{ field: KvField }]>;
		class?: string;
	} = $props();

	function isScalar(v: unknown): boolean {
		return v === null || v === undefined || typeof v !== 'object';
	}

	function defaultFmt(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'number') return Number.isInteger(v) ? String(v) : v.toFixed(4);
		if (typeof v === 'boolean') return v ? 'true' : 'false';
		return String(v);
	}
</script>

{#if !fields || fields.length === 0}
	<div class="text-surface-500-400 text-sm {extraClass}">{emptyMessage}</div>
{:else}
	<dl class="grid grid-cols-1 gap-x-4 gap-y-1 text-sm sm:grid-cols-[max-content_1fr] {extraClass}">
		{#each fields as field (field.key)}
			<dt class="text-surface-700-200 flex items-center gap-1.5 font-mono text-xs">
				{#if field.annotationKey}
					<AnnotationPill key={field.annotationKey} />
				{/if}
				{field.key}
			</dt>
			<dd>
				{#if isScalar(field.value)}
					<span class="font-mono tabular-nums">
						{(field.format ?? defaultFmt)(field.value)}
					</span>
				{:else if nested}
					{@render nested({ field })}
				{:else}
					<span class="text-surface-500-400 font-mono text-xs">[object]</span>
				{/if}
			</dd>
		{/each}
	</dl>
{/if}
