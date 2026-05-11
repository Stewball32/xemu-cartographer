<script lang="ts" generics="T">
	import AnnotationPill from '$lib/components/debug/shared/AnnotationPill.svelte';
	import JsonTree from '$lib/components/ui/JsonTree.svelte';
	import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon } from '@lucide/svelte';
	import type { DataColumn, DataColumnGroup, Density, SortDir, SortState } from './data-table';

	let {
		rows,
		groups,
		stickyFirst = false,
		emptyMessage = 'no rows',
		rowKey,
		keyFor,
		density = 'compact',
		loading = false,
		loadingRows = 5,
		defaultSort,
		secondarySort,
		sort,
		onSortChange,
		onRowClick,
		class: extraClass = ''
	}: {
		rows: ReadonlyArray<T> | null | undefined;
		groups: DataColumnGroup<T>[];
		stickyFirst?: boolean;
		emptyMessage?: string;
		rowKey?: (row: T, idx: number) => string | number;
		keyFor?: (col: DataColumn<T>) => string | undefined;
		density?: Density;
		loading?: boolean;
		loadingRows?: number;
		defaultSort?: SortState;
		secondarySort?: { key: string; dir?: SortDir };
		sort?: SortState;
		onSortChange?: (s: SortState) => void;
		onRowClick?: (row: T) => void;
		class?: string;
	} = $props();

	// svelte-ignore state_referenced_locally
	let internalSort = $state<SortState>(defaultSort ?? null);

	const currentSort = $derived<SortState>(sort !== undefined ? sort : internalSort);

	const allCols = $derived(groups.flatMap((g) => g.columns));
	const showGroupHeader = $derived(groups.some((g) => g.label !== undefined && g.label !== ''));

	const padCls = $derived(density === 'comfortable' ? 'px-4 py-2' : 'px-2 py-1');
	const textSize = $derived(density === 'comfortable' ? 'text-sm' : 'text-xs');
	const cellNumeric = $derived(density === 'comfortable' ? '' : 'tabular-nums');

	function isScalar(v: unknown): boolean {
		return v === null || v === undefined || typeof v !== 'object';
	}

	function fmt(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'number') {
			if (Number.isInteger(v)) return String(v);
			return v.toFixed(4);
		}
		if (typeof v === 'boolean') return v ? '✓' : '·';
		return String(v);
	}

	function readCell(row: T, col: DataColumn<T>): unknown {
		if (col.format) return col.format(row);
		return (row as unknown as Record<string, unknown>)[col.key];
	}

	function isFirstInGroup(colIndex: number): boolean {
		let acc = 0;
		for (let g = 0; g < groups.length; g++) {
			if (acc === colIndex) return g > 0;
			acc += groups[g].columns.length;
		}
		return false;
	}

	function sortValue(col: DataColumn<T>, row: T): unknown {
		if (col.sortAccessor) return col.sortAccessor(row);
		if (col.format) return col.format(row);
		return (row as unknown as Record<string, unknown>)[col.key];
	}

	function defaultCompare(a: unknown, b: unknown): number {
		const an = a === null || a === undefined;
		const bn = b === null || b === undefined;
		if (an && bn) return 0;
		if (an) return 1;
		if (bn) return -1;
		if (typeof a === 'number' && typeof b === 'number') return a - b;
		if (typeof a === 'boolean' && typeof b === 'boolean') {
			return (a ? 1 : 0) - (b ? 1 : 0);
		}
		return String(a).localeCompare(String(b));
	}

	function compareRows(col: DataColumn<T>, a: T, b: T): number {
		if (col.comparator) return col.comparator(a, b);
		return defaultCompare(sortValue(col, a), sortValue(col, b));
	}

	function findCol(key: string): DataColumn<T> | undefined {
		return allCols.find((c) => c.key === key);
	}

	const sortedRows = $derived.by<ReadonlyArray<T>>(() => {
		if (!rows || rows.length === 0) return rows ?? [];
		const cur = currentSort;
		const sec = secondarySort;
		const primaryCol = cur ? findCol(cur.key) : undefined;
		const secondaryCol = sec && cur?.key !== sec.key ? findCol(sec.key) : undefined;
		if (!primaryCol && !secondaryCol) return rows;
		const indexed = rows.map((row, idx) => ({ row, idx }));
		indexed.sort((a, b) => {
			if (primaryCol && cur) {
				const cmp = compareRows(primaryCol, a.row, b.row);
				if (cmp !== 0) return cur.dir === 'asc' ? cmp : -cmp;
			}
			if (secondaryCol && sec) {
				const cmp = compareRows(secondaryCol, a.row, b.row);
				const dir = sec.dir ?? 'asc';
				if (cmp !== 0) return dir === 'asc' ? cmp : -cmp;
			}
			return a.idx - b.idx;
		});
		return indexed.map((x) => x.row);
	});

	function setSort(s: SortState) {
		if (sort === undefined) internalSort = s;
		onSortChange?.(s);
	}

	function cycleSort(col: DataColumn<T>) {
		const cur = currentSort;
		if (cur && cur.key === col.key) {
			if (cur.dir === 'asc') setSort({ key: col.key, dir: 'desc' });
			else setSort(null);
		} else {
			setSort({ key: col.key, dir: 'asc' });
		}
	}

	function ariaSortFor(col: DataColumn<T>): 'ascending' | 'descending' | 'none' | undefined {
		if (col.sortable === false) return undefined;
		const cur = currentSort;
		if (!cur || cur.key !== col.key) return 'none';
		return cur.dir === 'asc' ? 'ascending' : 'descending';
	}

	function handleRowKeydown(event: KeyboardEvent, row: T) {
		if (!onRowClick) return;
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			onRowClick(row);
		}
	}
</script>

{#if loading && (!rows || rows.length === 0)}
	<div class="-mx-3 overflow-x-auto sm:mx-0 {extraClass}">
		<table class="w-full {textSize}" aria-busy="true">
			<colgroup>
				{#each allCols as col (col.key)}
					<col class={col.width ?? ''} />
				{/each}
			</colgroup>
			<thead>
				<tr class="bg-surface-100-900">
					{#each allCols as col (col.key)}
						<th class="{padCls} text-left font-medium">{col.label ?? col.key}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each Array(loadingRows), ri (ri)}
					<tr class="border-t border-surface-200-800">
						{#each allCols as col (col.key)}
							<td class="{padCls} align-top">
								<div class="h-4 placeholder animate-pulse rounded"></div>
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else if !rows || rows.length === 0}
	<div class="text-surface-500-400 card preset-tonal p-3 text-sm {extraClass}">{emptyMessage}</div>
{:else}
	<div class="-mx-3 overflow-x-auto sm:mx-0 {extraClass}">
		<table class="w-full {textSize}">
			<colgroup>
				{#each allCols as col (col.key)}
					<col class={col.width ?? ''} />
				{/each}
			</colgroup>
			<thead>
				{#if showGroupHeader}
					<tr class="bg-surface-200-800">
						{#each groups as group, gi (gi)}
							<th
								colspan={group.columns.length}
								class="border-surface-200-800 {padCls} text-left font-semibold {gi > 0
									? 'border-l'
									: ''}"
							>
								{group.label ?? ''}
							</th>
						{/each}
					</tr>
				{/if}
				<tr class="bg-surface-100-900">
					{#each allCols as col, ci (col.key)}
						{@const annKey = keyFor?.(col)}
						{@const sortable = col.sortable !== false}
						{@const active = sortable && currentSort?.key === col.key}
						{@const dir = active ? currentSort!.dir : null}
						<th
							aria-sort={ariaSortFor(col)}
							class="border-surface-200-800 {padCls} text-left font-medium {showGroupHeader &&
							isFirstInGroup(ci)
								? 'border-l'
								: ''} {stickyFirst && ci === 0
								? 'sticky left-0 z-10 bg-surface-100-900'
								: ''} {col.align === 'right' ? 'text-right' : ''}"
						>
							{#if sortable}
								<button
									type="button"
									class="inline-flex w-full items-center gap-1 {col.align === 'right'
										? 'justify-end'
										: ''} hover:text-primary-600-300"
									onclick={() => cycleSort(col)}
								>
									{#if annKey}
										<AnnotationPill key={annKey} />
									{/if}
									<span>{col.label ?? col.key}</span>
									{#if active && dir === 'asc'}
										<ArrowUpIcon class="size-3" />
									{:else if active && dir === 'desc'}
										<ArrowDownIcon class="size-3" />
									{:else}
										<ArrowUpDownIcon class="text-surface-500-400 size-3 opacity-60" />
									{/if}
								</button>
							{:else}
								<span class="inline-flex items-center gap-1">
									{#if annKey}
										<AnnotationPill key={annKey} />
									{/if}
									{col.label ?? col.key}
								</span>
							{/if}
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each sortedRows as row, ri (rowKey ? rowKey(row, ri) : ri)}
					<tr
						class="border-t border-surface-200-800 {onRowClick
							? 'cursor-pointer hover:preset-tonal-primary'
							: ''}"
						role={onRowClick ? 'button' : undefined}
						tabindex={onRowClick ? 0 : undefined}
						onclick={onRowClick ? () => onRowClick(row) : null}
						onkeydown={onRowClick ? (e) => handleRowKeydown(e, row) : null}
					>
						{#each allCols as col, ci (col.key)}
							{@const v = readCell(row, col)}
							<td
								class="{padCls} align-top {cellNumeric} {stickyFirst && ci === 0
									? 'sticky left-0 bg-surface-50-950'
									: ''} {col.align === 'right' ? 'text-right' : ''}"
							>
								{#if col.cell}
									{@render col.cell({ row, value: v })}
								{:else if isScalar(v)}
									<span class="font-mono">{fmt(v)}</span>
								{:else}
									<JsonTree value={v} depth={2} />
								{/if}
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
