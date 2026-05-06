<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { PlayIcon, SquareIcon, Trash2Icon, RefreshCwIcon, EyeIcon } from '@lucide/svelte';
	import { adminGet, adminPost, adminDelete, AdminFetchError } from '$lib/utils/admin-api';
	import { toaster } from '$lib/stores/toaster';
	import type {
		ContainerInfo,
		ContainerStatus,
		ContainerDetail,
		InstanceState
	} from '$lib/types/containers';

	type RowStatus = ContainerStatus | 'loading' | string;

	let containers = $state<ContainerInfo[]>([]);
	let statuses = $state<Record<string, RowStatus>>({});
	let scrapers = $state<Record<string, InstanceState | null>>({});
	let loading = $state(true);
	let createOpen = $state(false);
	let createName = $state('');
	let createBusy = $state(false);
	let confirmDelete = $state<ContainerInfo | null>(null);
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
			const created = await adminPost<ContainerInfo>('containers', { name });
			containers = [created, ...containers];
			statuses = { ...statuses, [created.name]: 'created' };
			createName = '';
			createOpen = false;
			toaster.success({ title: 'Container created', description: created.name });
		} catch (err) {
			toaster.error({ title: 'Create failed', description: describeError(err) });
		} finally {
			createBusy = false;
		}
	}

	async function handleStart(c: ContainerInfo) {
		statuses = { ...statuses, [c.name]: 'loading' };
		try {
			await adminPost(`containers/${encodeURIComponent(c.name)}/start`);
			toaster.success({ title: 'Starting', description: c.name });
			await refreshStatus(c.name);
		} catch (err) {
			toaster.error({ title: 'Start failed', description: describeError(err) });
			await refreshStatus(c.name);
		}
	}

	async function handleStop(c: ContainerInfo) {
		statuses = { ...statuses, [c.name]: 'loading' };
		try {
			await adminPost(`containers/${encodeURIComponent(c.name)}/stop`);
			toaster.success({ title: 'Stopping', description: c.name });
			await refreshStatus(c.name);
		} catch (err) {
			toaster.error({ title: 'Stop failed', description: describeError(err) });
			await refreshStatus(c.name);
		}
	}

	async function handleDelete(c: ContainerInfo) {
		try {
			await adminDelete(`containers/${encodeURIComponent(c.name)}`);
			containers = containers.filter((x) => x.name !== c.name);
			const nextS = { ...statuses };
			delete nextS[c.name];
			statuses = nextS;
			const nextG = { ...scrapers };
			delete nextG[c.name];
			scrapers = nextG;
			confirmDelete = null;
			toaster.success({ title: 'Deleted', description: c.name });
		} catch (err) {
			toaster.error({ title: 'Delete failed', description: describeError(err) });
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

	onMount(() => {
		loadContainers();
		startPolling();
	});

	onDestroy(() => {
		stopPolling();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-6">
	<header class="flex flex-wrap items-center justify-between gap-4">
		<div>
			<h1 class="h2">Containers</h1>
			<p class="text-sm text-surface-600-400">
				Manage xemu + browser container pairs. Start one to auto-launch its scraper.
			</p>
		</div>
		<div class="flex gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => loadContainers()}
				disabled={loading}
				aria-label="Refresh"
			>
				<RefreshCwIcon class="size-4" />
				<span>Refresh</span>
			</button>
			<button type="button" class="btn preset-filled" onclick={() => (createOpen = true)}>
				+ New container
			</button>
		</div>
	</header>

	<div class="overflow-x-auto card p-0">
		<table class="table-hover table w-full">
			<thead>
				<tr>
					<th>Name</th>
					<th>Status</th>
					<th>Game / Xbox</th>
					<th>Created</th>
					<th class="text-right">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#if loading && containers.length === 0}
					<tr>
						<td colspan="5" class="text-center text-surface-600-400">Loading…</td>
					</tr>
				{:else if containers.length === 0}
					<tr>
						<td colspan="5" class="text-center text-surface-600-400">
							No containers yet. Create one to get started.
						</td>
					</tr>
				{:else}
					{#each containers as c (c.name)}
						{@const status = statuses[c.name] ?? 'loading'}
						{@const isRunning = status === 'running'}
						{@const scraper = scrapers[c.name]}
						<tr>
							<td class="font-medium">{c.name}</td>
							<td>
								<span class={statusClass(status)}>{status}</span>
							</td>
							<td class="text-xs">
								{#if scraper}
									<div class="font-medium">{scraper.title || '—'}</div>
									<div class="text-surface-600-400">{scraper.xbox_name || '—'}</div>
								{:else}
									<span class="text-surface-600-400">—</span>
								{/if}
							</td>
							<td class="text-xs text-surface-600-400">
								{new Date(c.created).toLocaleString()}
							</td>
							<td class="text-right">
								<div class="inline-flex gap-1">
									<a
										href={resolve('/containers/[name]', { name: c.name })}
										class="btn-icon preset-tonal btn-sm"
										aria-label="View"
										title="View"
									>
										<EyeIcon class="size-4" />
									</a>
									{#if isRunning}
										<button
											type="button"
											class="btn-icon preset-tonal-warning btn-sm"
											aria-label="Stop"
											title="Stop"
											onclick={() => handleStop(c)}
										>
											<SquareIcon class="size-4" />
										</button>
									{:else}
										<button
											type="button"
											class="btn-icon preset-tonal-success btn-sm"
											aria-label="Start"
											title="Start"
											onclick={() => handleStart(c)}
										>
											<PlayIcon class="size-4" />
										</button>
									{/if}
									<button
										type="button"
										class="btn-icon preset-tonal-error btn-sm"
										aria-label="Delete"
										title="Delete"
										onclick={() => (confirmDelete = c)}
									>
										<Trash2Icon class="size-4" />
									</button>
								</div>
							</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>
</div>

{#if createOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget && !createBusy) createOpen = false;
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape' && !createBusy) createOpen = false;
		}}
	>
		<div class="w-full max-w-md card p-6">
			<h2 class="mb-4 h3">New container</h2>
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
						{createBusy ? 'Creating…' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if confirmDelete}
	{@const target = confirmDelete}
	{@const targetStatus = statuses[target.name] ?? 'unknown'}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget) confirmDelete = null;
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') confirmDelete = null;
		}}
	>
		<div class="w-full max-w-md card p-6">
			<h2 class="mb-2 h3">Delete container</h2>
			<p class="mb-4 text-sm text-surface-600-400">
				Permanently remove <strong>{target.name}</strong>?
				{#if targetStatus === 'running'}
					<span class="mt-2 block text-error-500">
						This container is currently running. It will be force-stopped before deletion.
					</span>
				{/if}
			</p>
			<div class="flex justify-end gap-2">
				<button type="button" class="btn preset-tonal" onclick={() => (confirmDelete = null)}>
					Cancel
				</button>
				<button
					type="button"
					class="btn preset-filled-error-500"
					onclick={() => handleDelete(target)}
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}
