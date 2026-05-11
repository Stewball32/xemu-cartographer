<script lang="ts">
	// Reusable mini-card stat tile. The simple form (label + display + statusKind)
	// matches the original MiniStatTile contract; richer call sites can opt into
	// an icon, secondary line, trend chip, footer, click/href, a custom value
	// snippet, or a multi-row body. Wraps ui/Card so card styling stays in one
	// place.

	import type { Snippet } from 'svelte';
	import { ArrowDownIcon, ArrowRightIcon, ArrowUpIcon } from '@lucide/svelte';
	import Card from '$lib/components/ui/Card.svelte';

	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';
	type TrendDirection = 'up' | 'down' | 'flat';

	// One row in the multi-row body. Pick statusIcon when you want a Lucide
	// glyph instead of the colored dot — both are tinted by statusKind.
	export type StatRow = {
		label?: string;
		display: string;
		statusKind?: StatusKind;
		statusIcon?: Snippet;
		title?: string;
	};

	let {
		label,
		display,
		statusKind = 'none',
		title,
		icon,
		statusIcon,
		subtext,
		trend,
		href,
		onclick,
		footer,
		value,
		rows
	}: {
		label: string;
		display?: string;
		statusKind?: StatusKind;
		title?: string;
		icon?: Snippet;
		statusIcon?: Snippet;
		subtext?: string;
		trend?: { delta: string; direction: TrendDirection };
		href?: string;
		onclick?: () => void;
		footer?: Snippet | string;
		value?: Snippet;
		rows?: StatRow[];
	} = $props();

	// statusKind → dot-bg / value-text / status-icon-text classes. Kept as a
	// single helper so per-row rendering can reuse the same mapping.
	function dotClassFor(k: StatusKind | undefined): string {
		switch (k) {
			case 'on':
				return 'bg-success-500';
			case 'off':
				return 'bg-error-500';
			case 'warn':
				return 'bg-warning-500';
			case 'info':
				return 'bg-secondary-500';
			case 'neutral':
				return 'bg-surface-400-600';
			case 'pointer':
				return 'bg-tertiary-500';
			default:
				return '';
		}
	}

	function valueColorFor(k: StatusKind | undefined): string {
		switch (k) {
			case 'on':
				return 'text-success-500';
			case 'off':
				return 'text-error-500';
			case 'warn':
				return 'text-warning-500';
			case 'info':
				return 'text-secondary-500';
			case 'neutral':
				return 'text-surface-500-400';
			default:
				return 'text-surface-950-50';
		}
	}

	function iconColorFor(k: StatusKind | undefined): string {
		switch (k) {
			case 'on':
				return 'text-success-500';
			case 'off':
				return 'text-error-500';
			case 'warn':
				return 'text-warning-500';
			case 'info':
				return 'text-secondary-500';
			case 'neutral':
				return 'text-surface-500-400';
			case 'pointer':
				return 'text-tertiary-500';
			default:
				return 'text-surface-700-200';
		}
	}

	const dotClass = $derived(dotClassFor(statusKind));
	const valueColor = $derived(valueColorFor(statusKind));
	const statusIconColor = $derived(iconColorFor(statusKind));

	const trendColor = $derived(
		trend?.direction === 'up'
			? 'text-success-500'
			: trend?.direction === 'down'
				? 'text-error-500'
				: 'text-surface-500-400'
	);

	const interactive = $derived(href !== undefined || onclick !== undefined);
	const interactiveClass = $derived(
		interactive
			? 'cursor-pointer transition hover:bg-surface-200-800 focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:outline-none'
			: ''
	);
	const isFooterString = $derived(typeof footer === 'string');
	const hasRows = $derived(rows !== undefined && rows.length > 0);
</script>

{#snippet statusGlyph(kind: StatusKind | undefined, glyph: Snippet | undefined)}
	{#if glyph}
		<span class="flex size-3.5 shrink-0 items-center justify-center {iconColorFor(kind)}">
			{@render glyph()}
		</span>
	{:else}
		{@const cls = dotClassFor(kind)}
		{#if cls}
			<span class="block size-2 shrink-0 rounded-full {cls}"></span>
		{/if}
	{/if}
{/snippet}

{#snippet body()}
	<div class="flex min-w-0 items-center gap-1.5">
		{#if icon}
			<span class="text-surface-700-200 flex shrink-0 items-center">
				{@render icon()}
			</span>
		{/if}
		<span
			class="text-surface-700-200 min-w-0 flex-1 truncate text-[10px] font-semibold tracking-wide uppercase"
		>
			{label}
		</span>
		{#if trend}
			<span class="flex shrink-0 items-center gap-0.5 text-[10px] font-semibold {trendColor}">
				{#if trend.direction === 'up'}
					<ArrowUpIcon class="size-3" />
				{:else if trend.direction === 'down'}
					<ArrowDownIcon class="size-3" />
				{:else}
					<ArrowRightIcon class="size-3" />
				{/if}
				{trend.delta}
			</span>
		{/if}
	</div>
	{#if hasRows}
		<div class="mt-1 flex flex-col gap-0.5">
			{#each rows ?? [] as row, i (i)}
				<div class="flex min-w-0 items-center gap-1.5" title={row.title}>
					{@render statusGlyph(row.statusKind, row.statusIcon)}
					{#if row.label}
						<span
							class="text-surface-700-200 shrink-0 text-[10px] font-semibold tracking-wide uppercase"
						>
							{row.label}
						</span>
					{/if}
					<span
						class="min-w-0 flex-1 truncate font-mono text-sm font-bold tabular-nums {valueColorFor(
							row.statusKind
						)}"
					>
						{row.display}
					</span>
				</div>
			{/each}
		</div>
	{:else}
		<div class="mt-1 flex min-w-0 items-center gap-1.5">
			{#if statusIcon}
				<span class="flex size-3.5 shrink-0 items-center justify-center {statusIconColor}">
					{@render statusIcon()}
				</span>
			{:else if dotClass}
				<span class="block size-2 shrink-0 rounded-full {dotClass}"></span>
			{/if}
			<span class="min-w-0 flex-1 truncate font-mono text-sm font-bold tabular-nums {valueColor}">
				{#if value}
					{@render value()}
				{:else}
					{display ?? '—'}
				{/if}
			</span>
		</div>
	{/if}
	{#if subtext}
		<div class="text-surface-500-400 mt-0.5 truncate text-[11px]">
			{subtext}
		</div>
	{/if}
	{#if footer}
		<div class="mt-1.5 border-t border-surface-300-700 pt-1.5">
			{#if isFooterString}
				<div class="text-surface-600-300 truncate text-[11px]">{footer}</div>
			{:else if typeof footer !== 'string'}
				{@render footer()}
			{/if}
		</div>
	{/if}
{/snippet}

{#if href}
	<!-- StatTile accepts arbitrary hrefs (hash anchors, external URLs, resolved
		routes); it's the caller's job to pass a route through resolve() when
		needed. Suppressing the SvelteKit no-navigation-without-resolve rule
		here so the prop can stay free-form. -->
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
	<a {href} {title} class="block h-full min-w-0 {interactiveClass} rounded-container">
		<Card size="xs" tone="tonal" class="flex h-full flex-col">
			{@render body()}
		</Card>
	</a>
{:else if onclick}
	<button
		type="button"
		{onclick}
		{title}
		class="block h-full w-full min-w-0 text-left {interactiveClass} rounded-container"
	>
		<Card size="xs" tone="tonal" class="flex h-full flex-col">
			{@render body()}
		</Card>
	</button>
{:else}
	<div {title} class="h-full min-w-0">
		<Card size="xs" tone="tonal" class="flex h-full flex-col">
			{@render body()}
		</Card>
	</div>
{/if}
