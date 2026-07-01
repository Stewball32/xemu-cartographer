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
		LibraryIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup, SortState } from '$lib/components/ui/data-table';
	import { downloadRecordFile } from '$lib/utils/gamertag';
	import type { GameTitleRecord } from '$lib/types/gamertag';

	let rows = $state<GameTitleRecord[]>([]);
	let loading = $state(true);
	let filter = $state('');
	let sort = $state<SortState>({ key: 'name', dir: 'asc' });

	let dialogOpen = $state(false);
	let formBusy = $state(false);
	let form = $state({ id: '', name: '', description: '' });
	let file = $state<File | null>(null);
	let fileInput = $state<HTMLInputElement | null>(null);
	let busyById = $state<Record<string, boolean>>({});

	async function load() {
		try {
			loading = true;
			rows = await pb
				.collection('game_titles')
				.getFullList<GameTitleRecord>({ expand: 'created_by', sort: 'name' });
		} catch (err) {
			toaster.error({ title: 'Load games failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function openNew() {
		form = { id: '', name: '', description: '' };
		file = null;
		if (fileInput) fileInput.value = '';
		dialogOpen = true;
	}

	function openEdit(r: GameTitleRecord) {
		form = { id: r.id, name: r.name, description: r.description ?? '' };
		file = null;
		if (fileInput) fileInput.value = '';
		dialogOpen = true;
	}

	function onFile(e: Event) {
		const input = e.target as HTMLInputElement;
		file = input.files?.[0] ?? null;
	}

	async function save() {
		const name = form.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'A name is required.' });
			return;
		}
		if (!form.id && !file) {
			toaster.error({ title: 'Invalid', description: 'Choose a file to upload.' });
			return;
		}
		if (!auth.user) return;

		const fd = new FormData();
		fd.set('name', name);
		fd.set('description', form.description.trim());
		fd.set('created_by', auth.user.id);
		if (file) fd.set('file', file);

		try {
			formBusy = true;
			await toastPromise(
				form.id
					? pb.collection('game_titles').update(form.id, fd)
					: pb.collection('game_titles').create(fd),
				{
					loading: { title: file ? 'Uploading' : 'Saving', description: name },
					success: { title: 'Saved', description: name },
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

	async function remove(r: GameTitleRecord) {
		const ok = await confirmToast({
			title: 'Delete game',
			description: `Remove "${r.name}" and its uploaded file? Players will no longer be able to download it.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		busyById = { ...busyById, [r.id]: true };
		try {
			await toastPromise(pb.collection('game_titles').delete(r.id), {
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

	async function download(r: GameTitleRecord) {
		try {
			await downloadRecordFile(r, 'file', r.file);
		} catch (e) {
			toaster.error({
				title: 'Download failed',
				description: e instanceof Error ? e.message : String(e)
			});
		}
	}

	const filtered = $derived.by<GameTitleRecord[]>(() => {
		const q = filter.trim().toLowerCase();
		const base = !q
			? rows
			: rows.filter(
					(r) => r.name.toLowerCase().includes(q) || r.description?.toLowerCase().includes(q)
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
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage games.' });
			return;
		}
		void load();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Games"
		description="Upload the playable game (XBE) files the LAN client can pull to the box. A plain library — name, description, and the file — not a configurator."
	/>

	<div class="flex items-center justify-between gap-2">
		<div class="input-group flex-1 grid-cols-[auto_1fr]">
			<div class="ig-cell preset-tonal"><SearchIcon class="size-4" /></div>
			<input type="search" class="ig-input" placeholder="Filter by name" bind:value={filter} />
		</div>
		<button class="btn preset-tonal" onclick={() => load()} disabled={loading} aria-label="Refresh">
			{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
					class="size-4"
				/>{/if}
			<span>Refresh</span>
		</button>
		<button class="btn preset-filled" onclick={openNew}>
			<PlusIcon class="size-4" /><span>Upload game</span>
		</button>
	</div>

	{#snippet nameCell({ row }: { row: GameTitleRecord })}
		<span class="flex items-center gap-2">
			<LibraryIcon class="size-3.5 opacity-60" />
			<span class="font-medium">{row.name}</span>
		</span>
	{/snippet}
	{#snippet descCell({ row }: { row: GameTitleRecord })}
		{#if row.description}<span>{row.description}</span>{:else}<span class="text-xs opacity-50"
				>—</span
			>{/if}
	{/snippet}
	{#snippet fileCell({ row }: { row: GameTitleRecord })}
		<span class="font-mono text-xs">{row.file || '—'}</span>
	{/snippet}
	{#snippet creatorCell({ row }: { row: GameTitleRecord })}
		<span>{row.expand?.created_by?.username ?? '—'}</span>
	{/snippet}
	{#snippet actionsCell({ row }: { row: GameTitleRecord })}
		<div
			role="presentation"
			class="inline-flex items-center justify-end gap-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<button
				class="btn-icon preset-tonal btn-sm"
				title="Download"
				onclick={() => download(row)}
				disabled={!row.file}
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
						{ key: 'name', label: 'Name', cell: nameCell },
						{ key: 'description', label: 'Description', cell: descCell },
						{ key: 'file', label: 'File', cell: fileCell, sortable: false },
						{ key: 'created_by', label: 'By', cell: creatorCell },
						{ key: 'actions', label: '', cell: actionsCell, sortable: false, align: 'right' }
					]
				}
			] satisfies DataColumnGroup<GameTitleRecord>[]}
			rowKey={(r) => r.id}
			density="comfortable"
			{sort}
			onSortChange={(s) => (sort = s)}
			secondarySort={{ key: 'name', dir: 'asc' }}
			loading={loading && filtered.length === 0}
			emptyMessage={filter ? 'No matches.' : 'No games uploaded yet.'}
		/>
	</Card>
</div>

<Dialog
	open={dialogOpen}
	onClose={() => {
		if (!formBusy) dialogOpen = false;
	}}
	title={form.id ? 'Edit game' : 'Upload game'}
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void save();
		}}
		class="flex flex-col gap-3"
	>
		<label class="label">
			<span class="label-text">Name</span>
			<input
				type="text"
				class="input"
				bind:value={form.name}
				maxlength="120"
				placeholder="e.g. Halo: Combat Evolved"
				disabled={formBusy}
			/>
		</label>
		<label class="label">
			<span class="label-text">Description</span>
			<textarea
				class="textarea"
				rows="3"
				bind:value={form.description}
				maxlength="2000"
				placeholder="Optional notes for players"
				disabled={formBusy}
			></textarea>
		</label>
		<label class="label">
			<span class="label-text">{form.id ? 'Replace file (optional)' : 'File'}</span>
			<input
				type="file"
				class="input"
				bind:this={fileInput}
				onchange={onFile}
				disabled={formBusy}
			/>
		</label>
		{#if form.id}
			<p class="text-xs opacity-60">Leave the file empty to keep the current upload.</p>
		{/if}
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (dialogOpen = false)}
				disabled={formBusy}>Cancel</button
			>
			<button type="submit" class="btn preset-filled" disabled={formBusy}>
				{#if formBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>{form.id ? 'Save' : 'Upload'}</span>
			</button>
		</div>
	</form>
</Dialog>
