<script lang="ts">
	// Maps — the canonical map catalog. A card is a unique BUILD (filename PLUS
	// content hash), not a filename: a modded disc shipping a different
	// damnation.map under the stock name gets its own card, identical hashes
	// across discs collapse into one. Cards are read-only; identity curation
	// (name, variant-of, description, graphic, power items) lives in the detail
	// panel under the shelf. Un-uploaded cards fall back to the BSP top-down
	// render the ingest pipeline already produces.
	import { onMount } from 'svelte';
	import { ChevronRightIcon, LoaderIcon, MapIcon, SearchIcon, XIcon } from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import ImageReframe from '$lib/components/organizer/ImageReframe.svelte';
	import DraftBar from '$lib/components/organizer/DraftBar.svelte';
	import {
		catalogArtURL,
		listMapsCatalog,
		type CatalogMap,
		type PowerItemRow
	} from '$lib/utils/isos';

	let maps = $state<CatalogMap[]>([]);
	let loading = $state(true);
	let q = $state('');
	let games = $state<Record<string, boolean>>({ ce: true, h2: true });
	let openId = $state('');
	let busy = $state(false);

	// ── Draft ───────────────────────────────────────────────────────────────
	interface MapDraft {
		display_name: string;
		variant_of: string;
		description: string;
		power_items: PowerItemRow[];
	}
	let draft = $state<MapDraft | null>(null);
	let reframe = $state<ImageReframe | null>(null);
	let graphicDirty = $state(false);

	const open = $derived(maps.find((m) => m.id === openId) ?? null);

	function draftFor(m: CatalogMap): MapDraft {
		return {
			display_name: m.display_name,
			variant_of: m.variant_of,
			description: m.description,
			power_items: m.power_items.map((r) => ({ items: [...r.items], every: r.every }))
		};
	}
	const dirty = $derived.by(() => {
		if (!open || !draft) return false;
		if (graphicDirty) return true;
		const base = draftFor(open);
		return (
			draft.display_name !== base.display_name ||
			draft.variant_of !== base.variant_of ||
			draft.description !== base.description ||
			JSON.stringify(draft.power_items) !== JSON.stringify(base.power_items)
		);
	});

	function toggleCard(m: CatalogMap) {
		if (openId === m.id) {
			openId = '';
			draft = null;
		} else {
			openId = m.id;
			draft = draftFor(m);
			graphicDirty = false;
			addRowOpen = false;
		}
	}

	// ── Shelf (filter + sort) ───────────────────────────────────────────────
	function nameOf(m: CatalogMap): string {
		return m.display_name || '';
	}
	const shelf = $derived.by(() => {
		const query = q.trim().toLowerCase();
		return maps
			.filter((m) => games[m.game])
			.filter((m) => {
				if (!query) return true;
				const hay = `${m.filename} ${m.content_hash} ${m.display_name}`.toLowerCase();
				return hay.includes(query);
			})
			.toSorted((a, b) =>
				(nameOf(a) || a.filename)
					.toLowerCase()
					.localeCompare((nameOf(b) || b.filename).toLowerCase())
			);
	});
	const byId = $derived(new Map(maps.map((m) => [m.id, m])));

	// Variant targets: same game, not self, not itself a variant (no chains).
	const parentOptions = $derived(
		open
			? maps
					.filter((p) => p.game === open.game && p.id !== open.id && !p.variant_of)
					.toSorted((a, b) => (nameOf(a) || a.filename).localeCompare(nameOf(b) || b.filename))
			: []
	);
	const children = $derived(open ? maps.filter((m) => m.variant_of === open.id) : []);
	const openParent = $derived(open && draft ? (byId.get(draft.variant_of) ?? null) : null);

	// ── Power items editing (add / remove rows, drafted) ────────────────────
	let addRowOpen = $state(false);
	let addItems = $state('');
	let addEvery = $state('');
	function addPowerRow() {
		if (!draft) return;
		const items = addItems
			.split(',')
			.map((s) => s.trim())
			.filter(Boolean);
		const every = addEvery.trim();
		if (items.length === 0 || !every) return;
		draft.power_items = [...draft.power_items, { items, every }];
		addItems = '';
		addEvery = '';
		addRowOpen = false;
	}

	// ── Data flow ───────────────────────────────────────────────────────────
	async function load() {
		try {
			loading = true;
			maps = await listMapsCatalog();
			if (openId && !maps.some((m) => m.id === openId)) {
				openId = '';
				draft = null;
			}
		} catch (err) {
			toaster.error({ title: 'Load map catalog failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	async function save() {
		if (!open || !draft) return;
		try {
			busy = true;
			const body = new FormData();
			body.set('display_name', draft.display_name.trim());
			body.set('variant_of', draft.variant_of);
			body.set('description', draft.description.trim());
			body.set('power_items', JSON.stringify(draft.power_items));
			const blob = await reframe?.exportBlob();
			if (blob) body.set('graphic', new File([blob], 'graphic.png', { type: 'image/png' }));
			await toastPromise(pb.collection('maps').update(open.id, body), {
				loading: { title: 'Saving', description: draft.display_name || open.filename },
				success: { title: 'Saved', description: draft.display_name || open.filename },
				errorTitle: 'Save failed',
				errorDescription: (err) => (err instanceof Error ? err.message : 'Failed')
			});
			reframe?.reset();
			graphicDirty = false;
			await load();
			if (openId) draft = draftFor(maps.find((m) => m.id === openId)!);
		} catch {
			/* toast shown */
		} finally {
			busy = false;
		}
	}
	function discard() {
		if (open) draft = draftFor(open);
		reframe?.reset();
		graphicDirty = false;
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to curate the catalog.' });
			return;
		}
		void load();
	});
</script>

{#snippet mapArt(m: CatalogMap, cls: string)}
	{@const url = catalogArtURL(m)}
	{#if url}
		<img src={url} alt="" class="{cls} object-cover" draggable="false" />
	{:else}
		<div class="{cls} flex items-center justify-center bg-surface-100-900">
			<MapIcon class="size-5 opacity-30" />
		</div>
	{/if}
{/snippet}

<div class="flex flex-col gap-4 sm:gap-6">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0 flex-1 basis-80">
			<PageHeader
				title="Maps"
				description="Every unique build parsed off the ingested discs — filename plus content hash, so a modded map shipped under a stock name still gets its own card. Cards are read-only; open one to name it, flag it as a variant, and log its power items."
			/>
		</div>
		<div
			class="flex max-w-72 min-w-0 flex-1 basis-52 items-center gap-2 rounded-lg bg-surface-100-900 px-3 py-2"
		>
			<SearchIcon class="size-4 flex-none opacity-60" />
			<input
				type="search"
				class="w-full min-w-0 border-none bg-transparent text-sm outline-none"
				placeholder="Filter maps"
				bind:value={q}
			/>
		</div>
	</div>

	<!-- game chips (independent toggles) + count -->
	<div class="flex items-center gap-2.5">
		<div class="inline-flex gap-0.5 rounded-lg bg-surface-100-900 p-1">
			{#each [['ce', 'Halo: CE'], ['h2', 'Halo 2']] as [key, label] (key)}
				<button
					class="rounded-md border px-3.5 py-1.5 text-[11px] font-bold tracking-widest uppercase transition-colors
						{games[key]
						? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
						: 'border-transparent text-surface-600-400'}"
					aria-pressed={games[key]}
					onclick={() => {
						games = { ...games, [key]: !games[key] };
						openId = '';
						draft = null;
					}}
				>
					{label}
				</button>
			{/each}
		</div>
		<span class="font-mono text-xs opacity-50">
			{shelf.length}
			{shelf.length === 1 ? 'map' : 'maps'}
		</span>
		{#if loading}<LoaderIcon class="size-4 animate-spin opacity-50" />{/if}
	</div>

	<!-- the shelf -->
	<div class="flex gap-3.5 overflow-x-auto pt-0.5 pr-0.5 pb-2.5 pl-0.5">
		{#each shelf as m (m.id)}
			<button
				class="flex w-49 flex-none flex-col gap-1.5 rounded-xl border bg-surface-50-950 p-2.5 text-left transition-colors hover:border-primary-500/45
					{openId === m.id ? 'border-primary-500/45' : 'border-surface-500/20'}"
				onclick={() => toggleCard(m)}
			>
				{@render mapArt(m, 'w-full aspect-4/3 rounded-md')}
				<span class="flex items-center gap-1.5">
					<span
						class="min-w-0 flex-1 truncate text-sm font-semibold {m.display_name
							? ''
							: 'font-normal opacity-50'}"
					>
						{m.display_name || 'unnamed'}
					</span>
					{#if m.variant_of}
						<span class="badge preset-tonal-primary text-[9px] uppercase">Variant</span>
					{/if}
					<ChevronRightIcon
						class="size-4 flex-none opacity-60 transition-transform {openId === m.id
							? 'rotate-90'
							: ''}"
					/>
				</span>
				<span class="flex items-center gap-1.5">
					<span class="min-w-0 flex-1 truncate font-mono text-[10px] opacity-50">
						{m.filename} · {m.content_hash.slice(0, 6)}
					</span>
					<span
						class="flex-none rounded border border-surface-500/30 bg-surface-500/10 px-1 font-mono text-[9px] tracking-widest opacity-70"
					>
						{m.game === 'ce' ? 'CE' : 'H2'}
					</span>
				</span>
			</button>
		{:else}
			<p class="p-6 text-sm text-surface-500">
				{loading ? 'Loading the catalog…' : 'No maps match — clear the filter.'}
			</p>
		{/each}
	</div>

	<!-- detail / editor -->
	{#if open && draft}
		<Card size="flush" class="flex min-w-0 flex-col">
			<div
				class="flex flex-wrap items-center gap-x-2.5 gap-y-1 border-b border-surface-200-800 p-4"
			>
				<span class="min-w-0 truncate text-sm font-semibold">
					{draft.display_name || 'unnamed'}
				</span>
				<span class="font-mono text-xs opacity-50">
					{open.filename} · {open.content_hash.slice(0, 6)}
				</span>
				{#if openParent}
					<span class="badge preset-tonal-primary text-[10px]">
						Variant · {nameOf(openParent) || openParent.filename}
					</span>
				{/if}
				<span class="flex-1"></span>
				<button
					class="btn-icon preset-tonal btn-sm"
					title="Close"
					onclick={() => toggleCard(open!)}
				>
					<XIcon class="size-4" />
				</button>
			</div>

			<div class="flex flex-col gap-4 p-4">
				<!-- identity: graphic · name/variant · description -->
				<div
					class="grid grid-cols-[96px_minmax(0,1fr)] gap-4 md:grid-cols-[112px_minmax(0,1fr)_minmax(0,1.05fr)]"
				>
					<div class="flex flex-col gap-1.5 md:row-span-2">
						<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
							In-game graphic
						</span>
						<ImageReframe
							bind:this={reframe}
							src={open.graphic_url ? catalogArtURL(open)! : ''}
							targetW={512}
							targetH={512}
							placeholder="Square art"
							onimage={() => (graphicDirty = true)}
						/>
					</div>
					<label class="label min-w-0">
						<span class="label-text">Name</span>
						<input
							class="input"
							placeholder="unnamed — add a name"
							bind:value={draft.display_name}
						/>
						<span class="text-xs opacity-60">What every other surface shows.</span>
					</label>
					<label class="label min-w-0 md:col-start-3 md:row-span-2">
						<span class="label-text">Description</span>
						<textarea
							class="textarea min-h-24 flex-1"
							rows={5}
							placeholder="What organizers should know about this map"
							bind:value={draft.description}
						></textarea>
					</label>
					<div class="col-span-2 flex min-w-0 flex-col gap-1.5 md:col-span-1 md:col-start-2">
						<span class="label-text text-sm">Variant of</span>
						<select class="select" bind:value={draft.variant_of}>
							<option value="">— not a variant —</option>
							{#each parentOptions as p (p.id)}
								<option value={p.id}>{nameOf(p) || p.filename}</option>
							{/each}
						</select>
						<span class="text-xs opacity-60">
							Flags this build as a take on another map — the card gets the Variant chip. Variants
							can't be targets, so chains can't form.
						</span>
						{#if openParent}
							<button
								class="flex items-center gap-2 rounded-md border border-surface-500/20 bg-black/20 p-1.5 text-left transition-colors hover:border-primary-500/45"
								onclick={() => toggleCard(openParent!)}
							>
								{@render mapArt(openParent, 'w-10 h-8 rounded flex-none')}
								<span class="min-w-0 flex-1 truncate text-xs font-semibold">
									{nameOf(openParent) || openParent.filename}
								</span>
								<ChevronRightIcon class="size-4 flex-none opacity-60" />
							</button>
						{/if}
					</div>
				</div>

				<!-- data: power items · discs · variants -->
				<div class="grid grid-cols-1 items-start gap-4 md:grid-cols-2 lg:grid-cols-[1.1fr_1fr_1fr]">
					<div class="flex min-w-0 flex-col gap-1.5 md:col-span-2 lg:col-span-1">
						<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
							Power items
						</span>
						<div class="overflow-hidden rounded-lg border border-surface-500/20">
							<div
								class="grid grid-cols-[1fr_70px_24px] gap-2 bg-black/25 px-3 py-1.5 text-[9px] font-bold tracking-[0.2em] text-surface-500 uppercase"
							>
								<span>Item</span><span>Every</span><span></span>
							</div>
							{#each draft.power_items as row, i (i)}
								<div
									class="grid grid-cols-[1fr_70px_24px] items-center gap-2 border-t border-surface-500/10 px-3 py-2"
								>
									<span class="flex min-w-0 flex-wrap items-center gap-1">
										{#each row.items as it (it)}
											<span class="badge preset-tonal text-[10px]">{it}</span>
										{/each}
										{#if row.items.length > 1}
											<span class="font-mono text-[8px] tracking-widest text-primary-600-400">
												ALTERNATES
											</span>
										{/if}
									</span>
									<span class="font-mono text-xs">{row.every}</span>
									<button
										class="btn-icon opacity-60 btn-sm hover:opacity-100"
										title="Remove row"
										onclick={() =>
											(draft!.power_items = draft!.power_items.filter((_, j) => j !== i))}
									>
										<XIcon class="size-3.5" />
									</button>
								</div>
							{:else}
								<div class="border-t border-surface-500/10 px-3 py-2.5 text-xs text-surface-500">
									No power items logged yet.
								</div>
							{/each}
						</div>
						{#if addRowOpen}
							<div class="flex flex-wrap items-center gap-1.5">
								<input
									class="input min-w-0 flex-1 text-xs"
									placeholder="Item(s), comma-separated — several ALTERNATE each spawn"
									bind:value={addItems}
								/>
								<input
									class="input w-20 font-mono text-xs"
									placeholder="2:00"
									bind:value={addEvery}
								/>
								<button class="btn preset-filled btn-sm" onclick={addPowerRow}>Add</button>
								<button class="btn preset-tonal btn-sm" onclick={() => (addRowOpen = false)}>
									Cancel
								</button>
							</div>
						{:else}
							<button
								class="w-fit rounded-md border border-dashed border-surface-500/40 px-3 py-1 text-[10px] font-bold tracking-widest text-surface-500 uppercase transition-colors hover:border-primary-500/50 hover:text-primary-600-400"
								onclick={() => (addRowOpen = true)}
							>
								+ Add item
							</button>
						{/if}
						<span class="text-xs opacity-60">
							One row per spawn rotation — no spot column, everything is up at map start.
						</span>
					</div>

					<div class="flex min-w-0 flex-col gap-1.5">
						<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
							Seen on discs
						</span>
						<div class="overflow-hidden rounded-lg border border-surface-500/20">
							{#each open.discs as d (d.id)}
								<div
									class="flex items-center gap-3 border-t border-surface-500/10 px-3 py-2 first:border-t-0"
								>
									<span class="min-w-0 flex-1 truncate text-xs">{d.name}</span>
									<span class="font-mono text-[10px] opacity-50">
										{open.content_hash.slice(0, 12)}
									</span>
								</div>
							{:else}
								<div class="px-3 py-2.5 text-xs text-surface-500">
									No disc currently carries this build.
								</div>
							{/each}
						</div>
						<span class="text-xs opacity-60">
							Every disc carrying this exact build — same filename with a different hash is its own
							card, so renamed-in-place mods can't hide.
						</span>
					</div>

					<div class="flex min-w-0 flex-col gap-1.5">
						<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
							Variants of this map
						</span>
						<div class="overflow-hidden rounded-lg border border-surface-500/20">
							{#each children as c (c.id)}
								<button
									class="flex w-full items-center gap-2 border-t border-surface-500/10 px-2.5 py-1.5 text-left transition-colors first:border-t-0 hover:bg-surface-100-900"
									onclick={() => toggleCard(c)}
								>
									{@render mapArt(c, 'w-9 h-7 rounded flex-none')}
									<span class="flex min-w-0 flex-1 flex-col">
										<span
											class="truncate text-xs font-semibold {c.display_name ? '' : 'opacity-50'}"
										>
											{c.display_name || 'unnamed'}
										</span>
										<span class="truncate font-mono text-[9px] opacity-50">
											{c.filename} · {c.content_hash.slice(0, 6)}
										</span>
									</span>
									<ChevronRightIcon class="size-4 flex-none opacity-60" />
								</button>
							{:else}
								<div class="px-3 py-2.5 text-xs text-surface-500">No map flags this one yet.</div>
							{/each}
						</div>
						<span class="text-xs opacity-60">
							Every build whose Variant-of points here — click through to open it.
						</span>
					</div>
				</div>
			</div>

			<div class="border-t border-surface-200-800 bg-black/20 px-4 py-3">
				<DraftBar {dirty} {busy} onsave={save} ondiscard={discard} />
			</div>
		</Card>
	{/if}
</div>
