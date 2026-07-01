<script lang="ts">
	import { onMount } from 'svelte';
	import {
		LoaderIcon,
		PlusIcon,
		RefreshCwIcon,
		SearchIcon,
		Trash2Icon,
		PencilIcon,
		DownloadIcon,
		SwordsIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup, SortState } from '$lib/components/ui/data-table';
	import GametypeForm, {
		type GametypeFormState
	} from '$lib/components/gamertag/GametypeForm.svelte';
	import { lanMeta } from '$lib/utils/lansaves';
	import { downloadRecordFile } from '$lib/utils/gamertag';
	import type { GametypeRecord, GametypeSettings } from '$lib/types/gamertag';

	let rows = $state<GametypeRecord[]>([]);
	let ceEngines = $state<string[]>(['slayer', 'ctf', 'oddball', 'king', 'race']);
	let loading = $state(true);
	let filter = $state('');
	let sort = $state<SortState>({ key: 'name', dir: 'asc' });

	let dialogOpen = $state(false);
	let formBusy = $state(false);
	let form = $state<GametypeFormState>(newForm());
	let busyById = $state<Record<string, boolean>>({});

	function newForm(): GametypeFormState {
		return { id: '', title: 'ce', engine: 'slayer', name: '', teams: true, radar: false };
	}

	async function load() {
		try {
			loading = true;
			rows = await pb
				.collection('gametypes')
				.getFullList<GametypeRecord>({ expand: 'created_by', sort: 'title,name' });
		} catch (err) {
			toaster.error({ title: 'Load gametypes failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function openNew() {
		form = newForm();
		dialogOpen = true;
	}

	function openEdit(r: GametypeRecord) {
		const s = r.settings ?? {};
		form = {
			id: r.id,
			title: r.title,
			engine: r.engine,
			name: r.name,
			teams: s.teams ?? true,
			radar: s.radar ?? false,
			scoreLimit: s.score_limit,
			timeMinutes: s.time_minutes
		};
		dialogOpen = true;
	}

	function num(v: number | undefined): number | undefined {
		return typeof v === 'number' && Number.isFinite(v) ? v : undefined;
	}

	function buildSettings(f: GametypeFormState): GametypeSettings {
		const s: GametypeSettings = {};
		if (num(f.scoreLimit) !== undefined) s.score_limit = f.scoreLimit;
		if (f.title === 'ce') {
			s.teams = f.teams;
			if (f.radar) s.radar = true;
			if (num(f.timeMinutes) !== undefined) s.time_minutes = f.timeMinutes;
		}
		return s;
	}

	async function save() {
		const name = form.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'A variant name is required.' });
			return;
		}
		if (!auth.user) return;
		const payload = {
			title: form.title,
			engine: form.title === 'h2' ? 'slayer' : form.engine,
			name,
			settings: buildSettings(form),
			created_by: auth.user.id
		};
		try {
			formBusy = true;
			await toastPromise(
				form.id
					? pb.collection('gametypes').update(form.id, payload)
					: pb.collection('gametypes').create(payload),
				{
					loading: { title: 'Saving', description: name },
					success: { title: 'Saved', description: `${name} — save file generated.` },
					errorTitle: 'Save failed'
				}
			);
			dialogOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			formBusy = false;
		}
	}

	async function remove(r: GametypeRecord) {
		const ok = await confirmToast({
			title: 'Delete gametype',
			description: `Remove "${r.name}" from the shared library? Players will no longer be able to download it.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		busyById = { ...busyById, [r.id]: true };
		try {
			await toastPromise(pb.collection('gametypes').delete(r.id), {
				loading: { title: 'Deleting', description: r.name },
				success: { title: 'Deleted', description: r.name },
				errorTitle: 'Delete failed'
			});
			await load();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...busyById };
			delete nx[r.id];
			busyById = nx;
		}
	}

	async function download(r: GametypeRecord) {
		try {
			await downloadRecordFile(r, 'save_bundle', `${r.title}-${r.name}.tar`);
		} catch (e) {
			toaster.error({
				title: 'Download failed',
				description: e instanceof Error ? e.message : String(e)
			});
		}
	}

	const filtered = $derived.by<GametypeRecord[]>(() => {
		const q = filter.trim().toLowerCase();
		const base = !q
			? rows
			: rows.filter(
					(r) =>
						r.name.toLowerCase().includes(q) ||
						r.engine.toLowerCase().includes(q) ||
						r.title.toLowerCase().includes(q)
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
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage gametypes.' });
			return;
		}
		void load();
		void lanMeta()
			.then((m) => (ceEngines = m.ce_engines))
			.catch(() => {});
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Gametype library"
		description="Curate the shared Halo: CE & Halo 2 multiplayer gametype variants. Each is template-patched into a real, ready-to-write save the moment you save it — players download them to their box."
	/>

	<div class="flex items-center justify-between gap-2">
		<div class="input-group flex-1 grid-cols-[auto_1fr]">
			<div class="ig-cell preset-tonal"><SearchIcon class="size-4" /></div>
			<input
				type="search"
				class="ig-input"
				placeholder="Filter by name, engine, or title"
				bind:value={filter}
			/>
		</div>
		<button class="btn preset-tonal" onclick={() => load()} disabled={loading} aria-label="Refresh">
			{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
					class="size-4"
				/>{/if}
			<span>Refresh</span>
		</button>
		<button class="btn preset-filled" onclick={openNew}>
			<PlusIcon class="size-4" /><span>New gametype</span>
		</button>
	</div>

	{#snippet titleCell({ row }: { row: GametypeRecord })}
		<span class="badge preset-tonal">{row.title.toUpperCase()}</span>
	{/snippet}
	{#snippet nameCell({ row }: { row: GametypeRecord })}
		<span class="flex items-center gap-2">
			<SwordsIcon class="size-3.5 opacity-60" />
			<span class="font-medium">{row.name}</span>
		</span>
	{/snippet}
	{#snippet fileCell({ row }: { row: GametypeRecord })}
		{#if row.save_bundle}
			<span class="badge preset-tonal-success">generated</span>
		{:else}
			<span class="badge preset-tonal-error">none</span>
		{/if}
	{/snippet}
	{#snippet creatorCell({ row }: { row: GametypeRecord })}
		<span>{row.expand?.created_by?.username ?? '—'}</span>
	{/snippet}
	{#snippet actionsCell({ row }: { row: GametypeRecord })}
		<div
			role="presentation"
			class="inline-flex items-center justify-end gap-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<button
				class="btn-icon preset-tonal btn-sm"
				title="Download save"
				onclick={() => download(row)}
				disabled={!row.save_bundle}
			>
				<DownloadIcon class="size-4" />
			</button>
			<button class="btn-icon preset-tonal btn-sm" title="Edit" onclick={() => openEdit(row)}>
				<PencilIcon class="size-4" />
			</button>
			<button
				class="btn-icon preset-tonal-error btn-sm"
				title="Delete"
				onclick={() => remove(row)}
				disabled={!!busyById[row.id]}
			>
				{#if busyById[row.id]}<LoaderIcon class="size-4 animate-spin" />{:else}<Trash2Icon
						class="size-4"
					/>{/if}
			</button>
		</div>
	{/snippet}

	<Card size="flush" class="overflow-x-auto">
		<DataTable
			rows={filtered}
			groups={[
				{
					columns: [
						{ key: 'title', label: 'Title', cell: titleCell },
						{ key: 'name', label: 'Variant', cell: nameCell },
						{ key: 'engine', label: 'Engine' },
						{ key: 'save_bundle', label: 'File', cell: fileCell, sortable: false },
						{ key: 'created_by', label: 'By', cell: creatorCell },
						{ key: 'actions', label: '', cell: actionsCell, sortable: false, align: 'right' }
					]
				}
			] satisfies DataColumnGroup<GametypeRecord>[]}
			rowKey={(r) => r.id}
			density="comfortable"
			{sort}
			onSortChange={(s) => (sort = s)}
			secondarySort={{ key: 'name', dir: 'asc' }}
			loading={loading && filtered.length === 0}
			emptyMessage={filter ? 'No matches.' : 'No gametypes yet — add one.'}
		/>
	</Card>
</div>

<Dialog
	open={dialogOpen}
	onClose={() => {
		if (!formBusy) dialogOpen = false;
	}}
	title={form.id ? 'Edit gametype' : 'New gametype'}
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void save();
		}}
		class="flex flex-col gap-4"
	>
		<GametypeForm bind:form {ceEngines} disabled={formBusy} />
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (dialogOpen = false)}
				disabled={formBusy}>Cancel</button
			>
			<button type="submit" class="btn preset-filled" disabled={formBusy}>
				{#if formBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>{form.id ? 'Save' : 'Create'}</span>
			</button>
		</div>
	</form>
</Dialog>
