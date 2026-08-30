<script lang="ts">
	// Offsets — the memory-offset set library. An offset set is a named map of
	// addresses the scraper reads out of a running build; stats only flow when
	// the disc's build matches the addresses being read. Each game ships an
	// embedded baseline; a modded build (NHE) binds its own. Binding lives on
	// the DISC (Discs → Memory offsets), never here — this page is the library:
	// import (the only way a set is born — an offsetmap export from the hunting
	// rig; nothing is hand-edited), download (byte-identical re-export), and
	// delete-with-migration.
	import { onMount } from 'svelte';
	import {
		DownloadIcon,
		LoaderIcon,
		SearchIcon,
		Trash2Icon,
		TriangleAlertIcon,
		UploadIcon
	} from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import MasterDetail from '$lib/components/organizer/MasterDetail.svelte';
	import {
		deleteOffsetSet,
		fetchOffsetSetRaw,
		importOffsetSet,
		listOffsetSets,
		parseOffsetmap,
		type OffsetEntry,
		type OffsetSetInfo
	} from '$lib/utils/isos';

	let sets = $state<OffsetSetInfo[]>([]);
	let loading = $state(true);
	let openId = $state('');
	let offQ = $state('');

	const open = $derived(sets.find((s) => s.id === openId) ?? null);

	// Detail table — the set's raw offsetmap, parsed client-side (the same
	// bytes Download saves; extra rig fields are tolerated).
	let entries = $state<OffsetEntry[]>([]);
	let entriesLoading = $state(false);
	let rawCache = $state<Record<string, string>>({});

	async function openSet(s: OffsetSetInfo) {
		openId = s.id;
		offQ = '';
		entries = [];
		entriesLoading = true;
		try {
			const raw = rawCache[s.id] ?? (await fetchOffsetSetRaw(s.id));
			rawCache = { ...rawCache, [s.id]: raw };
			entries = parseOffsetmap(raw).entries;
		} catch (err) {
			toaster.error({ title: 'Load set failed', description: describeAsyncError(err) });
		} finally {
			entriesLoading = false;
		}
	}

	const filteredEntries = $derived.by(() => {
		const q = offQ.trim().toLowerCase();
		if (!q) return entries;
		return entries.filter(
			(e) =>
				e.name.toLowerCase().includes(q) ||
				e.value.toLowerCase().includes(q) ||
				e.type.toLowerCase().includes(q)
		);
	});

	function gameLabel(key: string): string {
		if (key === 'haloce') return 'Halo: CE';
		if (key === 'halo2') return 'Halo 2';
		return key;
	}
	function importedLabel(s: OffsetSetInfo): string {
		if (!s.imported) return 'built-in';
		return 'imported ' + s.imported.slice(0, 10);
	}

	async function load() {
		try {
			loading = true;
			sets = await listOffsetSets();
			if (openId && !sets.some((s) => s.id === openId)) openId = '';
		} catch (err) {
			toaster.error({ title: 'Load offset sets failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	// ── Download ────────────────────────────────────────────────────────────
	async function download(s: OffsetSetInfo) {
		try {
			const raw = rawCache[s.id] ?? (await fetchOffsetSetRaw(s.id));
			rawCache = { ...rawCache, [s.id]: raw };
			const a = document.createElement('a');
			a.href = URL.createObjectURL(new Blob([raw], { type: 'application/json' }));
			a.download = s.source_name || `${s.id}.offsetmap.json`;
			a.click();
			URL.revokeObjectURL(a.href);
		} catch (err) {
			toaster.error({ title: 'Download failed', description: describeAsyncError(err) });
		}
	}

	// ── Import dialog ───────────────────────────────────────────────────────
	let importOpen = $state(false);
	let importFile = $state<File | null>(null);
	let importParsed = $state<{ game: string; id: string; count: number } | null>(null);
	let importError = $state('');
	let saveAs = $state('');
	let importBusy = $state(false);
	let importInput: HTMLInputElement;

	async function acceptImport(f: File | undefined | null) {
		if (!f) return;
		importFile = f;
		importParsed = null;
		importError = '';
		try {
			const parsed = parseOffsetmap(await f.text());
			importParsed = { game: parsed.game, id: parsed.id, count: parsed.entries.length };
			saveAs = parsed.id;
		} catch (err) {
			importError = err instanceof Error ? err.message : String(err);
		}
	}

	async function runImport() {
		if (!importFile || !importParsed) return;
		try {
			importBusy = true;
			const res = await toastPromise(importOffsetSet(importFile, saveAs), {
				loading: { title: 'Importing', description: saveAs },
				success: { title: 'Imported', description: `${saveAs} — discs can bind it now.` },
				errorTitle: 'Import failed'
			});
			importOpen = false;
			importFile = null;
			importParsed = null;
			rawCache = {};
			await load();
			const landed = sets.find((s) => s.id === res.id);
			if (landed) void openSet(landed);
		} catch {
			/* toast shown */
		} finally {
			importBusy = false;
		}
	}

	// ── Delete dialog (migration required) ──────────────────────────────────
	let deleteOpen = $state(false);
	let migrateTo = $state('');
	let deleteBusy = $state(false);
	const migrationTargets = $derived(
		open ? sets.filter((s) => s.game === open.game && s.id !== open.id) : []
	);

	async function runDelete() {
		if (!open) return;
		try {
			deleteBusy = true;
			await toastPromise(deleteOffsetSet(open.id, migrateTo), {
				loading: { title: 'Deleting', description: open.id },
				success: { title: 'Deleted', description: open.id },
				errorTitle: 'Delete failed'
			});
			deleteOpen = false;
			openId = '';
			await load();
		} catch {
			/* toast shown */
		} finally {
			deleteBusy = false;
		}
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage offset sets.' });
			return;
		}
		void load();
	});
</script>

{#snippet setRow(s: OffsetSetInfo)}
	<button
		class="flex w-full items-center gap-2.5 border-b border-surface-100-900 px-3 py-2 text-left transition-colors hover:bg-surface-100-900
			{openId === s.id
			? 'border-l-2 border-l-primary-500 bg-primary-500/10'
			: 'border-l-2 border-l-transparent'}"
		onclick={() => openSet(s)}
	>
		<span class="min-w-0 flex-1">
			<span class="flex items-center gap-1.5">
				<span class="truncate font-mono text-sm font-medium">{s.id}</span>
				<span
					class="badge {s.game === 'halo2' ? 'preset-tonal-tertiary' : 'preset-tonal'} text-[10px]"
				>
					{gameLabel(s.game)}
				</span>
			</span>
			<span class="block truncate text-xs opacity-50">
				{s.count} offsets · {s.bound_discs} disc{s.bound_discs === 1 ? '' : 's'} · {importedLabel(
					s
				)}
			</span>
		</span>
	</button>
{/snippet}

{#snippet listPanel()}
	<Card size="flush" class="flex min-w-0 flex-col overflow-hidden">
		<div class="flex items-center justify-between gap-2 border-b border-surface-200-800 p-3">
			<span class="text-sm font-semibold">Sets</span>
			<button class="btn preset-filled btn-sm" onclick={() => (importOpen = true)}>
				<UploadIcon class="size-4" /><span>Import set</span>
			</button>
		</div>
		<div class="flex max-h-[65vh] flex-col overflow-y-auto">
			{#if loading}
				<div class="flex items-center gap-2 p-4 text-sm text-surface-500">
					<LoaderIcon class="size-4 animate-spin" /> Loading…
				</div>
			{:else}
				{#each sets as s (s.id)}
					{@render setRow(s)}
				{/each}
			{/if}
		</div>
	</Card>
{/snippet}

{#snippet compactPills()}
	<div class="flex flex-wrap items-center gap-1.5">
		{#each sets as s (s.id)}
			<button
				class="rounded-full border px-3 py-1 font-mono text-xs transition-colors
					{openId === s.id
					? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
					: 'border-surface-500/25 bg-surface-100-900 text-surface-700-300'}"
				onclick={() => openSet(s)}
			>
				{s.id} · {s.count}
			</button>
		{/each}
		<button class="btn preset-filled btn-sm" onclick={() => (importOpen = true)}>
			<UploadIcon class="size-4" /><span>Import</span>
		</button>
	</div>
{/snippet}

{#snippet detailPanel()}
	{#if !open}
		<Card class="flex min-h-40 items-center justify-center">
			<p class="text-sm text-surface-500">Pick a set to view its addresses.</p>
		</Card>
	{:else}
		<Card class="flex min-w-0 flex-col gap-4">
			<div class="flex flex-wrap items-center gap-2">
				<h3 class="min-w-0 truncate h4 font-mono">{open.id}</h3>
				<span class="badge {open.game === 'halo2' ? 'preset-tonal-tertiary' : 'preset-tonal'}">
					{gameLabel(open.game)}
				</span>
				{#if open.baseline}
					<span class="badge preset-tonal-surface">baseline</span>
				{/if}
				<span class="flex-1"></span>
				<button class="btn preset-tonal btn-sm" onclick={() => download(open!)}>
					<DownloadIcon class="size-4" /><span>Download</span>
				</button>
				{#if !open.baseline}
					<button
						class="btn-icon preset-tonal-error btn-sm"
						title="Delete set"
						onclick={() => {
							migrateTo = migrationTargets[0]?.id ?? '';
							deleteOpen = true;
						}}
					>
						<Trash2Icon class="size-4" />
					</button>
				{/if}
			</div>

			<div class="flex flex-wrap gap-x-6 gap-y-1 text-xs opacity-70">
				<span>{importedLabel(open)}{open.version ? ` · v${open.version}` : ''}</span>
				{#if open.source_name}<span class="font-mono">{open.source_name}</span>{/if}
				<span>bound by {open.bound_discs} disc{open.bound_discs === 1 ? '' : 's'}</span>
				{#if open.description}<span>{open.description}</span>{/if}
			</div>

			<div class="flex items-center gap-2 rounded bg-surface-100-900 px-3 py-2">
				<SearchIcon class="size-4 flex-none opacity-60" />
				<input
					type="search"
					class="w-full min-w-0 border-none bg-transparent text-sm outline-none"
					placeholder="Filter by name, address, or type"
					bind:value={offQ}
				/>
				<span class="flex-none text-xs tabular-nums opacity-50">{open.count} mapped</span>
			</div>

			{#if entriesLoading}
				<div class="flex items-center gap-2 p-2 text-sm text-surface-500">
					<LoaderIcon class="size-4 animate-spin" /> Loading addresses…
				</div>
			{:else if filteredEntries.length === 0}
				<p class="p-2 text-sm text-surface-500">No offsets match the filter.</p>
			{:else}
				<div class="max-h-[50vh] overflow-y-auto">
					<table class="w-full text-left font-mono text-xs">
						<thead class="sticky top-0 bg-surface-50-950">
							<tr class="text-[10px] tracking-widest text-surface-500 uppercase">
								<th class="py-1.5 pr-4 font-semibold">Name</th>
								<th class="py-1.5 pr-4 font-semibold">Address</th>
								<th class="py-1.5 font-semibold">Type</th>
							</tr>
						</thead>
						<tbody>
							{#each filteredEntries as e (e.name)}
								<tr class="border-t border-surface-100-900">
									<td class="py-1.5 pr-4">{e.name}</td>
									<td class="py-1.5 pr-4 opacity-80">{e.value}</td>
									<td class="py-1.5 opacity-60">{e.type}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</Card>
	{/if}
{/snippet}

<div class="flex flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Offsets"
		description="Named offset-set versions the scraper reads builds with — stats only flow when the disc's build matches the addresses being read. Discs bind a set on the Discs page; this library imports, exports, and retires them."
	/>

	<MasterDetail
		open={!!open}
		onback={() => (openId = '')}
		backLabel="All sets"
		list={listPanel}
		compact={compactPills}
		detail={detailPanel}
	/>
</div>

<!-- Import dialog -->
<Dialog
	open={importOpen}
	onClose={() => {
		importOpen = false;
		importFile = null;
		importParsed = null;
		importError = '';
	}}
	title="Import offset set"
	size="md"
>
	<div class="flex flex-col gap-4">
		<button
			class="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-surface-500/40 bg-black/20 p-6 text-center transition-colors hover:border-primary-500/50"
			ondragover={(e) => e.preventDefault()}
			ondrop={(e) => {
				e.preventDefault();
				void acceptImport(e.dataTransfer?.files?.[0]);
			}}
			onclick={() => importInput.click()}
		>
			<UploadIcon class="size-5 opacity-60" />
			<span class="text-sm">
				{importFile ? importFile.name : 'Drop an offsetmap JSON — or browse'}
			</span>
			<span class="max-w-80 text-xs opacity-60">
				Exports from the hunting rig only. Addresses land pre-named and pre-typed — nothing is
				hand-edited here.
			</span>
		</button>
		<input
			bind:this={importInput}
			type="file"
			accept="application/json,.json"
			class="hidden"
			onchange={(e) => void acceptImport((e.currentTarget as HTMLInputElement).files?.[0])}
		/>

		{#if importError}
			<p class="flex items-center gap-2 text-sm text-error-500">
				<TriangleAlertIcon class="size-4" />{importError}
			</p>
		{:else if importParsed}
			<div class="flex flex-col gap-1.5">
				<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
					Read from your file
				</span>
				<div
					class="flex flex-wrap items-center gap-2 rounded border border-surface-500/20 bg-black/20 px-3 py-2"
				>
					<span class="font-mono text-sm">{importParsed.id}</span>
					<span class="flex-1"></span>
					<span class="font-mono text-xs opacity-60">
						{gameLabel(importParsed.game)} · {importParsed.count} offsets
					</span>
				</div>
				<span class="text-xs opacity-60">
					Checked before anything is saved — confirm it's the right export, then Import. Same id
					re-imported = new version of that set.
				</span>
			</div>
			<label class="label">
				<span class="label-text">Save as</span>
				<input class="input font-mono" bind:value={saveAs} maxlength={64} />
				<span class="text-xs opacity-60">
					Prefilled from the file — rename here before importing. Discs will reference this id.
				</span>
			</label>
		{/if}
	</div>
	{#snippet footer()}
		<button class="btn preset-tonal" onclick={() => (importOpen = false)} disabled={importBusy}>
			Cancel
		</button>
		<button
			class="btn preset-filled"
			onclick={runImport}
			disabled={importBusy || !importParsed || !saveAs.trim()}
		>
			{#if importBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
			<span>Import set</span>
		</button>
	{/snippet}
</Dialog>

<!-- Delete + migrate dialog -->
<Dialog open={deleteOpen} onClose={() => (deleteOpen = false)} title="Delete offset set" size="sm">
	{#if open}
		<div class="flex flex-col gap-4">
			<p class="flex items-start gap-2 text-sm">
				<TriangleAlertIcon class="mt-0.5 size-4 flex-none text-warning-500" />
				<span>
					Remove <span class="font-mono">{open.id}</span>?
					{#if open.bound_discs > 0}
						{open.bound_discs} disc{open.bound_discs === 1 ? '' : 's'} bind{open.bound_discs === 1
							? 's'
							: ''} it — they need a new set to keep their stats flowing.
					{:else}
						No discs currently bind it.
					{/if}
				</span>
			</p>
			<label class="label">
				<span class="label-text">Migrate bound discs to</span>
				<select class="select" bind:value={migrateTo}>
					{#each migrationTargets as t (t.id)}
						<option value={t.id}>{t.id} ({t.count} offsets)</option>
					{/each}
					<option value="">— unbound (stats go dark) —</option>
				</select>
				<span class="text-xs opacity-60">
					Only sets for the same game are offered. Every dependent disc re-binds on confirm.
				</span>
			</label>
		</div>
	{/if}
	{#snippet footer()}
		<button class="btn preset-tonal" onclick={() => (deleteOpen = false)} disabled={deleteBusy}>
			Cancel
		</button>
		<button class="btn preset-tonal-error" onclick={runDelete} disabled={deleteBusy}>
			{#if deleteBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
			<span>Delete &amp; migrate</span>
		</button>
	{/snippet}
</Dialog>
