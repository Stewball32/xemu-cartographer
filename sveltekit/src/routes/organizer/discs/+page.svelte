<script lang="ts">
	// Discs — the managed disc-image library (renamed from "Games": the page
	// manages disc images, not game entries). Pipeline unchanged: drop .iso
	// files in the inbox, Ingest hashes + freezes + extracts each, dupes (same
	// hash) skip. The old row-actions + edit dialog became the same
	// master-detail as Offsets: pick a disc on the left, edit inline on the
	// right — everything from the old dialog is here, nothing moved off-page.
	import { onMount } from 'svelte';
	import {
		AlertTriangleIcon,
		DiscIcon,
		InboxIcon,
		LoaderIcon,
		MapIcon,
		RefreshCwIcon,
		SearchIcon,
		Trash2Icon,
		UploadIcon
	} from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import MasterDetail from '$lib/components/organizer/MasterDetail.svelte';
	import DraftBar from '$lib/components/organizer/DraftBar.svelte';
	import OrgToggle from '$lib/components/organizer/OrgToggle.svelte';
	import {
		listIsos,
		scanInbox,
		ingestInbox,
		listOffsetSets,
		updateIso,
		deleteIso,
		formatBytes,
		shortHash,
		type IngestResult,
		type InboxFile,
		type IsoEntry,
		type IsoRole,
		type OffsetSetInfo
	} from '$lib/utils/isos';

	let rows = $state<IsoEntry[]>([]);
	let inbox = $state<InboxFile[]>([]);
	let offsetSets = $state<OffsetSetInfo[]>([]);
	let loading = $state(true);
	let ingesting = $state(false);
	let filter = $state('');
	let openId = $state('');
	let busy = $state(false);

	// ── Draft (form vs saved row) ───────────────────────────────────────────
	interface DiscForm {
		name: string;
		description: string;
		role: IsoRole;
		allow_on_xbox: boolean;
		server_iso: string;
		offset_set: string;
	}
	let form = $state<DiscForm | null>(null);

	const open = $derived(rows.find((r) => r.id === openId) ?? null);

	function formFor(r: IsoEntry): DiscForm {
		return {
			name: r.name,
			description: r.description,
			role: r.role,
			allow_on_xbox: r.allow_on_xbox,
			server_iso: r.server_iso,
			offset_set: r.offset_set
		};
	}

	const dirty = $derived.by(() => {
		if (!open || !form) return false;
		const base = formFor(open);
		return (Object.keys(base) as (keyof DiscForm)[]).some((k) => form![k] !== base[k]);
	});

	function pick(r: IsoEntry) {
		openId = r.id;
		form = formFor(r);
	}

	// ── Grouping + filtering (list groups by role, per the redesign) ────────
	const filtered = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		if (!q) return rows;
		return rows.filter(
			(r) =>
				r.name.toLowerCase().includes(q) ||
				r.filename.toLowerCase().includes(q) ||
				r.title_id.toLowerCase().includes(q)
		);
	});
	const groups = $derived(
		(
			[
				['Play discs', 'play'],
				['Server builds', 'server'],
				['Shelved', 'shelved']
			] as const
		)
			.map(([label, role]) => ({ label, items: filtered.filter((r) => r.role === role) }))
			.filter((g) => g.items.length > 0)
	);

	const roleDefs: [IsoRole, string, string][] = [
		['play', 'Playable', 'Shows in player-facing pickers and syncs to stations.'],
		['server', 'Server only', 'Boots the xemu-cart host instance — never shown to players.'],
		['shelved', 'Shelved', 'Stays in the library, hidden everywhere.']
	];
	const roleDot: Record<IsoRole, string> = {
		play: 'bg-success-500',
		server: 'bg-tertiary-500',
		shelved: 'bg-surface-500'
	};

	function gameChip(titleID: string): string {
		if (titleID === '4D530004') return 'Halo: CE';
		if (titleID === '4D530064') return 'Halo 2';
		return titleID ? 'TID ' + titleID : '—';
	}

	const serverOptions = $derived(rows.filter((r) => r.role === 'server' && r.id !== openId));
	const byId = $derived(new Map(rows.map((r) => [r.id, r])));

	// ── Data flow ───────────────────────────────────────────────────────────
	async function load() {
		try {
			loading = true;
			rows = await listIsos();
			inbox = await scanInbox();
			offsetSets = await listOffsetSets();
			if (openId && !rows.some((r) => r.id === openId)) {
				openId = '';
				form = null;
			} else if (openId) {
				// refresh the form's baseline only when there's no pending draft
				if (!dirty) form = formFor(rows.find((r) => r.id === openId)!);
			}
		} catch (err) {
			toaster.error({ title: 'Load disc library failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	// ── Ingest + report dialog ──────────────────────────────────────────────
	let report = $state<IngestResult | null>(null);
	async function runIngest() {
		try {
			ingesting = true;
			report = await ingestInbox();
			await load();
		} catch (err) {
			toaster.error({ title: 'Ingest failed', description: describeAsyncError(err) });
		} finally {
			ingesting = false;
		}
	}

	async function save() {
		if (!open || !form) return;
		const name = form.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'Name is required.' });
			return;
		}
		try {
			busy = true;
			await toastPromise(
				updateIso(open.id, {
					name,
					description: form.description.trim(),
					role: form.role,
					allow_on_xbox: form.allow_on_xbox,
					server_iso: form.server_iso,
					offset_set: form.offset_set
				}),
				{
					loading: { title: 'Saving', description: name },
					success: { title: 'Saved', description: name },
					errorTitle: 'Save failed',
					errorDescription: (err) => (err instanceof Error ? err.message : 'Failed')
				}
			);
			await load();
		} catch {
			/* toast shown */
		} finally {
			busy = false;
		}
	}

	function discard() {
		if (open) form = formFor(open);
	}

	// ── Delete confirm (delete-to-replace flow) ─────────────────────────────
	let confirmDelete = $state(false);
	async function removeDisc() {
		if (!open) return;
		try {
			busy = true;
			await toastPromise(deleteIso(open.id), {
				loading: { title: 'Deleting', description: open.name },
				success: { title: 'Deleted', description: open.name },
				errorTitle: 'Delete failed'
			});
			confirmDelete = false;
			openId = '';
			form = null;
			await load();
		} catch {
			/* toast shown */
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage the library.' });
			return;
		}
		void load();
	});
</script>

{#snippet discRow(r: IsoEntry)}
	<button
		class="flex w-full items-center gap-2.5 border-b border-surface-100-900 px-3 py-2 text-left transition-colors hover:bg-surface-100-900
			{openId === r.id
			? 'border-l-2 border-l-primary-500 bg-primary-500/10'
			: 'border-l-2 border-l-transparent'}"
		onclick={() => pick(r)}
	>
		<span class="size-2 flex-none rounded-full {roleDot[r.role]}"></span>
		<span class="min-w-0 flex-1">
			<span class="flex items-center gap-1.5">
				<span class="truncate text-sm font-medium">{r.name}</span>
				{#if r.drift_detected}
					<span class="badge preset-tonal-error text-[10px]">drift</span>
				{/if}
				{#if r.role === 'server'}
					<span class="badge preset-tonal-tertiary text-[10px] uppercase">server</span>
				{/if}
			</span>
			<span class="block truncate font-mono text-xs opacity-50">
				{r.title_id || '—'} · {shortHash(r.content_hash)}
			</span>
		</span>
	</button>
{/snippet}

{#snippet listPanel()}
	<Card size="flush" class="flex min-w-0 flex-col overflow-hidden">
		<div class="flex items-center gap-2 border-b border-surface-200-800 p-3">
			<SearchIcon class="size-4 flex-none opacity-60" />
			<input
				type="search"
				class="w-full min-w-0 border-none bg-transparent text-sm outline-none"
				placeholder="Filter by name, file, or title id"
				bind:value={filter}
			/>
		</div>
		<div class="flex max-h-[65vh] flex-col overflow-y-auto">
			{#if loading}
				<div class="flex items-center gap-2 p-4 text-sm text-surface-500">
					<LoaderIcon class="size-4 animate-spin" /> Loading…
				</div>
			{:else if filtered.length === 0}
				<p class="p-4 text-sm text-surface-500">
					{rows.length === 0
						? 'No discs ingested yet — drop images in the inbox above and ingest.'
						: 'No discs match — clear the filter.'}
				</p>
			{:else}
				{#each groups as g (g.label)}
					<div
						class="px-3 pt-3 pb-1 text-[10px] font-bold tracking-widest text-surface-500 uppercase"
					>
						{g.label}
					</div>
					{#each g.items as r (r.id)}
						{@render discRow(r)}
					{/each}
				{/each}
			{/if}
		</div>
	</Card>
{/snippet}

{#snippet compactPills()}
	<div class="flex flex-wrap gap-1.5">
		{#each filtered as r (r.id)}
			<button
				class="rounded-full border px-3 py-1 text-xs transition-colors
					{openId === r.id
					? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
					: 'border-surface-500/25 bg-surface-100-900 text-surface-700-300'}"
				onclick={() => pick(r)}
			>
				{r.name}
			</button>
		{/each}
	</div>
{/snippet}

{#snippet detailPanel()}
	{#if !open || !form}
		<Card class="flex min-h-40 items-center justify-center">
			<p class="text-sm text-surface-500">Pick a disc to view and edit it.</p>
		</Card>
	{:else}
		<Card class="flex min-w-0 flex-col gap-4">
			<!-- header -->
			<div class="flex flex-wrap items-center gap-2">
				<DiscIcon class="size-4 opacity-60" />
				<h3 class="min-w-0 truncate h4">{open.name}</h3>
				<span class="badge preset-tonal">{gameChip(open.title_id)}</span>
				{#if open.drift_detected}
					<span class="badge preset-tonal-error">
						<AlertTriangleIcon class="size-3" /> drift detected
					</span>
				{/if}
				<span class="flex-1"></span>
				<button
					class="btn-icon preset-tonal-error btn-sm"
					title="Delete disc"
					onclick={() => (confirmDelete = true)}
				>
					<Trash2Icon class="size-4" />
				</button>
			</div>

			{#if open.drift_detected}
				<p class="rounded border border-error-500/40 bg-error-500/10 p-2 text-xs">
					Managed bytes no longer match the recorded hash — this disc won't boot or sync. Fix by
					re-ingest or delete-to-replace.
				</p>
			{/if}

			<!-- facts block (read-only provenance) -->
			<div
				class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 rounded bg-surface-100-900 p-3 text-xs"
			>
				<span class="text-surface-500">Original file</span>
				<span class="truncate font-mono">{open.filename}</span>
				<span class="text-surface-500">Title ID</span>
				<span class="font-mono">{open.title_id || '— pending extraction —'}</span>
				<span class="text-surface-500">Content hash</span>
				<span class="truncate font-mono">{shortHash(open.content_hash)}</span>
				<span class="text-surface-500">Extraction</span>
				<span class={open.extracted_ready ? 'text-success-600-400' : 'opacity-60'}>
					{open.extracted_ready
						? `extracted · ${formatBytes(open.footprint_bytes)}`
						: '— pending extraction —'}
				</span>
				<span class="text-surface-500">Parsed maps</span>
				<span>
					<!-- static route href, matching the NavPanel pattern -->
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a class="inline-flex items-center gap-1 anchor" href="/organizer/maps/">
						<MapIcon class="size-3.5" /> open the Maps page
					</a>
				</span>
			</div>

			<!-- editable fields -->
			<label class="label">
				<span class="label-text">Name</span>
				<input class="input" bind:value={form.name} />
			</label>
			<label class="label">
				<span class="label-text">Description <span class="opacity-50">(optional)</span></span>
				<input class="input" bind:value={form.description} />
			</label>

			<div class="flex flex-col gap-1.5">
				<span class="label-text text-sm">Role</span>
				<div class="flex flex-wrap gap-1.5">
					{#each roleDefs as [key, label] (key)}
						<button
							class="rounded-md border px-3 py-1.5 text-xs font-semibold transition-colors
								{form.role === key
								? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
								: 'border-surface-500/25 bg-surface-100-900 text-surface-600-400'}"
							onclick={() => (form!.role = key)}
						>
							{label}
						</button>
					{/each}
				</div>
				<span class="text-xs opacity-60">
					{roleDefs.find(([k]) => k === form!.role)?.[2]}
				</span>
			</div>

			<OrgToggle
				on={form.allow_on_xbox}
				label="Allow on Xboxes"
				onflip={() => (form!.allow_on_xbox = !form!.allow_on_xbox)}
			/>
			<p class="-mt-2 text-xs opacity-60">
				Marks this disc eligible for station HDDs regardless of role — the actual push selection
				happens at sync time.
			</p>

			{#if form.role !== 'server'}
				<label class="label">
					<span class="label-text">Server build <span class="opacity-50">(optional)</span></span>
					<select class="select" bind:value={form.server_iso}>
						<option value="">— none (host boots this disc) —</option>
						{#each serverOptions as r (r.id)}
							<option value={r.id}>{r.name}</option>
						{/each}
						{#if form.server_iso && !serverOptions.some((r) => r.id === form!.server_iso)}
							<!-- keep a non-server-role link visible rather than silently dropping it -->
							<option value={form.server_iso}>
								{byId.get(form.server_iso)?.name ?? form.server_iso}
							</option>
						{/if}
					</select>
					<span class="text-xs opacity-60">
						Only server-role discs are offered. “None” means the host boots this disc itself.
					</span>
				</label>
			{/if}

			<label class="label">
				<span class="label-text"
					>Memory offsets <span class="opacity-50">(modded builds)</span></span
				>
				<select class="select" bind:value={form.offset_set}>
					<option value="">— game baseline (stock build) —</option>
					{#each offsetSets as os (os.id)}
						<option value={os.id}>{os.id} ({os.count} offsets)</option>
					{/each}
				</select>
				<span class="text-xs opacity-60">
					Which offset set the scraper binds for this build. Baseline unless it's a modded build
					with a mapped set.
				</span>
			</label>

			<DraftBar {dirty} {busy} onsave={save} ondiscard={discard} />
		</Card>
	{/if}
{/snippet}

<div class="flex flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Discs"
		description="Disc images, managed: drop .iso files into the inbox, then ingest — each is hashed, frozen read-only, and extracted. A disc's role decides where it shows; its server build boots the xemu-cart host."
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

	<MasterDetail
		open={!!open}
		onback={() => {
			openId = '';
			form = null;
		}}
		backLabel="All discs"
		list={listPanel}
		compact={compactPills}
		detail={detailPanel}
	/>
</div>

<!-- Ingest report -->
<Dialog open={!!report} onClose={() => (report = null)} title="Ingest complete" size="md">
	{#if report}
		<div class="flex flex-col gap-2">
			<div class="flex flex-wrap gap-2">
				{#if report.ingested.length}
					<span class="badge preset-tonal-success">{report.ingested.length} ingested</span>
				{/if}
				{#if report.skipped.length}
					<span class="badge preset-tonal-surface">{report.skipped.length} skipped</span>
				{/if}
				{#if report.errors.length}
					<span class="badge preset-tonal-error">{report.errors.length} error(s)</span>
				{/if}
			</div>
			{#each report.ingested as it (it.filename)}
				<div class="flex items-center gap-2 rounded border border-surface-500/20 bg-black/20 p-2">
					<span class="size-2 flex-none rounded-full bg-success-500"></span>
					<span class="min-w-0 flex-1 truncate font-mono text-xs">{it.filename}</span>
					<span class="text-xs opacity-60">ingested · extracting…</span>
				</div>
			{/each}
			{#each report.skipped as it (it.filename)}
				<div class="flex items-center gap-2 rounded border border-surface-500/20 bg-black/20 p-2">
					<span class="size-2 flex-none rounded-full bg-surface-500"></span>
					<span class="min-w-0 flex-1 truncate font-mono text-xs">{it.filename}</span>
					<span class="text-xs opacity-60">
						skipped — {it.reason}{it.dup_of ? ` (${it.dup_of})` : ''}
					</span>
				</div>
			{/each}
			{#each report.errors as msg (msg)}
				<p class="text-xs text-error-500">{msg}</p>
			{/each}
			<p class="text-xs opacity-60">
				New discs land shelved with baseline offsets — set role and bindings in the detail, then
				save.
			</p>
		</div>
	{/if}
	{#snippet footer()}
		<button class="btn preset-tonal" onclick={() => (report = null)}>Close</button>
	{/snippet}
</Dialog>

<!-- Delete confirm (delete-to-replace) -->
<Dialog open={confirmDelete} onClose={() => (confirmDelete = false)} title="Delete disc" size="sm">
	<p class="flex items-start gap-2 text-sm">
		<AlertTriangleIcon class="mt-0.5 size-4 flex-none text-warning-500" />
		<span>
			Remove <strong>{open?.name}</strong> from the catalog AND delete its managed disc + extracted tree
			from disk? This is the delete-to-replace flow. Any disc linking this as its server build falls back
			to its own disc.
		</span>
	</p>
	{#snippet footer()}
		<button class="btn preset-tonal" onclick={() => (confirmDelete = false)} disabled={busy}>
			Cancel
		</button>
		<button class="btn preset-tonal-error" onclick={removeDisc} disabled={busy}>
			{#if busy}<LoaderIcon class="size-4 animate-spin" />{/if}
			<span>Delete disc</span>
		</button>
	{/snippet}
</Dialog>
