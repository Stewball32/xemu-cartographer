<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		ArrowDownIcon,
		ArrowUpIcon,
		BugIcon,
		EraserIcon,
		EyeIcon,
		LoaderIcon,
		PlayIcon,
		RadarIcon,
		RefreshCwIcon,
		SearchIcon,
		SquareIcon,
		Trash2Icon
	} from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { adminGet, adminPost, adminDelete } from '$lib/utils/admin-api';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
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
		xbox_name: string;
		score_summary: string | null;
		sock?: string;
	};

	const NAME_PATTERN = /^[a-z0-9][a-z0-9_-]*$/;

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
	let sortState = $state<SortState>({ key: 'name', dir: 'asc' });
	let pollTimer: ReturnType<typeof setInterval> | null = null;

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

	async function handleDelete(name: string) {
		const targetStatus = statuses[name] ?? 'unknown';
		const runningWarning =
			targetStatus === 'running' ? ' It is currently running and will be force-stopped.' : '';
		const ok = await confirmToast({
			title: 'Delete container',
			description: `Permanently remove ${name}?${runningWarning}`,
			confirmLabel: 'Delete',
			type: targetStatus === 'running' ? 'error' : 'warning'
		});
		if (!ok) return;
		busy = { ...busy, [name]: 'delete' };
		try {
			await toastPromise(adminDelete(`containers/${encodeURIComponent(name)}`), {
				loading: { title: 'Deleting', description: name },
				success: { title: 'Deleted', description: name },
				errorTitle: 'Delete failed'
			});
			containers = containers.filter((x) => x.name !== name);
			const nextS = { ...statuses };
			delete nextS[name];
			statuses = nextS;
			const nextG = { ...scrapers };
			delete nextG[name];
			scrapers = nextG;
		} catch {
			// toast already shown
		} finally {
			busy = { ...busy, [name]: null };
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

	function statusBadgeClass(status: RowStatus): string {
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
	}

	const phaseBadgeClass: Record<string, string> = {
		idle: 'preset-tonal',
		ready: 'preset-filled-warning-500',
		live: 'preset-filled-success-500'
	};

	const rows = $derived<Row[]>([
		...containers.map<Row>((c) => {
			const sc = scrapers[c.name] ?? null;
			const summary = scraperWS.hostSummaries[c.name];
			return {
				name: c.name,
				source: 'container',
				created: c.created,
				status: statuses[c.name] ?? 'loading',
				running: !!sc?.running,
				phase: summary?.phase ?? null,
				title: summary?.title || sc?.title || '',
				xbox_name: sc?.xbox_name || '',
				score_summary: summary?.score_summary ?? null
			};
		}),
		...orphans.map<Row>((o) => {
			const summary = scraperWS.hostSummaries[o.name];
			return {
				name: o.name,
				source: 'orphan',
				created: '',
				status: 'running',
				running: true,
				phase: summary?.phase ?? null,
				title: summary?.title || o.title || '',
				xbox_name: o.xbox_name || '',
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
				r.xbox_name.toLowerCase().includes(q)
		);
	});

	function compareRows(a: Row, b: Row, key: string): number {
		switch (key) {
			case 'status':
				return statusPriority(a.status) - statusPriority(b.status);
			case 'phase':
				return phaseRank(a.phase) - phaseRank(b.phase);
			case 'game_xbox':
				return (a.title || '').localeCompare(b.title || '');
			case 'name':
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

	onMount(() => {
		loadAll();
		startPolling();
		if (auth.token) scraperWS.connect(auth.token);
	});

	onDestroy(() => {
		stopPolling();
		scraperWS.disconnect();
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

	<div class="input-group grid-cols-[auto_1fr]">
		<div class="ig-cell preset-tonal">
			<SearchIcon class="size-4" />
		</div>
		<input
			type="search"
			class="ig-input"
			placeholder="Filter by name, game, or Xbox name"
			bind:value={filter}
			aria-label="Filter containers"
		/>
	</div>

	<!-- Mobile: card per container. Stacked layout, 44px-min action buttons. -->
	<div class="flex flex-col gap-3 sm:hidden">
		<div class="flex items-center gap-2 text-xs text-surface-600-400">
			<span>Sort</span>
			<select
				class="select-sm select flex-1"
				bind:value={
					() => sortState?.key ?? 'name',
					(v) => (sortState = { key: v, dir: sortState?.dir ?? 'asc' })
				}
				aria-label="Sort by"
			>
				<option value="name">Name</option>
				<option value="status">Status</option>
				<option value="phase">Phase</option>
				<option value="game_xbox">Game</option>
			</select>
			<button
				type="button"
				class="btn-icon preset-tonal btn-sm"
				aria-label={sortState?.dir === 'desc' ? 'Sort descending' : 'Sort ascending'}
				title={sortState?.dir === 'desc' ? 'Descending' : 'Ascending'}
				onclick={() =>
					(sortState = {
						key: sortState?.key ?? 'name',
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
			{#each sortedFilteredRows as row (`${row.source}:${row.name}`)}
				{@const rowBusy = busy[row.name] ?? null}
				<!-- Sibling units own /pod/view/[name], /pod/debug/[name], and
					/pod/probe/[name]; until those routes exist they aren't in the
					typed route map, so the hrefs stay as plain strings. -->
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<Card size="sm">
					<div class="flex flex-col gap-3">
						<div class="flex items-start justify-between gap-2">
							<div class="min-w-0">
								<a
									href="/pod/view/{row.name}/"
									class="block truncate font-semibold hover:underline"
								>
									{row.name}
								</a>
								<div class="mt-1 flex flex-wrap items-center gap-1">
									<span class={statusBadgeClass(row.status)}>{row.status}</span>
									{#if row.phase}
										<span
											class="badge {phaseBadgeClass[row.phase] ??
												'preset-tonal'} text-[10px] uppercase"
										>
											{row.phase}
										</span>
									{/if}
									{#if row.source === 'orphan'}
										<span class="badge preset-tonal-warning text-[10px]" title={row.sock}>
											External QMP
										</span>
									{/if}
								</div>
							</div>
						</div>
						<div class="text-xs">
							<div class="font-medium">{row.title || '—'}</div>
							<div class="text-surface-600-400">{row.xbox_name || '—'}</div>
							{#if row.score_summary}
								<div class="text-surface-500-400 mt-1 font-mono">{row.score_summary}</div>
							{/if}
						</div>
						<div class="grid grid-cols-3 gap-2">
							<a
								href="/pod/view/{row.name}/"
								class="btn min-h-11 justify-center preset-tonal"
								aria-label="View"
							>
								<EyeIcon class="size-4" />
								<span>View</span>
							</a>
							<a
								href="/pod/debug/{row.name}/"
								class="btn min-h-11 justify-center preset-tonal"
								aria-label="Debug"
							>
								<BugIcon class="size-4" />
								<span>Debug</span>
							</a>
							<a
								href="/pod/probe/{row.name}/"
								class="btn min-h-11 justify-center preset-tonal"
								aria-label="Probe"
							>
								<RadarIcon class="size-4" />
								<span>Probe</span>
							</a>
							{#if row.source === 'container'}
								{#if row.status === 'running'}
									<button
										type="button"
										class="btn min-h-11 justify-center preset-tonal-warning"
										aria-label="Stop"
										onclick={() => handleStop(row.name)}
										disabled={rowBusy !== null}
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
										class="btn min-h-11 justify-center preset-tonal-success"
										aria-label="Start"
										onclick={() => handleStart(row.name)}
										disabled={rowBusy !== null}
									>
										{#if rowBusy === 'start'}
											<LoaderIcon class="size-4 animate-spin" />
										{:else}
											<PlayIcon class="size-4" />
										{/if}
										<span>Start</span>
									</button>
								{/if}
								<button
									type="button"
									class="col-span-2 btn min-h-11 justify-center preset-tonal-error"
									aria-label="Delete"
									onclick={() => handleDelete(row.name)}
									disabled={rowBusy !== null}
								>
									{#if rowBusy === 'delete'}
										<LoaderIcon class="size-4 animate-spin" />
									{:else}
										<Trash2Icon class="size-4" />
									{/if}
									<span>Delete</span>
								</button>
							{/if}
						</div>
						<!-- eslint-enable svelte/no-navigation-without-resolve -->
					</div>
				</Card>
			{/each}
		{/if}
	</div>

	<!-- Desktop / tablet: sortable DataTable. -->
	<div class="hidden sm:block">
		{#snippet statusCell({ row }: { row: Row })}
			<div class="flex flex-wrap items-center gap-1">
				<span class={statusBadgeClass(row.status)}>{row.status}</span>
				{#if row.source === 'orphan'}
					<span class="badge preset-tonal-warning text-[10px]" title={row.sock}>External QMP</span>
				{/if}
			</div>
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
		{#snippet gameXboxCell({ row }: { row: Row })}
			<div class="font-medium">{row.title || '—'}</div>
			<div class="text-xs text-surface-600-400">{row.xbox_name || '—'}</div>
			{#if row.score_summary}
				<div class="text-surface-500-400 mt-0.5 font-mono text-xs">{row.score_summary}</div>
			{/if}
		{/snippet}
		{#snippet actionsCell({ row }: { row: Row })}
			{@const rowBusy = busy[row.name] ?? null}
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<div
				role="presentation"
				class="inline-flex flex-wrap items-center justify-end gap-3"
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => e.stopPropagation()}
			>
				<div class="inline-flex gap-1">
					<a
						href="/pod/view/{row.name}/"
						class="btn-icon preset-tonal btn-sm"
						aria-label="View"
						title="View"
					>
						<EyeIcon class="size-4" />
					</a>
					<a
						href="/pod/debug/{row.name}/"
						class="btn-icon preset-tonal btn-sm"
						aria-label="Debug"
						title="Debug"
					>
						<BugIcon class="size-4" />
					</a>
					<a
						href="/pod/probe/{row.name}/"
						class="btn-icon preset-tonal btn-sm"
						aria-label="Probe"
						title="Probe"
					>
						<RadarIcon class="size-4" />
					</a>
				</div>
				{#if row.source === 'container'}
					<div class="inline-flex gap-1">
						{#if row.status === 'running'}
							<button
								type="button"
								class="btn-icon preset-tonal-warning btn-sm"
								aria-label="Stop"
								title="Stop"
								onclick={() => handleStop(row.name)}
								disabled={rowBusy !== null}
							>
								{#if rowBusy === 'stop'}
									<LoaderIcon class="size-4 animate-spin" />
								{:else}
									<SquareIcon class="size-4" />
								{/if}
							</button>
						{:else}
							<button
								type="button"
								class="btn-icon preset-tonal-success btn-sm"
								aria-label="Start"
								title="Start"
								onclick={() => handleStart(row.name)}
								disabled={rowBusy !== null}
							>
								{#if rowBusy === 'start'}
									<LoaderIcon class="size-4 animate-spin" />
								{:else}
									<PlayIcon class="size-4" />
								{/if}
							</button>
						{/if}
						<button
							type="button"
							class="btn-icon preset-tonal-error btn-sm"
							aria-label="Delete"
							title="Delete"
							onclick={() => handleDelete(row.name)}
							disabled={rowBusy !== null}
						>
							{#if rowBusy === 'delete'}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<Trash2Icon class="size-4" />
							{/if}
						</button>
					</div>
				{/if}
			</div>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/snippet}

		<Card size="flush" class="overflow-x-auto">
			<DataTable
				rows={filteredRows}
				groups={[
					{
						columns: [
							{ key: 'name', label: 'Name' },
							{
								key: 'status',
								label: 'Status',
								cell: statusCell,
								comparator: (a, b) => statusPriority(a.status) - statusPriority(b.status)
							},
							{
								key: 'phase',
								label: 'Phase',
								cell: phaseCell,
								comparator: (a, b) => phaseRank(a.phase) - phaseRank(b.phase)
							},
							{
								key: 'game_xbox',
								label: 'Game / Xbox',
								cell: gameXboxCell,
								sortAccessor: (r) => r.title ?? ''
							},
							{
								key: 'actions',
								label: 'Actions',
								cell: actionsCell,
								sortable: false,
								align: 'right'
							}
						]
					}
				] satisfies DataColumnGroup<Row>[]}
				rowKey={(r) => `${r.source}:${r.name}`}
				density="comfortable"
				sort={sortState}
				onSortChange={(s) => (sortState = s)}
				secondarySort={{ key: 'name', dir: 'asc' }}
				onRowClick={(row) => goto(resolve('/pod/view/[name]', { name: row.name }))}
				loading={loading && filteredRows.length === 0}
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
