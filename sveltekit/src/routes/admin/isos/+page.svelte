<script lang="ts">
	// Admin ISO library / organizer.
	//
	// The on-disk game-ISO library (podman CONTAINERS_ISO_DIR) surfaced as a
	// manageable catalog: scan the disc images present on the host, register one
	// as a catalog entry (name + title_id + availability), give a game an
	// optional dedicated SERVER build (server_iso → another catalog entry), and
	// watch each disc's extract-cache status (the tree synced to consoles).
	//
	// Write-once immutability: a row's underlying library file can't be changed
	// (isos_immutable) — metadata + server_iso stay editable; to point at a
	// different disc, delete the row and register the new file.
	//
	// /admin/+layout.ts enforces isAdmin; this page assumes it.

	import { onMount } from 'svelte';
	import {
		DiscIcon,
		HardDriveIcon,
		LoaderIcon,
		PencilIcon,
		PlusIcon,
		RefreshCwIcon,
		SearchIcon,
		ServerIcon,
		Trash2Icon,
		CheckCircle2Icon,
		ClockIcon
	} from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { SortState } from '$lib/components/ui/data-table';
	import {
		listIsos,
		scanLibrary,
		createIso,
		updateIso,
		deleteIso,
		formatBytes,
		IsoApiError,
		type IsoEntry,
		type LibraryFile
	} from '$lib/utils/isos';

	let rows = $state<IsoEntry[]>([]);
	let libraryFiles = $state<LibraryFile[]>([]);
	let libraryDir = $state('');
	let libraryUnavailable = $state(false); // container subsystem off (503)
	let loading = $state(true);
	let filter = $state('');
	let sort = $state<SortState>({ key: 'name', dir: 'asc' });
	let deleteBusy = $state<Record<string, boolean>>({});

	// name lookup for rendering a server_iso id as its entry name.
	const byId = $derived(new Map(rows.map((r) => [r.id, r])));

	// ── Register dialog ──────────────────────────────────────────────────────
	let regOpen = $state(false);
	let regBusy = $state(false);
	let reg = $state({
		filename: '',
		name: '',
		title_id: '',
		description: '',
		available: true,
		server_iso: ''
	});

	// ── Edit dialog ──────────────────────────────────────────────────────────
	let editOpen = $state(false);
	let editBusy = $state(false);
	let editId = $state('');
	let editFilename = $state('');
	let edit = $state({
		name: '',
		title_id: '',
		description: '',
		available: true,
		server_iso: ''
	});

	const unregistered = $derived(libraryFiles.filter((f) => !f.registered));

	async function load() {
		try {
			loading = true;
			rows = await listIsos();
			try {
				const lib = await scanLibrary();
				libraryDir = lib.dir;
				libraryFiles = lib.files;
				libraryUnavailable = false;
			} catch (err) {
				// 503 = container subsystem disabled; catalog still works.
				libraryUnavailable = err instanceof IsoApiError && err.status === 503;
				if (!libraryUnavailable) throw err;
				libraryFiles = [];
			}
		} catch (err) {
			toaster.error({ title: 'Load ISO library failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function openRegister(filename = '') {
		reg = { filename, name: '', title_id: '', description: '', available: true, server_iso: '' };
		regOpen = true;
	}

	async function saveRegister() {
		const name = reg.name.trim();
		const filename = reg.filename.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'Name is required.' });
			return;
		}
		if (!filename) {
			toaster.error({ title: 'Invalid', description: 'Pick a disc file to register.' });
			return;
		}
		try {
			regBusy = true;
			await toastPromise(
				createIso({
					name,
					filename,
					title_id: reg.title_id.trim(),
					description: reg.description.trim(),
					available: reg.available,
					server_iso: reg.server_iso || undefined
				}),
				{
					loading: { title: 'Registering', description: name },
					success: { title: 'Registered', description: name },
					errorTitle: 'Register failed',
					errorDescription: (err) => (err instanceof Error ? err.message : 'Failed')
				}
			);
			regOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			regBusy = false;
		}
	}

	function openEdit(row: IsoEntry) {
		editId = row.id;
		editFilename = row.filename;
		edit = {
			name: row.name,
			title_id: row.title_id,
			description: row.description,
			available: row.available,
			server_iso: row.server_iso
		};
		editOpen = true;
	}

	async function saveEdit() {
		const name = edit.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'Name is required.' });
			return;
		}
		try {
			editBusy = true;
			await toastPromise(
				updateIso(editId, {
					name,
					title_id: edit.title_id.trim(),
					description: edit.description.trim(),
					available: edit.available,
					server_iso: edit.server_iso // "" clears the link
				}),
				{
					loading: { title: 'Saving', description: name },
					success: { title: 'Saved', description: name },
					errorTitle: 'Save failed',
					errorDescription: (err) => (err instanceof Error ? err.message : 'Failed')
				}
			);
			editOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			editBusy = false;
		}
	}

	async function removeEntry(row: IsoEntry) {
		const ok = await confirmToast({
			title: 'Delete catalog entry',
			description: `Remove "${row.name}" (${row.filename}) from the catalog? The disc file on disk is left untouched — this only unregisters it. Any game linking this as its server build will fall back to its own disc.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		deleteBusy = { ...deleteBusy, [row.id]: true };
		try {
			await toastPromise(deleteIso(row.id), {
				loading: { title: 'Deleting', description: row.name },
				success: { title: 'Deleted', description: row.name },
				errorTitle: 'Delete failed'
			});
			await load();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...deleteBusy };
			delete nx[row.id];
			deleteBusy = nx;
		}
	}

	const filtered = $derived.by<IsoEntry[]>(() => {
		const q = filter.trim().toLowerCase();
		const base = !q
			? rows
			: rows.filter(
					(r) =>
						r.name.toLowerCase().includes(q) ||
						r.filename.toLowerCase().includes(q) ||
						r.title_id.toLowerCase().includes(q)
				);
		const s = sort;
		if (!s) return base;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...base].sort((a, b) => {
			const av = String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv = String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	onMount(() => {
		if (!auth.token) {
			toaster.error({
				title: 'Not authenticated',
				description: 'Log in to manage the ISO library.'
			});
			return;
		}
		void load();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="ISO library"
		description="Register game discs from the on-disk library, give a game an optional dedicated server build, and track extraction status. A game's own disc is the client build synced to real Xboxes; its optional server build boots the xemu-cart host instance."
	/>

	<!-- On-disk library scan -->
	<Card>
		<div class="flex items-center gap-2">
			<HardDriveIcon class="size-4 opacity-70" />
			<h3 class="h4">On-disk library</h3>
		</div>
		{#if libraryUnavailable}
			<p class="text-sm opacity-70">
				The container subsystem is disabled (<code>CONTAINERS_ENABLED=false</code>), so the host
				library can't be scanned. You can still edit and delete existing catalog entries, but
				registering a new disc needs the library scan to confirm the file exists.
			</p>
		{:else}
			<p class="text-xs opacity-60">
				{#if libraryDir}<span class="font-mono">{libraryDir}</span>{/if}
			</p>
			{#if unregistered.length === 0}
				<p class="text-sm opacity-70">
					Every disc on disk is registered. Drop a new <code>.iso</code> into the library dir and refresh
					to register it.
				</p>
			{:else}
				<ul class="flex flex-col gap-1">
					{#each unregistered as f (f.filename)}
						<li
							class="flex items-center justify-between gap-2 rounded px-2 py-1 hover:bg-surface-200-800"
						>
							<span class="flex items-center gap-2 truncate">
								<DiscIcon class="size-4 shrink-0 opacity-60" />
								<span class="truncate font-mono text-sm">{f.filename}</span>
							</span>
							<button class="btn preset-filled btn-sm" onclick={() => openRegister(f.filename)}>
								<PlusIcon class="size-4" />
								<span>Register</span>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</Card>

	<!-- Catalog toolbar -->
	<div class="flex items-center justify-between gap-2">
		<div class="input-group flex-1 grid-cols-[auto_1fr]">
			<div class="ig-cell preset-tonal"><SearchIcon class="size-4" /></div>
			<input
				type="search"
				class="ig-input"
				placeholder="Filter by name, file, or title id"
				bind:value={filter}
			/>
		</div>
		<button class="btn preset-tonal" onclick={() => load()} disabled={loading} aria-label="Refresh">
			{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
					class="size-4"
				/>{/if}
			<span>Refresh</span>
		</button>
		<button class="btn preset-filled" onclick={() => openRegister()} disabled={libraryUnavailable}>
			<PlusIcon class="size-4" />
			<span>Register disc</span>
		</button>
	</div>

	<!-- Row cells -->
	{#snippet nameCell({ row }: { row: IsoEntry })}
		<div class="flex flex-col">
			<span class="font-medium">{row.name}</span>
			<span class="font-mono text-xs opacity-60">{row.filename}</span>
		</div>
	{/snippet}
	{#snippet titleCell({ row }: { row: IsoEntry })}
		{#if row.title_id}<span class="font-mono text-xs">{row.title_id}</span>{:else}<span
				class="text-xs opacity-40">—</span
			>{/if}
	{/snippet}
	{#snippet availableCell({ row }: { row: IsoEntry })}
		{#if row.available}
			<span class="badge preset-tonal-success">available</span>
		{:else}
			<span class="badge preset-tonal-surface">hidden</span>
		{/if}
	{/snippet}
	{#snippet serverCell({ row }: { row: IsoEntry })}
		{#if row.server_iso}
			<span class="badge preset-tonal-primary">
				<ServerIcon class="size-3" />
				<span class="truncate">{byId.get(row.server_iso)?.name ?? row.server_iso}</span>
			</span>
		{:else}
			<span class="text-xs opacity-40">—</span>
		{/if}
	{/snippet}
	{#snippet extractCell({ row }: { row: IsoEntry })}
		{#if row.extracted_ready}
			<span class="inline-flex items-center gap-1 text-success-600-400">
				<CheckCircle2Icon class="size-4" />
				<span class="text-xs">{formatBytes(row.footprint_bytes)}</span>
			</span>
		{:else}
			<span class="inline-flex items-center gap-1 opacity-60">
				<ClockIcon class="size-4" />
				<span class="text-xs">pending</span>
			</span>
		{/if}
	{/snippet}
	{#snippet actionsCell({ row }: { row: IsoEntry })}
		<div
			role="presentation"
			class="inline-flex items-center justify-end gap-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<button class="btn-icon preset-tonal btn-sm" title="Edit" onclick={() => openEdit(row)}>
				<PencilIcon class="size-4" />
			</button>
			<button
				class="btn-icon preset-tonal-error btn-sm"
				title="Delete"
				onclick={() => removeEntry(row)}
				disabled={!!deleteBusy[row.id]}
			>
				{#if deleteBusy[row.id]}<LoaderIcon class="size-4 animate-spin" />{:else}<Trash2Icon
						class="size-4"
					/>{/if}
			</button>
		</div>
	{/snippet}

	<Card size="flush" class="overflow-x-auto">
		<DataTable
			rows={filtered}
			{sort}
			onSortChange={(s) => (sort = s)}
			{loading}
			keyFor={(c) => c.key}
			rowKey={(r) => r.id}
			emptyMessage="no discs registered yet — register one from the on-disk library above"
			groups={[
				{
					columns: [
						{ key: 'name', label: 'Game', sortable: true, cell: nameCell },
						{ key: 'title_id', label: 'Title ID', sortable: true, cell: titleCell },
						{ key: 'available', label: 'Player', sortable: true, cell: availableCell },
						{ key: 'server_iso', label: 'Server build', cell: serverCell },
						{ key: 'extracted_ready', label: 'Extraction', cell: extractCell },
						{ key: 'actions', label: '', align: 'right', cell: actionsCell }
					]
				}
			]}
		/>
	</Card>
</div>

<!-- Register dialog -->
<Dialog open={regOpen} onClose={() => (regOpen = false)} title="Register disc" size="md">
	<div class="flex flex-col gap-3">
		<label class="label">
			<span class="label-text">Disc file</span>
			{#if reg.filename && libraryFiles.every((f) => f.filename !== reg.filename)}
				<input class="input font-mono" bind:value={reg.filename} readonly />
			{:else}
				<select class="select" bind:value={reg.filename}>
					<option value="" disabled>— pick a disc on disk —</option>
					{#each unregistered as f (f.filename)}
						<option value={f.filename}>{f.filename}</option>
					{/each}
				</select>
			{/if}
			<span class="text-xs opacity-60"
				>The library file is fixed after registering (write-once).</span
			>
		</label>
		<label class="label">
			<span class="label-text">Name</span>
			<input class="input" bind:value={reg.name} placeholder="e.g. Halo: Combat Evolved" />
		</label>
		<label class="label">
			<span class="label-text">Title ID <span class="opacity-50">(optional)</span></span>
			<input class="input font-mono" bind:value={reg.title_id} placeholder="e.g. 4d530064" />
		</label>
		<label class="label">
			<span class="label-text">Description <span class="opacity-50">(optional)</span></span>
			<input class="input" bind:value={reg.description} />
		</label>
		<label class="label">
			<span class="label-text">Server build <span class="opacity-50">(optional)</span></span>
			<select class="select" bind:value={reg.server_iso}>
				<option value="">— none (host boots this disc) —</option>
				{#each rows as r (r.id)}
					<option value={r.id}>{r.name} <span>({r.filename})</span></option>
				{/each}
			</select>
			<span class="text-xs opacity-60">
				If set, a xemu-cart host instance boots that build; real Xboxes still get this disc.
			</span>
		</label>
		<label class="flex items-center gap-2">
			<input type="checkbox" class="checkbox" bind:checked={reg.available} />
			<span class="text-sm">Available to players (shows in the pick-a-game list)</span>
		</label>
	</div>
	{#snippet footer()}
		<button class="btn preset-tonal" onclick={() => (regOpen = false)} disabled={regBusy}
			>Cancel</button
		>
		<button class="btn preset-filled" onclick={saveRegister} disabled={regBusy}>
			{#if regBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
			<span>Register</span>
		</button>
	{/snippet}
</Dialog>

<!-- Edit dialog -->
<Dialog open={editOpen} onClose={() => (editOpen = false)} title="Edit catalog entry" size="md">
	<div class="flex flex-col gap-3">
		<label class="label">
			<span class="label-text">Disc file <span class="opacity-50">(write-once)</span></span>
			<input class="input font-mono" value={editFilename} readonly />
			<span class="text-xs opacity-60"
				>To point at a different disc, delete this entry and register the new file.</span
			>
		</label>
		<label class="label">
			<span class="label-text">Name</span>
			<input class="input" bind:value={edit.name} />
		</label>
		<label class="label">
			<span class="label-text">Title ID <span class="opacity-50">(optional)</span></span>
			<input class="input font-mono" bind:value={edit.title_id} />
		</label>
		<label class="label">
			<span class="label-text">Description <span class="opacity-50">(optional)</span></span>
			<input class="input" bind:value={edit.description} />
		</label>
		<label class="label">
			<span class="label-text">Server build <span class="opacity-50">(optional)</span></span>
			<select class="select" bind:value={edit.server_iso}>
				<option value="">— none (host boots this disc) —</option>
				{#each rows as r (r.id)}
					{#if r.id !== editId}
						<option value={r.id}>{r.name} ({r.filename})</option>
					{/if}
				{/each}
			</select>
		</label>
		<label class="flex items-center gap-2">
			<input type="checkbox" class="checkbox" bind:checked={edit.available} />
			<span class="text-sm">Available to players</span>
		</label>
	</div>
	{#snippet footer()}
		<button class="btn preset-tonal" onclick={() => (editOpen = false)} disabled={editBusy}
			>Cancel</button
		>
		<button class="btn preset-filled" onclick={saveEdit} disabled={editBusy}>
			{#if editBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
			<span>Save</span>
		</button>
	{/snippet}
</Dialog>
