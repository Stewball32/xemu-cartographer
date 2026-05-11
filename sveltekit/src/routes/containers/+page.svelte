<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import {
		PlayIcon,
		SquareIcon,
		Trash2Icon,
		EraserIcon,
		RefreshCwIcon,
		EyeIcon,
		LoaderIcon
	} from '@lucide/svelte';
	import { adminGet, adminPost, adminDelete, AdminFetchError } from '$lib/utils/admin-api';
	import { toaster, toastPromise, confirmToast } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import type {
		ContainerInfo,
		ContainerStatus,
		ContainerDetail,
		InstanceState
	} from '$lib/types/containers';

	type RowStatus = ContainerStatus | 'loading' | string;
	type Row = ContainerInfo & { status: RowStatus; scraper: InstanceState | null };

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

	type BusyAction = 'start' | 'stop' | 'delete';

	let containers = $state<ContainerInfo[]>([]);
	let statuses = $state<Record<string, RowStatus>>({});
	let scrapers = $state<Record<string, InstanceState | null>>({});
	let loading = $state(true);
	let createOpen = $state(false);
	let createName = $state('');
	let createBusy = $state(false);
	let busy = $state<Record<string, BusyAction | null>>({});
	let cleanupBusy = $state(false);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	const NAME_PATTERN = /^[a-z0-9][a-z0-9_-]*$/;

	function describeError(err: unknown): string {
		if (err instanceof AdminFetchError) return err.message;
		if (err instanceof Error) return err.message;
		return String(err);
	}

	async function loadContainers() {
		try {
			loading = true;
			const list = await adminGet<ContainerInfo[] | null>('containers');
			containers = list ?? [];
			// Reset rows we no longer have to clean stale entries.
			const next: Record<string, RowStatus> = {};
			for (const c of containers) {
				next[c.name] = statuses[c.name] ?? 'loading';
			}
			statuses = next;
			await refreshAllStatuses();
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeError(err) });
		} finally {
			loading = false;
		}
	}

	async function refreshStatus(name: string) {
		try {
			const res = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
			statuses = { ...statuses, [name]: res.status };
			scrapers = { ...scrapers, [name]: res.scraper };
		} catch (err) {
			statuses = { ...statuses, [name]: 'unknown' };
			// Status polling is best-effort; don't toast every failure.
			console.warn('status fetch failed for', name, err);
		}
	}

	async function refreshAllStatuses() {
		await Promise.all(containers.map((c) => refreshStatus(c.name)));
	}

	function startPolling() {
		stopPolling();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			refreshAllStatuses();
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
			// toast already shown by toastPromise
		} finally {
			createBusy = false;
		}
	}

	async function handleStart(c: ContainerInfo) {
		busy = { ...busy, [c.name]: 'start' };
		statuses = { ...statuses, [c.name]: 'loading' };
		try {
			await toastPromise(adminPost(`containers/${encodeURIComponent(c.name)}/start`), {
				loading: { title: 'Starting', description: c.name },
				success: { title: 'Started', description: c.name },
				errorTitle: 'Start failed'
			});
		} catch {
			// toast already shown
		} finally {
			busy = { ...busy, [c.name]: null };
			await refreshStatus(c.name);
		}
	}

	async function handleStop(c: ContainerInfo) {
		busy = { ...busy, [c.name]: 'stop' };
		statuses = { ...statuses, [c.name]: 'loading' };
		try {
			await toastPromise(adminPost(`containers/${encodeURIComponent(c.name)}/stop`), {
				loading: { title: 'Stopping', description: c.name },
				success: { title: 'Stopped', description: c.name },
				errorTitle: 'Stop failed'
			});
		} catch {
			// toast already shown
		} finally {
			busy = { ...busy, [c.name]: null };
			await refreshStatus(c.name);
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
			await loadContainers();
		} catch {
			// toast already shown
		} finally {
			cleanupBusy = false;
		}
	}

	async function handleDelete(c: ContainerInfo) {
		const targetStatus = statuses[c.name] ?? 'unknown';
		const runningWarning =
			targetStatus === 'running' ? ' It is currently running and will be force-stopped.' : '';
		const ok = await confirmToast({
			title: 'Delete container',
			description: `Permanently remove ${c.name}?${runningWarning}`,
			confirmLabel: 'Delete',
			type: targetStatus === 'running' ? 'error' : 'warning'
		});
		if (!ok) return;
		busy = { ...busy, [c.name]: 'delete' };
		try {
			await toastPromise(adminDelete(`containers/${encodeURIComponent(c.name)}`), {
				loading: { title: 'Deleting', description: c.name },
				success: { title: 'Deleted', description: c.name },
				errorTitle: 'Delete failed'
			});
			containers = containers.filter((x) => x.name !== c.name);
			const nextS = { ...statuses };
			delete nextS[c.name];
			statuses = nextS;
			const nextG = { ...scrapers };
			delete nextG[c.name];
			scrapers = nextG;
		} catch {
			// toast already shown
		} finally {
			busy = { ...busy, [c.name]: null };
		}
	}

	function statusClass(status: RowStatus): string {
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

	const rows = $derived<Row[]>(
		containers.map((c) => ({
			...c,
			status: statuses[c.name] ?? 'loading',
			scraper: scrapers[c.name] ?? null
		}))
	);

	onMount(() => {
		loadContainers();
		startPolling();
	});

	onDestroy(() => {
		stopPolling();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-6">
	<PageHeader
		title="Containers"
		description="Manage xemu + browser container pairs. Start one to auto-launch its scraper."
	>
		{#snippet actions()}
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => loadContainers()}
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
				+ New container
			</button>
		{/snippet}
	</PageHeader>

	{#snippet statusCell({ row }: { row: Row })}
		<span class={statusClass(row.status)}>{row.status}</span>
	{/snippet}
	{#snippet gameXboxCell({ row }: { row: Row })}
		{#if row.scraper}
			<div class="font-medium">{row.scraper.title || '—'}</div>
			<div class="text-xs text-surface-600-400">{row.scraper.xbox_name || '—'}</div>
		{:else}
			<span class="text-surface-600-400">—</span>
		{/if}
	{/snippet}
	{#snippet createdCell({ row }: { row: Row })}
		<span class="text-xs text-surface-600-400">{new Date(row.created).toLocaleString()}</span>
	{/snippet}
	{#snippet actionsCell({ row }: { row: Row })}
		{@const rowBusy = busy[row.name] ?? null}
		<div class="inline-flex gap-1">
			<a
				href={resolve('/containers/[name]', { name: row.name })}
				class="btn-icon preset-tonal btn-sm"
				aria-label="View"
				title="View"
			>
				<EyeIcon class="size-4" />
			</a>
			{#if row.status === 'running'}
				<button
					type="button"
					class="btn-icon preset-tonal-warning btn-sm"
					aria-label="Stop"
					title="Stop"
					onclick={() => handleStop(row)}
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
					onclick={() => handleStart(row)}
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
				onclick={() => handleDelete(row)}
				disabled={rowBusy !== null}
			>
				{#if rowBusy === 'delete'}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<Trash2Icon class="size-4" />
				{/if}
			</button>
		</div>
	{/snippet}

	<Card size="flush" class="overflow-x-auto">
		<DataTable
			{rows}
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
							key: 'game_xbox',
							label: 'Game / Xbox',
							cell: gameXboxCell,
							sortAccessor: (r) => r.scraper?.title ?? ''
						},
						{
							key: 'created',
							label: 'Created',
							cell: createdCell,
							sortAccessor: (r) => Date.parse(r.created)
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
			rowKey={(r) => r.name}
			density="comfortable"
			defaultSort={{ key: 'name', dir: 'asc' }}
			secondarySort={{ key: 'created', dir: 'desc' }}
			loading={loading && rows.length === 0}
			emptyMessage="No containers yet. Create one to get started."
		/>
	</Card>
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
