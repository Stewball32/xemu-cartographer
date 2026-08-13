<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		ArrowDownIcon,
		ArrowUpIcon,
		EraserIcon,
		LoaderIcon,
		PlayIcon,
		RefreshCwIcon,
		SearchIcon,
		SquareIcon,
		Trash2Icon,
		XIcon
	} from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { SvelteSet } from 'svelte/reactivity';
	import { adminGet, adminPost, adminDelete } from '$lib/utils/admin-api';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup, SortState } from '$lib/components/ui/data-table';
	import type {
		ContainerInfo,
		ContainerStatus,
		ContainerDetail,
		InstanceState
	} from '$lib/types/containers';
	import type { Phase, ScraperInfo } from '$lib/types/scraper';

	type RowStatus = ContainerStatus | 'loading' | string;
	type Source = 'container' | 'orphan';
	type BusyAction = 'start' | 'stop' | 'delete';
	type Row = {
		name: string;
		source: Source;
		created: string;
		status: RowStatus;
		running: boolean;
		phase: Phase | null;
		title: string;
		xbe_title_name: string;
		xbox_name: string;
		map: string;
		gametype: string;
		score_summary: string | null;
		sock?: string;
	};
	type BulkActionKind = 'start' | 'stop' | 'delete';
	type BulkConfirmState = {
		action: BulkActionKind;
		affected: string[];
		effective: string[];
	};

	const NAME_PATTERN = /^[a-z0-9][a-z0-9_-]*$/;

	function tc(s: string): string {
		return s ? s[0].toUpperCase() + s.slice(1) : s;
	}

	function statusPriority(s: RowStatus): number {
		switch (s) {
			case 'running':
				return 0;
			case 'starting':
			case 'stopping':
			case 'loading':
				return 1;
			case 'created':
			case 'paused':
				return 2;
			case 'exited':
			case 'stopped':
				return 3;
			default:
				return 4;
		}
	}

	const phasePriority: Record<Phase, number> = { live: 0, ready: 1, idle: 2 };
	function phaseRank(p: Phase | null): number {
		return p ? phasePriority[p] : 99;
	}

	let containers = $state<ContainerInfo[]>([]);
	let statuses = $state<Record<string, RowStatus>>({});
	let scrapers = $state<Record<string, InstanceState | null>>({});
	let orphans = $state<ScraperInfo[]>([]);
	let busy = $state<Record<string, BusyAction | null>>({});
	let loading = $state(true);
	let cleanupBusy = $state(false);
	let createOpen = $state(false);
	let createName = $state('');
	let createBusy = $state(false);
	let filter = $state('');
	let sortState = $state<SortState>({ key: 'pod_xbox', dir: 'asc' });
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	let selected = new SvelteSet<string>();
	let bulkBusy = $state(false);
	let bulkConfirm = $state<BulkConfirmState | null>(null);

	async function loadAll() {
		try {
			loading = true;
			const list = await adminGet<ContainerInfo[] | null>('containers');
			containers = list ?? [];
			const next: Record<string, RowStatus> = {};
			for (const c of containers) next[c.name] = statuses[c.name] ?? 'loading';
			statuses = next;
			await refreshAll();
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	async function refreshDetail(name: string) {
		try {
			const res = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
			statuses = { ...statuses, [name]: res.status };
			scrapers = { ...scrapers, [name]: res.scraper };
		} catch (err) {
			statuses = { ...statuses, [name]: 'unknown' };
			console.warn('detail fetch failed for', name, err);
		}
	}

	async function refreshOrphans() {
		try {
			const list = await adminGet<ScraperInfo[] | null>('scraper');
			const known = new Set(containers.map((c) => c.name));
			orphans = (list ?? []).filter((s) => !known.has(s.name));
		} catch (err) {
			console.warn('scraper list fetch failed', err);
		}
	}

	async function refreshAll() {
		await Promise.all([...containers.map((c) => refreshDetail(c.name)), refreshOrphans()]);
	}

	function startPolling() {
		stopPolling();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			if (bulkBusy) return;
			refreshAll();
		}, 3000);
	}

	function stopPolling() {
		if (pollTimer !== null) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function handleCreate() {
		const name = createName.trim();
		if (!NAME_PATTERN.test(name)) {
			toaster.error({
				title: 'Invalid name',
				description:
					'Use lowercase letters, digits, hyphens, or underscores. Must start with a letter or digit.'
			});
			return;
		}
		try {
			createBusy = true;
			const created = await toastPromise(adminPost<ContainerInfo>('containers', { name }), {
				loading: { title: 'Creating', description: name },
				success: { title: 'Container created', description: name },
				errorTitle: 'Create failed'
			});
			containers = [created, ...containers];
			statuses = { ...statuses, [created.name]: 'created' };
			createName = '';
			createOpen = false;
		} catch {
			// toast already shown
		} finally {
			createBusy = false;
		}
	}

	async function handleStart(name: string) {
		busy = { ...busy, [name]: 'start' };
		statuses = { ...statuses, [name]: 'loading' };
		try {
			await toastPromise(adminPost(`containers/${encodeURIComponent(name)}/start`), {
				loading: { title: 'Starting', description: name },
				success: { title: 'Started', description: name },
				errorTitle: 'Start failed'
			});
		} catch {
			// toast already shown
		} finally {
			busy = { ...busy, [name]: null };
			await refreshDetail(name);
		}
	}

	async function handleStop(name: string) {
		const ok = await confirmToast({
			title: 'Stop container',
			description: `Stop ${name}? Any active session will end immediately.`,
			confirmLabel: 'Stop',
			type: 'warning'
		});
		if (!ok) return;
		busy = { ...busy, [name]: 'stop' };
		statuses = { ...statuses, [name]: 'loading' };
		try {
			await toastPromise(adminPost(`containers/${encodeURIComponent(name)}/stop`), {
				loading: { title: 'Stopping', description: name },
				success: { title: 'Stopped', description: name },
				errorTitle: 'Stop failed'
			});
		} catch {
			// toast already shown
		} finally {
			busy = { ...busy, [name]: null };
			await refreshDetail(name);
		}
	}

	async function handleCleanup() {
		const ok = await confirmToast({
			title: 'Cleanup orphaned files',
			description:
				'Remove on-disk config dirs and HDD files for any container no longer tracked. Baseline HDDs starting with _ are preserved.',
			confirmLabel: 'Cleanup'
		});
		if (!ok) return;
		cleanupBusy = true;
		try {
			await toastPromise(adminPost<{ deleted: string[] | null }>('containers/cleanup'), {
				loading: { title: 'Cleaning up' },
				success: (res) => {
					const count = res.deleted?.length ?? 0;
					return count > 0
						? {
								title: 'Cleaned up',
								description: `Removed ${count} orphaned ${count === 1 ? 'entry' : 'entries'}`
							}
						: { title: 'Nothing to clean', description: 'No orphaned files were found.' };
				},
				errorTitle: 'Cleanup failed'
			});
			await loadAll();
		} catch {
			// toast already shown
		} finally {
			cleanupBusy = false;
		}
	}

	function rawStart(name: string): Promise<unknown> {
		return adminPost(`containers/${encodeURIComponent(name)}/start`);
	}
	function rawStop(name: string): Promise<unknown> {
		return adminPost(`containers/${encodeURIComponent(name)}/stop`);
	}
	function rawDelete(name: string): Promise<unknown> {
		return adminDelete(`containers/${encodeURIComponent(name)}`);
	}

	function openBulkConfirm(action: BulkActionKind) {
		const containerNames: string[] = [];
		for (const r of rows) {
			if (r.source !== 'container') continue;
			if (selected.has(r.name)) containerNames.push(r.name);
		}
		if (containerNames.length === 0) {
			toaster.info({ title: 'Nothing selected' });
			return;
		}
		let effective: string[];
		if (action === 'start') {
			effective = containerNames.filter((n) => statuses[n] !== 'running');
		} else if (action === 'stop') {
			effective = containerNames.filter((n) => statuses[n] === 'running');
		} else {
			effective = containerNames;
		}
		bulkConfirm = { action, affected: containerNames, effective };
	}

	function bulkActionLabel(a: BulkActionKind): string {
		return a === 'start' ? 'Start' : a === 'stop' ? 'Stop' : 'Delete';
	}
	function bulkActionPast(a: BulkActionKind): string {
		return a === 'start' ? 'Started' : a === 'stop' ? 'Stopped' : 'Deleted';
	}

	async function runBulkConfirm() {
		if (!bulkConfirm) return;
		const { action, effective } = bulkConfirm;
		bulkConfirm = null;
		if (effective.length === 0) {
			toaster.info({ title: 'Nothing to do' });
			return;
		}
		bulkBusy = true;
		const busyKey: BusyAction = action;
		const nextBusy = { ...busy };
		const nextStatuses = { ...statuses };
		for (const n of effective) {
			nextBusy[n] = busyKey;
			if (action !== 'delete') nextStatuses[n] = 'loading';
		}
		busy = nextBusy;
		statuses = nextStatuses;
		const fn = action === 'start' ? rawStart : action === 'stop' ? rawStop : rawDelete;
		const results = await Promise.allSettled(effective.map((n) => fn(n)));
		const succeeded: string[] = [];
		const failed: { name: string; err: unknown }[] = [];
		results.forEach((r, i) => {
			const n = effective[i];
			if (r.status === 'fulfilled') succeeded.push(n);
			else failed.push({ name: n, err: r.reason });
		});
		const clearBusy = { ...busy };
		for (const n of effective) clearBusy[n] = null;
		busy = clearBusy;
		for (const n of succeeded) selected.delete(n);
		if (action === 'delete' && succeeded.length > 0) {
			const removed = new Set(succeeded);
			containers = containers.filter((c) => !removed.has(c.name));
			const sNext = { ...statuses };
			const gNext = { ...scrapers };
			for (const n of succeeded) {
				delete sNext[n];
				delete gNext[n];
			}
			statuses = sNext;
			scrapers = gNext;
		}
		bulkBusy = false;
		await refreshAll();
		const past = bulkActionPast(action);
		if (failed.length === 0) {
			toaster.success({
				title: `${past} ${succeeded.length}`,
				description: succeeded.join(', ')
			});
		} else if (succeeded.length === 0) {
			toaster.error({
				title: `${bulkActionLabel(action)} failed`,
				description: failed.map((f) => f.name).join(', ')
			});
			for (const f of failed) console.error(`${action} ${f.name}:`, f.err);
		} else {
			toaster.warning({
				title: `${past} ${succeeded.length} — ${failed.length} failed`,
				description: failed.map((f) => f.name).join(', ')
			});
			for (const f of failed) console.error(`${action} ${f.name}:`, f.err);
		}
	}

	function clearSelection() {
		selected.clear();
	}

	const statusBadgeClass = (status: RowStatus): string => {
		switch (status) {
			case 'running':
				return 'badge preset-filled-success-500';
			case 'exited':
			case 'stopped':
				return 'badge preset-tonal-error';
			case 'created':
			case 'paused':
			case 'stopping':
				return 'badge preset-tonal-warning';
			case 'loading':
				return 'badge preset-tonal';
			default:
				return 'badge preset-tonal-surface';
		}
	};

	const phaseBadgeClass: Record<string, string> = {
		idle: 'preset-tonal',
		ready: 'preset-filled-warning-500',
		live: 'preset-filled-success-500'
	};

	const rows = $derived<Row[]>([
		...containers.map<Row>((c) => {
			const sc = scrapers[c.name] ?? null;
			const summary = scraperWSV2.hostSummaries[c.name];
			return {
				name: c.name,
				source: 'container',
				created: c.created,
				status: statuses[c.name] ?? 'loading',
				running: !!sc?.running,
				phase: summary?.phase ?? null,
				title: summary?.title || sc?.title || '',
				xbe_title_name: summary?.xbe_title_name || '',
				xbox_name: sc?.xbox_name || '',
				map: summary?.map || '',
				gametype: summary?.gametype || '',
				score_summary: summary?.score_summary ?? null
			};
		}),
		...orphans.map<Row>((o) => {
			const summary = scraperWSV2.hostSummaries[o.name];
			return {
				name: o.name,
				source: 'orphan',
				created: '',
				status: 'running',
				running: true,
				phase: summary?.phase ?? null,
				title: summary?.title || o.title || '',
				xbe_title_name: summary?.xbe_title_name || '',
				xbox_name: o.xbox_name || '',
				map: summary?.map || '',
				gametype: summary?.gametype || '',
				score_summary: summary?.score_summary ?? null,
				sock: o.sock
			};
		})
	]);

	const filteredRows = $derived.by<Row[]>(() => {
		const q = filter.trim().toLowerCase();
		if (!q) return rows;
		return rows.filter(
			(r) =>
				r.name.toLowerCase().includes(q) ||
				r.title.toLowerCase().includes(q) ||
				r.xbe_title_name.toLowerCase().includes(q) ||
				r.xbox_name.toLowerCase().includes(q) ||
				r.gametype.toLowerCase().includes(q) ||
				r.map.toLowerCase().includes(q)
		);
	});

	function compareRows(a: Row, b: Row, key: string): number {
		switch (key) {
			case 'status_phase': {
				const sp = statusPriority(a.status) - statusPriority(b.status);
				if (sp !== 0) return sp;
				return phaseRank(a.phase) - phaseRank(b.phase);
			}
			case 'xbe_title': {
				const av = a.xbe_title_name || a.title || '';
				const bv = b.xbe_title_name || b.title || '';
				return av.localeCompare(bv);
			}
			case 'game': {
				const av = a.gametype || '';
				const bv = b.gametype || '';
				// rows with no gametype sort last
				if (av && !bv) return -1;
				if (!av && bv) return 1;
				return av.localeCompare(bv);
			}
			case 'pod_xbox':
			default:
				return a.name.localeCompare(b.name);
		}
	}

	const sortedFilteredRows = $derived.by<Row[]>(() => {
		const s = sortState;
		if (!s) return filteredRows;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...filteredRows].sort((a, b) => {
			const primary = compareRows(a, b, s.key) * dir;
			if (primary !== 0) return primary;
			return a.name.localeCompare(b.name);
		});
	});

	const selectedCount = $derived(selected.size);

	function isSelectable(r: Row): boolean {
		return r.source === 'container';
	}

	function toggleRowSelection(name: string) {
		if (selected.has(name)) selected.delete(name);
		else selected.add(name);
	}

	function showGameSection(row: Row): boolean {
		return row.phase === 'live' || row.phase === 'ready';
	}

	function gameLine(row: Row): string {
		return [row.gametype, row.map].filter(Boolean).join(' • ');
	}

	onMount(() => {
		loadAll();
		startPolling();
		if (auth.token) {
			scraperWSV2.connect(auth.token);
			scraperWSV2.subscribeSummary();
		}
	});

	onDestroy(() => {
		stopPolling();
		scraperWSV2.unsubscribeSummary();
		scraperWSV2.disconnect();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Pod"
		description="Manage xemu + browser pairs and inspect their live scrapers."
	>
		{#snippet actions()}
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => loadAll()}
				disabled={loading}
				aria-label="Refresh"
			>
				{#if loading}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<RefreshCwIcon class="size-4" />
				{/if}
				<span>Refresh</span>
			</button>
			<button
				type="button"
				class="btn preset-tonal-error"
				onclick={handleCleanup}
				disabled={loading || cleanupBusy}
				aria-label="Cleanup files"
				title="Remove on-disk artefacts for containers no longer tracked"
			>
				{#if cleanupBusy}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<EraserIcon class="size-4" />
				{/if}
				<span>Cleanup files</span>
			</button>
			<button type="button" class="btn preset-filled" onclick={() => (createOpen = true)}>
				+ <span class="ml-1">New</span>
			</button>
		{/snippet}
	</PageHeader>

	{#if selectedCount > 0}
		<div
			class="sticky top-0 z-20 flex flex-wrap items-center gap-2 card border border-surface-300-700 preset-tonal px-3 py-2 shadow-md backdrop-blur"
		>
			<span class="text-sm font-medium">{selectedCount} selected</span>
			<div class="ms-auto flex flex-wrap gap-2">
				<button
					type="button"
					class="btn preset-tonal-success btn-sm"
					onclick={() => openBulkConfirm('start')}
					disabled={bulkBusy}
				>
					{#if bulkBusy}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<PlayIcon class="size-4" />
					{/if}
					<span>Start</span>
				</button>
				<button
					type="button"
					class="btn preset-tonal-warning btn-sm"
					onclick={() => openBulkConfirm('stop')}
					disabled={bulkBusy}
				>
					{#if bulkBusy}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<SquareIcon class="size-4" />
					{/if}
					<span>Stop</span>
				</button>
				<button
					type="button"
					class="btn preset-tonal-error btn-sm"
					onclick={() => openBulkConfirm('delete')}
					disabled={bulkBusy}
				>
					{#if bulkBusy}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<Trash2Icon class="size-4" />
					{/if}
					<span>Delete</span>
				</button>
				<button
					type="button"
					class="btn-icon preset-tonal btn-sm"
					onclick={clearSelection}
					disabled={bulkBusy}
					aria-label="Clear selection"
					title="Clear selection"
				>
					<XIcon class="size-4" />
				</button>
			</div>
		</div>
	{/if}

	<div class="field-group grid-cols-[auto_1fr]">
		<div class="flex items-center justify-center preset-tonal px-3">
			<SearchIcon class="size-4" />
		</div>
		<input
			type="search"
			class="input"
			placeholder="Filter by name, game, or Xbox"
			bind:value={filter}
			aria-label="Filter containers"
		/>
	</div>

	<!-- Mobile + tablet: card grid (1 col < sm, 2 col sm–lg). Sort control above. -->
	<div class="flex flex-col gap-3 lg:hidden">
		<div class="flex items-center gap-2 text-xs text-surface-600-400">
			<span>Sort</span>
			<select
				class="select-sm select flex-1"
				bind:value={
					() => sortState?.key ?? 'pod_xbox',
					(v) => (sortState = { key: v, dir: sortState?.dir ?? 'asc' })
				}
				aria-label="Sort by"
			>
				<option value="pod_xbox">Pod / Xbox</option>
				<option value="status_phase">Status / Phase</option>
				<option value="xbe_title">XBE / Title</option>
				<option value="game">Game</option>
			</select>
			<button
				type="button"
				class="btn-icon preset-tonal btn-sm"
				aria-label={sortState?.dir === 'desc' ? 'Sort descending' : 'Sort ascending'}
				title={sortState?.dir === 'desc' ? 'Descending' : 'Ascending'}
				onclick={() =>
					(sortState = {
						key: sortState?.key ?? 'pod_xbox',
						dir: sortState?.dir === 'desc' ? 'asc' : 'desc'
					})}
			>
				{#if sortState?.dir === 'desc'}
					<ArrowDownIcon class="size-4" />
				{:else}
					<ArrowUpIcon class="size-4" />
				{/if}
			</button>
		</div>
		{#if loading && sortedFilteredRows.length === 0}
			<Card><div class="text-sm text-surface-600-400">Loading…</div></Card>
		{:else if sortedFilteredRows.length === 0}
			<Card>
				<div class="text-sm text-surface-600-400">
					{filter ? 'No matches for that filter.' : 'No containers yet. Create one to get started.'}
				</div>
			</Card>
		{:else}
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4">
				{#each sortedFilteredRows as row (row.name)}
					{@const rowBusy = busy[row.name] ?? null}
					{@const canSelect = isSelectable(row)}
					<Card size="sm">
						<div class="flex flex-col gap-3">
							<div class="flex items-start gap-2">
								{#if canSelect}
									<div
										class="mt-1 flex items-center"
										role="presentation"
										onclick={(e) => e.stopPropagation()}
										onkeydown={(e) => e.stopPropagation()}
									>
										<input
											type="checkbox"
											class="checkbox"
											checked={selected.has(row.name)}
											onchange={() => toggleRowSelection(row.name)}
											aria-label="Select {row.name}"
										/>
									</div>
								{:else}
									<div class="mt-1 w-4" aria-hidden="true"></div>
								{/if}
								<div class="min-w-0 flex-1">
									<a
										href={resolve('/admin/pod/[name]', { name: row.name })}
										class="block truncate font-semibold hover:underline"
									>
										{row.name}
									</a>
									{#if row.xbox_name}
										<div class="truncate text-xs text-surface-600-400">{row.xbox_name}</div>
									{/if}
								</div>
								{#if row.source === 'container'}
									<div onclick={(e) => e.stopPropagation()} role="presentation">
										{#if row.status === 'running'}
											<button
												type="button"
												class="btn preset-tonal-warning btn-sm"
												aria-label="Stop"
												onclick={() => handleStop(row.name)}
												disabled={rowBusy !== null || bulkBusy}
											>
												{#if rowBusy === 'stop'}
													<LoaderIcon class="size-4 animate-spin" />
												{:else}
													<SquareIcon class="size-4" />
												{/if}
												<span>Stop</span>
											</button>
										{:else}
											<button
												type="button"
												class="btn preset-tonal-success btn-sm"
												aria-label="Start"
												onclick={() => handleStart(row.name)}
												disabled={rowBusy !== null || bulkBusy}
											>
												{#if rowBusy === 'start'}
													<LoaderIcon class="size-4 animate-spin" />
												{:else}
													<PlayIcon class="size-4" />
												{/if}
												<span>Start</span>
											</button>
										{/if}
									</div>
								{/if}
							</div>

							<div class="flex flex-wrap items-center gap-1">
								<span class={statusBadgeClass(row.status)}>{tc(row.status)}</span>
								{#if row.phase}
									<span class="badge {phaseBadgeClass[row.phase] ?? 'preset-tonal'}">
										{tc(row.phase)}
									</span>
								{/if}
								{#if row.source === 'orphan'}
									<span class="badge preset-tonal-warning text-[10px]" title={row.sock}>
										External QMP
									</span>
								{/if}
							</div>

							{#if row.xbe_title_name || row.title}
								<div class="text-xs">
									<div class="font-medium">{row.xbe_title_name || row.title}</div>
									{#if row.title && row.xbe_title_name && row.title !== row.xbe_title_name}
										<div class="text-surface-600-400">{row.title}</div>
									{/if}
								</div>
							{:else}
								<div
									class="text-surface-500-400 flex flex-1 items-center justify-center py-6 text-sm italic"
								>
									Not running
								</div>
							{/if}

							{#if showGameSection(row)}
								<div class="border-t border-surface-300-700 pt-2 text-xs">
									<div class="font-medium">{gameLine(row) || '—'}</div>
									{#if row.score_summary}
										<div class="text-surface-500-400 mt-0.5 font-mono">{row.score_summary}</div>
									{/if}
								</div>
							{/if}
						</div>
					</Card>
				{/each}
			</div>
		{/if}
	</div>

	<!-- Desktop (≥ lg): DataTable with selection + sortable headers. -->
	<div class="hidden lg:block">
		{#snippet podXboxCell({ row }: { row: Row })}
			<div class="font-medium">{row.name}</div>
			<div class="text-xs text-surface-600-400">{row.xbox_name || '—'}</div>
		{/snippet}
		{#snippet statusPhaseCell({ row }: { row: Row })}
			<div class="flex flex-col items-start gap-1">
				<div class="flex flex-wrap items-center gap-1">
					<span class={statusBadgeClass(row.status)}>{tc(row.status)}</span>
					{#if row.source === 'orphan'}
						<span class="badge preset-tonal-warning text-[10px]" title={row.sock}>
							External QMP
						</span>
					{/if}
				</div>
				{#if row.phase}
					<span class="badge {phaseBadgeClass[row.phase] ?? 'preset-tonal'}">{tc(row.phase)}</span>
				{/if}
			</div>
		{/snippet}
		{#snippet xbeTitleCell({ row }: { row: Row })}
			<div class="font-medium">{row.xbe_title_name || row.title || '—'}</div>
			{#if row.title && row.xbe_title_name && row.title !== row.xbe_title_name}
				<div class="text-xs text-surface-600-400">{row.title}</div>
			{/if}
		{/snippet}
		{#snippet gameCell({ row }: { row: Row })}
			{#if showGameSection(row)}
				<div class="font-medium">{gameLine(row) || '—'}</div>
				{#if row.score_summary}
					<div class="text-surface-500-400 mt-0.5 font-mono text-xs">{row.score_summary}</div>
				{/if}
			{:else}
				<span class="text-surface-500-400">—</span>
			{/if}
		{/snippet}
		{#snippet controlCell({ row }: { row: Row })}
			{@const rowBusy = busy[row.name] ?? null}
			<div
				role="presentation"
				class="inline-flex items-center justify-end gap-1"
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => e.stopPropagation()}
			>
				{#if row.source === 'container'}
					{#if row.status === 'running'}
						<button
							type="button"
							class="btn preset-tonal-warning btn-sm"
							aria-label="Stop"
							title="Stop"
							onclick={() => handleStop(row.name)}
							disabled={rowBusy !== null || bulkBusy}
						>
							{#if rowBusy === 'stop'}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<SquareIcon class="size-4" />
							{/if}
							<span>Stop</span>
						</button>
					{:else}
						<button
							type="button"
							class="btn preset-tonal-success btn-sm"
							aria-label="Start"
							title="Start"
							onclick={() => handleStart(row.name)}
							disabled={rowBusy !== null || bulkBusy}
						>
							{#if rowBusy === 'start'}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<PlayIcon class="size-4" />
							{/if}
							<span>Start</span>
						</button>
					{/if}
				{/if}
			</div>
		{/snippet}

		<Card size="flush" class="overflow-x-auto">
			<DataTable
				rows={sortedFilteredRows}
				groups={[
					{
						columns: [
							{
								key: 'pod_xbox',
								label: 'Pod / Xbox',
								cell: podXboxCell,
								sortAccessor: (r) => r.name
							},
							{
								key: 'status_phase',
								label: 'Status / Phase',
								cell: statusPhaseCell,
								comparator: (a, b) => {
									const sp = statusPriority(a.status) - statusPriority(b.status);
									if (sp !== 0) return sp;
									return phaseRank(a.phase) - phaseRank(b.phase);
								}
							},
							{
								key: 'xbe_title',
								label: 'XBE / Title',
								cell: xbeTitleCell,
								sortAccessor: (r) => r.xbe_title_name || r.title || ''
							},
							{
								key: 'game',
								label: 'Game',
								cell: gameCell,
								comparator: (a, b) => compareRows(a, b, 'game')
							},
							{
								key: 'control',
								label: '',
								cell: controlCell,
								sortable: false,
								align: 'right'
							}
						]
					}
				] satisfies DataColumnGroup<Row>[]}
				rowKey={(r) => r.name}
				density="comfortable"
				sort={sortState}
				onSortChange={(s) => (sortState = s)}
				secondarySort={{ key: 'pod_xbox', dir: 'asc' }}
				onRowClick={(row) => goto(resolve('/admin/pod/[name]', { name: row.name }))}
				selectable
				{selected}
				selectableRow={isSelectable}
				loading={loading && sortedFilteredRows.length === 0}
				emptyMessage={filter
					? 'No matches for that filter.'
					: 'No containers yet. Create one to get started.'}
			/>
		</Card>
	</div>
</div>

<Dialog
	open={createOpen}
	onClose={() => {
		if (!createBusy) createOpen = false;
	}}
	title="New container"
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			handleCreate();
		}}
		class="flex flex-col gap-4"
	>
		<label class="label">
			<span class="label-text">Name</span>
			<!-- svelte-ignore a11y_autofocus -->
			<input
				type="text"
				class="input"
				bind:value={createName}
				placeholder="e.g. smoke"
				autocomplete="off"
				autofocus
				disabled={createBusy}
			/>
			<span class="text-xs text-surface-600-400">
				Lowercase letters, digits, <code>-</code>, <code>_</code>. Must start with a letter or
				digit.
			</span>
		</label>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (createOpen = false)}
				disabled={createBusy}
			>
				Cancel
			</button>
			<button type="submit" class="btn preset-filled" disabled={createBusy}>
				{#if createBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Create</span>
			</button>
		</div>
	</form>
</Dialog>

<Dialog
	open={bulkConfirm !== null}
	onClose={() => {
		if (!bulkBusy) bulkConfirm = null;
	}}
	title={bulkConfirm
		? `${bulkActionLabel(bulkConfirm.action)} ${bulkConfirm.affected.length} pod${bulkConfirm.affected.length === 1 ? '' : 's'}`
		: ''}
>
	{#if bulkConfirm}
		{@const cfg = bulkConfirm}
		{@const skipped = cfg.affected.length - cfg.effective.length}
		<div class="flex flex-col gap-3 text-sm">
			{#if cfg.effective.length === 0}
				<p class="text-surface-600-400">
					Nothing to do — all {cfg.affected.length} selected pod{cfg.affected.length === 1
						? ' is'
						: 's are'} already in the target state.
				</p>
			{:else if skipped > 0}
				<p>
					{bulkActionLabel(cfg.action)}
					<strong>{cfg.effective.length}</strong>
					of {cfg.affected.length} selected —
					<span class="text-surface-600-400">
						{skipped} already {cfg.action === 'start' ? 'running' : 'stopped'}.
					</span>
				</p>
			{:else}
				<p>
					{bulkActionLabel(cfg.action)}
					<strong>{cfg.effective.length}</strong>
					{cfg.effective.length === 1 ? 'pod' : 'pods'}?
				</p>
			{/if}

			{#if cfg.action === 'delete'}
				<p class="text-xs text-error-600-400">
					Containers will be permanently removed. Running pods will be force-stopped.
				</p>
			{/if}

			{#if cfg.effective.length > 0}
				<ul class="max-h-40 overflow-y-auto text-xs text-surface-700-300">
					{#each cfg.effective.slice(0, 10) as name (name)}
						<li class="font-mono">{name}</li>
					{/each}
					{#if cfg.effective.length > 10}
						<li class="text-surface-500-400">+ {cfg.effective.length - 10} more</li>
					{/if}
				</ul>
			{/if}

			<div class="flex justify-end gap-2">
				<button
					type="button"
					class="btn preset-tonal"
					onclick={() => (bulkConfirm = null)}
					disabled={bulkBusy}
				>
					Cancel
				</button>
				{#if cfg.effective.length > 0}
					<button
						type="button"
						class="btn {cfg.action === 'delete'
							? 'preset-filled-error'
							: cfg.action === 'stop'
								? 'preset-filled-warning'
								: 'preset-filled-success'}"
						onclick={runBulkConfirm}
						disabled={bulkBusy}
					>
						{#if bulkBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
						<span>{bulkActionLabel(cfg.action)} {cfg.effective.length}</span>
					</button>
				{/if}
			</div>
		</div>
	{/if}
</Dialog>
