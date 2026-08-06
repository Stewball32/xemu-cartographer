<script lang="ts">
	// ISO / Game management (organizer surface).
	//
	// Ingest model: drop disc images into the tier-root inbox (inbox/isos/), then
	// "Ingest" — each becomes a managed <id>.iso (hashed, frozen read-only,
	// extracted), deduped by content hash. The display name is decoupled from the
	// file and freely editable. A game may link an optional server build
	// (server_iso). Managed bytes are re-verified on boot + before boot/sync; a
	// drifted disc is flagged and forced unavailable.
	//
	// /organizer/+layout.ts enforces organizer-or-admin; the /api/admin/isos
	// routes enforce the same server-side. (This page replaced the old
	// game_titles XBE uploader — discs now arrive via the ingest inbox.)

	import { onMount } from 'svelte';
	import {
		AlertTriangleIcon,
		CheckCircle2Icon,
		ClockIcon,
		DiscIcon,
		InboxIcon,
		LoaderIcon,
		PencilIcon,
		RefreshCwIcon,
		SearchIcon,
		ServerIcon,
		Trash2Icon,
		UploadIcon
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
		scanInbox,
		ingestInbox,
		listOffsetSets,
		updateIso,
		deleteIso,
		formatBytes,
		shortHash,
		type IsoEntry,
		type InboxFile,
		type OffsetSetInfo
	} from '$lib/utils/isos';

	let rows = $state<IsoEntry[]>([]);
	let inbox = $state<InboxFile[]>([]);
	let offsetSets = $state<OffsetSetInfo[]>([]);
	let loading = $state(true);
	let ingesting = $state(false);
	let filter = $state('');
	let sort = $state<SortState>({ key: 'name', dir: 'asc' });
	let deleteBusy = $state<Record<string, boolean>>({});

	const byId = $derived(new Map(rows.map((r) => [r.id, r])));

	// ── Edit dialog ──────────────────────────────────────────────────────────
	let editOpen = $state(false);
	let editBusy = $state(false);
	let editId = $state('');
	let editFilename = $state('');
	let editTitleID = $state('');
	let edit = $state({
		name: '',
		description: '',
		available: true,
		server_iso: '',
		offset_set: ''
	});

	async function load() {
		try {
			loading = true;
			rows = await listIsos();
			inbox = await scanInbox();
			offsetSets = await listOffsetSets();
		} catch (err) {
			toaster.error({ title: 'Load ISO library failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	async function runIngest() {
		try {
			ingesting = true;
			const res = await ingestInbox();
			const parts: string[] = [];
			if (res.ingested.length) parts.push(`${res.ingested.length} ingested`);
			if (res.skipped.length) parts.push(`${res.skipped.length} skipped (dupes)`);
			if (res.errors.length) parts.push(`${res.errors.length} error(s)`);
			if (res.errors.length) {
				toaster.error({ title: 'Ingest finished with errors', description: res.errors.join('; ') });
			} else {
				toaster.success({
					title: 'Ingest complete',
					description: parts.join(', ') || 'nothing to ingest'
				});
			}
			await load();
		} catch (err) {
			toaster.error({ title: 'Ingest failed', description: describeAsyncError(err) });
		} finally {
			ingesting = false;
		}
	}

	function openEdit(row: IsoEntry) {
		editId = row.id;
		editFilename = row.filename;
		editTitleID = row.title_id;
		edit = {
			name: row.name,
			description: row.description,
			available: row.available,
			server_iso: row.server_iso,
			offset_set: row.offset_set
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
					description: edit.description.trim(),
					available: edit.available,
					server_iso: edit.server_iso,
					offset_set: edit.offset_set
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
			title: 'Delete disc',
			description: `Remove "${row.name}" from the catalog AND delete its managed disc + extracted tree from disk? This is the delete-to-replace flow. Any game linking this as its server build will fall back to its own disc.`,
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
		title="Games"
		description="Drop disc images into the inbox, then ingest them into the managed library — each is hashed, frozen read-only, and extracted. A game's own disc is the client build synced to real Xboxes; its optional server build boots the xemu-cart host instance."
	/>

	<!-- Inbox / ingest -->
	<Card>
		<div class="flex items-center gap-2">
			<InboxIcon class="size-4 opacity-70" />
			<h3 class="h4">Inbox</h3>
		</div>
		<p class="text-xs opacity-60">
			Staged in <span class="font-mono">inbox/isos/</span>. Ingest moves each into the managed
			library as <span class="font-mono">&lt;id&gt;.iso</span>, hashes + freezes it, and extracts.
			Duplicate content (same hash) is skipped.
		</p>
		{#if inbox.length === 0}
			<p class="text-sm opacity-70">
				No pending files. Drop <code>.iso</code> images into the inbox and refresh.
			</p>
		{:else}
			<ul class="flex flex-col gap-1">
				{#each inbox as f (f.filename)}
					<li
						class="flex items-center justify-between gap-2 rounded px-2 py-1 hover:bg-surface-200-800"
					>
						<span class="flex items-center gap-2 truncate">
							<DiscIcon class="size-4 shrink-0 opacity-60" />
							<span class="truncate font-mono text-sm">{f.filename}</span>
						</span>
						<span class="text-xs tabular-nums opacity-60">{formatBytes(f.size)}</span>
					</li>
				{/each}
			</ul>
		{/if}
		<div class="flex items-center gap-2">
			<button class="btn preset-tonal" onclick={() => load()} disabled={loading || ingesting}>
				{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
						class="size-4"
					/>{/if}
				<span>Refresh</span>
			</button>
			<button
				class="btn preset-filled"
				onclick={runIngest}
				disabled={ingesting || inbox.length === 0}
			>
				{#if ingesting}<LoaderIcon class="size-4 animate-spin" />{:else}<UploadIcon
						class="size-4"
					/>{/if}
				<span>Ingest {inbox.length || ''} file{inbox.length === 1 ? '' : 's'}</span>
			</button>
		</div>
	</Card>

	<!-- Catalog toolbar -->
	<div class="input-group grid-cols-[auto_1fr]">
		<div class="ig-cell preset-tonal"><SearchIcon class="size-4" /></div>
		<input
			type="search"
			class="ig-input"
			placeholder="Filter by name, file, or title id"
			bind:value={filter}
		/>
	</div>

	<!-- Row cells -->
	{#snippet nameCell({ row }: { row: IsoEntry })}
		<div class="flex flex-col">
			<span class="flex items-center gap-1.5 font-medium">
				{row.name}
				{#if row.drift_detected}
					<span
						class="badge preset-tonal-error"
						title="Managed bytes no longer match the recorded hash"
					>
						<AlertTriangleIcon class="size-3" /> drift
					</span>
				{/if}
			</span>
			<span class="font-mono text-xs opacity-60">{shortHash(row.content_hash)}</span>
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
			emptyMessage="no discs ingested yet — drop images in the inbox above and ingest"
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

<!-- Edit dialog -->
<Dialog open={editOpen} onClose={() => (editOpen = false)} title="Edit disc" size="md">
	<div class="flex flex-col gap-3">
		<label class="label">
			<span class="label-text">Original file <span class="opacity-50">(provenance)</span></span>
			<input class="input font-mono" value={editFilename} readonly />
			<span class="text-xs opacity-60"
				>The managed disc is stored by ID; delete this entry to replace the file.</span
			>
		</label>
		<label class="label">
			<span class="label-text">Name</span>
			<input class="input" bind:value={edit.name} />
		</label>
		<label class="label">
			<span class="label-text"
				>Title ID <span class="opacity-50">(read from the disc's XBE)</span></span
			>
			<input class="input font-mono" value={editTitleID || '— pending extraction —'} readonly />
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
						<option value={r.id}>{r.name}</option>
					{/if}
				{/each}
			</select>
		</label>
		<label class="label">
			<span class="label-text">Memory offsets <span class="opacity-50">(modded builds)</span></span>
			<select class="select" bind:value={edit.offset_set}>
				<option value="">— game baseline (stock build) —</option>
				{#each offsetSets as os (os.id)}
					<option value={os.id}>{os.id} ({os.game}, {os.count} offsets)</option>
				{/each}
			</select>
			<span class="text-xs opacity-60">
				Which offset set the scraper binds for this build. Leave on baseline unless this disc is a
				modded build with a mapped set.
			</span>
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
