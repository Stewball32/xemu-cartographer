<script lang="ts">
	import { untrack } from 'svelte';
	import { ChevronDownIcon, ChevronRightIcon } from '@lucide/svelte';
	import Self from './JsonTree.svelte';

	let {
		value,
		label,
		depth = 0,
		defaultOpen = true
	}: {
		value: unknown;
		label?: string;
		depth?: number;
		defaultOpen?: boolean;
	} = $props();

	// `defaultOpen` is the initial state only; subsequent changes are ignored.
	let open = $state(untrack(() => defaultOpen));

	function isObject(v: unknown): v is Record<string, unknown> {
		return typeof v === 'object' && v !== null && !Array.isArray(v);
	}

	function isHomogeneousObjectArray(arr: unknown[]): boolean {
		if (arr.length === 0) return false;
		if (!arr.every((e) => isObject(e))) return false;
		const first = arr[0] as Record<string, unknown>;
		const firstKeys = Object.keys(first).sort().join(',');
		return arr.every(
			(e) => Object.keys(e as Record<string, unknown>).sort().join(',') === firstKeys
		);
	}

	function looksLikeTagPath(s: string): boolean {
		return s.includes('\\') || (s.includes('/') && s.length > 12);
	}

	function isScalar(v: unknown): boolean {
		return v === null || v === undefined || typeof v !== 'object';
	}

	let kind = $derived.by(() => {
		if (value === null || value === undefined) return 'nullish';
		if (typeof value === 'boolean') return 'boolean';
		if (typeof value === 'number') return 'number';
		if (typeof value === 'string') return 'string';
		if (Array.isArray(value)) {
			if (value.length === 0) return 'empty-array';
			if (isHomogeneousObjectArray(value)) return 'object-array';
			return 'array';
		}
		if (isObject(value)) {
			if (Object.keys(value).length === 0) return 'empty-object';
			return 'object';
		}
		return 'unknown';
	});
</script>

{#if depth === 0 && (kind === 'object' || kind === 'object-array' || kind === 'array')}
	<!-- Top-level: collapsible section card -->
	<div class="card preset-tonal mb-4">
		<button
			type="button"
			class="flex w-full items-center justify-between px-4 py-3 text-left"
			onclick={() => (open = !open)}
		>
			<span class="font-semibold">{label ?? 'value'}</span>
			{#if open}
				<ChevronDownIcon class="size-4" />
			{:else}
				<ChevronRightIcon class="size-4" />
			{/if}
		</button>
		{#if open}
			<div class="border-surface-300-700 border-t px-4 py-3">
				<Self {value} {label} depth={1} />
			</div>
		{/if}
	</div>
{:else if kind === 'nullish'}
	<span class="text-surface-500-400 font-mono text-xs">null</span>
{:else if kind === 'boolean'}
	{#if value === true}
		<span class="badge preset-filled-success-500 text-xs">true</span>
	{:else}
		<span class="badge preset-filled-error-500 text-xs">false</span>
	{/if}
{:else if kind === 'number'}
	<span class="font-mono text-sm tabular-nums">{value}</span>
{:else if kind === 'string'}
	{#if looksLikeTagPath(value as string)}
		<span class="font-mono text-xs break-all">{value}</span>
	{:else}
		<span class="text-sm">{value}</span>
	{/if}
{:else if kind === 'empty-array'}
	<span class="text-surface-500-400 font-mono text-xs">[]</span>
{:else if kind === 'empty-object'}
	<span class="text-surface-500-400 font-mono text-xs">{'{}'}</span>
{:else if kind === 'object-array'}
	<!-- Homogeneous array of objects → table -->
	{@const arr = value as Record<string, unknown>[]}
	{@const cols = Object.keys(arr[0])}
	<div class="overflow-x-auto">
		<table class="table-hover w-full text-xs">
			<thead class="bg-surface-200-800">
				<tr>
					{#each cols as col}
						<th class="px-2 py-1 text-left font-medium">{col}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each arr as row}
					<tr class="border-surface-300-700 border-t">
						{#each cols as col}
							<td class="px-2 py-1 align-top">
								<Self value={row[col]} depth={depth + 1} />
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else if kind === 'array'}
	<!-- Heterogeneous array → ordered list -->
	{@const arr = value as unknown[]}
	<ol class="border-surface-300-700 ml-2 border-l pl-3">
		{#each arr as item, i}
			<li class="py-0.5">
				<span class="text-surface-500-400 mr-2 font-mono text-xs">[{i}]</span>
				<Self value={item} depth={depth + 1} />
			</li>
		{/each}
	</ol>
{:else if kind === 'object'}
	<!-- Object at depth>0 → key-value rows in indented border block -->
	{@const obj = value as Record<string, unknown>}
	<div
		class={depth === 1
			? 'space-y-1'
			: 'border-surface-300-700 ml-2 space-y-1 border-l pl-3'}
	>
		{#each Object.entries(obj) as [k, v]}
			<div class="flex flex-wrap items-baseline gap-2">
				<span class="text-surface-700-200 font-mono text-xs">{k}:</span>
				{#if isScalar(v)}
					<Self value={v} depth={depth + 1} />
				{:else}
					<div class="w-full pl-2">
						<Self value={v} depth={depth + 1} />
					</div>
				{/if}
			</div>
		{/each}
	</div>
{/if}
