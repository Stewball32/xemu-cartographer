<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { ActivityIcon } from '@lucide/svelte';
	import { adminGet, AdminFetchError } from '$lib/utils/admin-api';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { toaster } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import type { ContainerInfo, ContainerDetail, InstanceState } from '$lib/types/containers';
	import type { ScraperInfo } from '$lib/types/scraper';

	type Source = 'container' | 'orphan';
	type Row = {
		name: string;
		source: Source;
		running: boolean;
		phase: string | null;
		title: string;
		score_summary: string | null;
		xbox_name: string;
		sock?: string;
	};

	const phasePriority: Record<string, number> = { live: 0, ready: 1, idle: 2 };
	function phaseRank(p: string | null): number {
		if (!p) return 99;
		return p in phasePriority ? phasePriority[p] : 50;
	}

	let containers = $state<ContainerInfo[]>([]);
	let scrapers = $state<Record<string, InstanceState | null>>({});
	let orphans = $state<ScraperInfo[]>([]);
	let loading = $state(true);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	function describeError(err: unknown): string {
		if (err instanceof AdminFetchError) return err.message;
		if (err instanceof Error) return err.message;
		return String(err);
	}

	async function loadAll() {
		try {
			loading = true;
			const list = await adminGet<ContainerInfo[] | null>('containers');
			containers = list ?? [];
			await refreshAll();
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeError(err) });
		} finally {
			loading = false;
		}
	}

	async function refreshOne(name: string) {
		try {
			const res = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
			scrapers = { ...scrapers, [name]: res.scraper };
		} catch (err) {
			console.warn('detail fetch failed for', name, err);
		}
	}

	async function refreshOrphans() {
		try {
			const list = await adminGet<ScraperInfo[] | null>('scraper');
			const knownNames = new Set(containers.map((c) => c.name));
			orphans = (list ?? []).filter((s) => !knownNames.has(s.name));
		} catch (err) {
			console.warn('scraper list fetch failed', err);
		}
	}

	async function refreshAll() {
		await Promise.all([...containers.map((c) => refreshOne(c.name)), refreshOrphans()]);
	}

	function startPolling() {
		stopPolling();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			refreshAll();
		}, 3000);
	}

	function stopPolling() {
		if (pollTimer !== null) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	onMount(() => {
		loadAll();
		startPolling();
		// Connecting here gives every row live phase + score from the
		// host:all aggregator (rendered alongside HTTP-derived state below).
		// The store's onopen joins host:all itself.
		if (auth.token) scraperWS.connect(auth.token);
	});

	onDestroy(() => {
		stopPolling();
		scraperWS.disconnect();
	});

	const phaseBadgeClass: Record<string, string> = {
		idle: 'preset-tonal',
		ready: 'preset-filled-warning-500',
		live: 'preset-filled-success-500'
	};

	const rows = $derived<Row[]>([
		...containers.map<Row>((c) => {
			const sc = scrapers[c.name];
			const summary = scraperWS.hostSummaries[c.name];
			return {
				name: c.name,
				source: 'container',
				running: !!sc?.running,
				phase: summary?.phase ?? null,
				title: summary?.title || sc?.title || '',
				score_summary: summary?.score_summary ?? null,
				xbox_name: sc?.xbox_name || ''
			};
		}),
		...orphans.map<Row>((o) => {
			const summary = scraperWS.hostSummaries[o.name];
			return {
				name: o.name,
				source: 'orphan',
				running: true,
				phase: summary?.phase ?? null,
				title: summary?.title || o.title || '',
				score_summary: summary?.score_summary ?? null,
				xbox_name: o.xbox_name || '',
				sock: o.sock
			};
		})
	]);
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6">
	<PageHeader
		title="Scraper debug"
		description="Pick a container to inspect every scraped field — the live game data, the latest tick, and recent events. Useful for verifying a freshly-added field is populated and for chasing offset drift."
	>
		{#snippet actions()}
			<ActivityIcon class="size-6" />
		{/snippet}
	</PageHeader>

	{#snippet sourceCell({ row }: { row: Row })}
		{#if row.source === 'container'}
			<span class="badge preset-tonal text-xs">Container</span>
		{:else}
			<span class="badge preset-tonal-warning text-xs" title={row.sock}>External QMP</span>
		{/if}
	{/snippet}
	{#snippet scraperCell({ row }: { row: Row })}
		{#if row.running}
			<span class="badge preset-filled-success-500 text-xs">Running</span>
		{:else}
			<span class="badge preset-tonal text-xs">Idle</span>
		{/if}
	{/snippet}
	{#snippet phaseCell({ row }: { row: Row })}
		{#if row.phase}
			<span class="badge {phaseBadgeClass[row.phase] ?? 'preset-tonal'} text-[10px] uppercase">
				{row.phase}
			</span>
		{:else}
			—
		{/if}
	{/snippet}
	{#snippet gameCell({ row }: { row: Row })}
		{row.title || '—'}
		{#if row.score_summary}
			<span class="text-surface-500-400 ml-1 font-mono text-xs">· {row.score_summary}</span>
		{/if}
	{/snippet}
	{#snippet xboxCell({ row }: { row: Row })}
		{row.xbox_name || '—'}
	{/snippet}
	{#snippet actionCell({ row }: { row: Row })}
		<a class="btn preset-filled-primary-500 text-xs" href={resolve(`/admin/debug/${row.name}/`)}>
			Open
		</a>
	{/snippet}

	<Card size="flush" class="overflow-hidden">
		<DataTable
			{rows}
			groups={[
				{
					columns: [
						{ key: 'name', label: 'Name' },
						{
							key: 'source',
							label: 'Source',
							cell: sourceCell,
							sortAccessor: (r) => r.source
						},
						{
							key: 'scraper',
							label: 'Scraper',
							cell: scraperCell,
							sortAccessor: (r) => (r.running ? 0 : 1)
						},
						{
							key: 'phase',
							label: 'Phase',
							cell: phaseCell,
							comparator: (a, b) => phaseRank(a.phase) - phaseRank(b.phase)
						},
						{
							key: 'title',
							label: 'Game',
							cell: gameCell,
							sortAccessor: (r) => r.title
						},
						{ key: 'xbox_name', label: 'Xbox name', cell: xboxCell },
						{
							key: 'action',
							label: 'Action',
							cell: actionCell,
							sortable: false,
							align: 'right'
						}
					]
				}
			] satisfies DataColumnGroup<Row>[]}
			rowKey={(r) => `${r.source}:${r.name}`}
			density="comfortable"
			defaultSort={{ key: 'name', dir: 'asc' }}
			secondarySort={{ key: 'name', dir: 'asc' }}
			loading={loading && rows.length === 0}
			emptyMessage="No containers yet. Provision one from the Containers page."
		/>
	</Card>
</div>
