<script lang="ts">
	// Nameplates — the organizer-owned library of 600×100 (6:1) banner art for
	// the overlay NamePlate (CL-18). One list, one dialog: cards render each
	// banner AS the real plate pill (the library view doubles as the legibility
	// check); the only quick action on a card is the Selectable toggle —
	// everything else is behind the card click. Players pick from whatever is
	// Selectable in their own settings; this page never assigns banners.
	import { onMount } from 'svelte';
	import { SearchIcon } from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import { apiBaseURL } from '$lib/utils/api-base';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import FilterChips from '$lib/components/organizer/FilterChips.svelte';
	import ImageReframe from '$lib/components/organizer/ImageReframe.svelte';
	import OrgToggle from '$lib/components/organizer/OrgToggle.svelte';
	import PlatePreview from '$lib/components/organizer/PlatePreview.svelte';

	interface NameplateRecord {
		id: string;
		name: string;
		art: string;
		selectable: boolean;
		created: string;
		updated: string;
	}

	let rows = $state<NameplateRecord[]>([]);
	let loading = $state(true);
	let q = $state('');
	let fSel = $state<Record<string, boolean>>({ on: true, off: true });

	function artURL(r: NameplateRecord): string {
		return r.art ? `${apiBaseURL()}/api/files/nameplates/${r.id}/${r.art}` : '';
	}
	function metaLine(r: NameplateRecord): string {
		return r.art ? `uploaded ${r.created.slice(0, 10)} · 600×100` : 'no art yet — open to upload';
	}

	const filtered = $derived.by(() => {
		const query = q.trim().toLowerCase();
		return rows
			.filter((r) => (r.selectable ? fSel.on : fSel.off))
			.filter((r) => !query || r.name.toLowerCase().includes(query))
			.toSorted((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
	});

	async function load() {
		try {
			loading = true;
			rows = await pb.collection('nameplates').getFullList<NameplateRecord>({ sort: 'name' });
		} catch (err) {
			toaster.error({ title: 'Load banners failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	// Instant toggle on the card — players already wearing a hidden banner keep
	// it; the toggle only gates the picker.
	async function flip(r: NameplateRecord) {
		const next = !r.selectable;
		rows = rows.map((x) => (x.id === r.id ? { ...x, selectable: next } : x));
		try {
			await pb.collection('nameplates').update(r.id, { selectable: next });
		} catch (err) {
			rows = rows.map((x) => (x.id === r.id ? { ...x, selectable: r.selectable } : x));
			toaster.error({ title: 'Toggle failed', description: describeAsyncError(err) });
		}
	}

	// ── Editor dialog (the ONLY drop zone on the page) ──────────────────────
	let dlgFor = $state<'' | '__new' | string>('');
	let dName = $state('');
	let dSel = $state(true);
	let dBusy = $state(false);
	let reframe = $state<ReturnType<typeof ImageReframe> | null>(null);

	const dlgRecord = $derived(rows.find((r) => r.id === dlgFor) ?? null);
	const dlgOpen = $derived(dlgFor === '__new' || !!dlgRecord);

	function openEditor(r: NameplateRecord | null) {
		dlgFor = r ? r.id : '__new';
		dName = r?.name ?? '';
		dSel = r?.selectable ?? true;
	}
	function closeEditor() {
		dlgFor = '';
	}

	async function saveEditor() {
		const name = dName.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'A banner name is required.' });
			return;
		}
		try {
			dBusy = true;
			const body = new FormData();
			body.set('name', name);
			body.set('selectable', dSel ? 'true' : 'false');
			const blob = await reframe?.exportBlob();
			if (blob) body.set('art', new File([blob], 'banner.png', { type: 'image/png' }));
			await toastPromise(
				dlgRecord
					? pb.collection('nameplates').update(dlgRecord.id, body)
					: pb.collection('nameplates').create(body),
				{
					loading: { title: 'Saving', description: name },
					success: { title: dlgRecord ? 'Saved' : 'Added to library', description: name },
					errorTitle: 'Save failed',
					errorDescription: (err) => (err instanceof Error ? err.message : 'Failed')
				}
			);
			closeEditor();
			await load();
		} catch {
			/* toast shown */
		} finally {
			dBusy = false;
		}
	}

	async function deleteEditor() {
		if (!dlgRecord) return;
		const ok = await confirmToast({
			title: 'Delete banner',
			description: `Remove "${dlgRecord.name}" from the library? Players wearing it fall back to the plain plate.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		try {
			dBusy = true;
			await toastPromise(pb.collection('nameplates').delete(dlgRecord.id), {
				loading: { title: 'Deleting', description: dlgRecord.name },
				success: { title: 'Deleted', description: dlgRecord.name },
				errorTitle: 'Delete failed'
			});
			closeEditor();
			await load();
		} catch {
			/* toast shown */
		} finally {
			dBusy = false;
		}
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to curate banners.' });
			return;
		}
		void load();
	});
</script>

<div class="flex flex-col gap-4 sm:gap-6">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0 flex-1 basis-80">
			<PageHeader
				title="Nameplates"
				description="Banner art for the overlay nameplate. Every banner in the library at a glance; uploading and reframing live in the editor — open a card or hit + New banner."
			/>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<div class="flex max-w-56 min-w-0 items-center gap-2 rounded-lg bg-surface-100-900 px-3 py-2">
				<SearchIcon class="size-4 flex-none opacity-60" />
				<input
					type="search"
					class="w-full min-w-0 border-none bg-transparent text-sm outline-none"
					placeholder="Filter banners"
					bind:value={q}
				/>
			</div>
			<FilterChips
				chips={[
					{ key: 'on', label: 'Selectable' },
					{ key: 'off', label: 'Hidden' }
				]}
				bind:active={fSel}
			/>
			<button class="btn preset-filled btn-sm" onclick={() => openEditor(null)}>
				+ New banner
			</button>
		</div>
	</div>

	<Card size="flush" class="flex flex-col overflow-hidden">
		<div class="flex items-center gap-2 border-b border-surface-200-800 p-3">
			<span class="text-sm font-semibold">Library</span>
			<span class="font-mono text-xs opacity-50">
				{filtered.length} of {rows.length} banners
			</span>
		</div>
		{#if loading}
			<p class="p-4 text-sm text-surface-500">Loading…</p>
		{:else if filtered.length === 0}
			<p class="p-4 text-sm text-surface-500">
				{rows.length === 0 ? 'No banners yet — add one.' : 'No banners match — clear the filter.'}
			</p>
		{:else}
			<div class="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-x-2 p-1.5">
				{#each filtered as r (r.id)}
					<div
						class="flex cursor-pointer flex-col gap-2 rounded-lg p-2.5 transition-colors hover:bg-surface-100-900"
						role="button"
						tabindex="0"
						onclick={() => openEditor(r)}
						onkeydown={(e) => {
							if (e.key === 'Enter' || e.key === ' ') openEditor(r);
						}}
					>
						<PlatePreview bg={artURL(r)} />
						<div class="flex min-w-0 items-center gap-2">
							<span class="flex min-w-0 flex-1 flex-col gap-0.5">
								<span class="truncate text-sm font-semibold">{r.name || 'untitled'}</span>
								<span class="truncate font-mono text-[10px] opacity-50">{metaLine(r)}</span>
							</span>
							<span class="flex flex-none items-center gap-2">
								<OrgToggle on={r.selectable} onflip={() => flip(r)} />
								<span class="w-16 text-xs {r.selectable ? 'text-primary-600-400' : 'opacity-50'}">
									{r.selectable ? 'Selectable' : 'Hidden'}
								</span>
							</span>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card>
</div>

<!-- Editor dialog -->
<Dialog
	open={dlgOpen}
	onClose={closeEditor}
	title={dlgRecord ? 'Edit banner' : 'New banner'}
	size="lg"
>
	<div class="flex flex-col gap-4">
		<div class="flex flex-col gap-1.5">
			<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
				Banner art
			</span>
			<ImageReframe
				bind:this={reframe}
				src={dlgRecord ? artURL(dlgRecord) : ''}
				targetW={600}
				targetH={100}
				radius="999px"
				placeholder="Drop banner art — any size"
			>
				{#snippet overlay()}
					<PlatePreview chromeOnly />
				{/snippet}
			</ImageReframe>
			<span class="text-xs opacity-60">
				The one drop zone. Drag any image in, then drag / zoom to reframe the 6:1 crop (stored as
				600×100). The chrome on top is the real plate — scrim, avatar well, gamertag, motto at exact
				NamePlate geometry — so what you line up here is what the overlay draws.
			</span>
		</div>

		<label class="label">
			<span class="label-text">Name</span>
			<input class="input" maxlength={48} placeholder="e.g. Neon" bind:value={dName} />
			<span class="text-xs opacity-60">Organizer-facing label — search runs on it.</span>
		</label>

		<OrgToggle on={dSel} label="Players can pick this" onflip={() => (dSel = !dSel)} />
	</div>
	{#snippet footer()}
		<div class="flex w-full flex-wrap items-center gap-2">
			{#if dlgRecord}
				<button class="btn preset-tonal-error btn-sm" onclick={deleteEditor} disabled={dBusy}>
					Delete
				</button>
			{/if}
			<span class="flex-1"></span>
			<button class="btn preset-tonal btn-sm" onclick={closeEditor} disabled={dBusy}>
				Cancel
			</button>
			<button class="btn preset-filled btn-sm" onclick={saveEditor} disabled={dBusy}>
				{dlgRecord ? 'Save' : 'Add to library'}
			</button>
		</div>
	{/snippet}
</Dialog>
