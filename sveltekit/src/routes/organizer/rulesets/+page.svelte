<script lang="ts">
	// Rulesets — how a night runs: gametypes + a map pool at a team size and
	// series length. References the other two libraries — gametypes carry their
	// signed-save status in, maps bring their art. CE-or-H2 is picked at
	// creation so both pickers only offer same-game content; an EMPTY pool
	// reads as an open pool (organizer picks at the sticks). Series and
	// stations reference rulesets in a later phase.
	import { onMount } from 'svelte';
	import {
		AlertTriangleIcon,
		LoaderIcon,
		ScrollTextIcon,
		SearchIcon,
		Trash2Icon,
		XIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import MasterDetail from '$lib/components/organizer/MasterDetail.svelte';
	import FilterChips from '$lib/components/organizer/FilterChips.svelte';
	import DraftBar from '$lib/components/organizer/DraftBar.svelte';
	import { catalogArtURL, listMapsCatalog, type CatalogMap } from '$lib/utils/isos';
	import type { GametypeRecord } from '$lib/types/gamertag';

	interface RulesetRecord {
		id: string;
		name: string;
		game: 'ce' | 'h2';
		team_size: string;
		series: string;
		gametypes: string[];
		map_pool: string[];
		notes: string;
		created: string;
		updated: string;
	}
	interface EditorState {
		id: string; // '' = new
		name: string;
		game: 'ce' | 'h2';
		team_size: string;
		series: string;
		gametypes: string[];
		map_pool: string[];
		notes: string;
	}

	const TEAM_SIZES = [
		['1v1', '1v1'],
		['2v2', '2v2'],
		['4v4', '4v4'],
		['open', 'Open']
	] as const;
	const SERIES = ['bo1', 'bo2', 'bo3', 'bo4', 'bo5', 'bo6', 'bo7'];
	const seriesLabel = (s: string) => (s ? 'Bo' + s.slice(2) : '—');
	const sizeLabel = (s: string) => TEAM_SIZES.find(([k]) => k === s)?.[1] ?? s;

	let rows = $state<RulesetRecord[]>([]);
	let gametypes = $state<GametypeRecord[]>([]);
	let maps = $state<CatalogMap[]>([]);
	let loading = $state(true);
	let busy = $state(false);
	let q = $state('');
	let fGames = $state<Record<string, boolean>>({ ce: true, h2: true });
	let fSizes = $state<Record<string, boolean>>({
		'1v1': true,
		'2v2': true,
		'4v4': true,
		open: true
	});
	let newPick = $state(false);
	let ed = $state<EditorState | null>(null);
	let baseline = $state('');

	const gtById = $derived(new Map(gametypes.map((g) => [g.id, g])));
	const mapById = $derived(new Map(maps.map((m) => [m.id, m])));

	function editorFor(r: RulesetRecord): EditorState {
		return {
			id: r.id,
			name: r.name,
			game: r.game,
			team_size: r.team_size,
			series: r.series,
			gametypes: [...(r.gametypes ?? [])],
			map_pool: [...(r.map_pool ?? [])],
			notes: r.notes
		};
	}
	function select(r: RulesetRecord) {
		ed = editorFor(r);
		baseline = JSON.stringify(ed);
		newPick = false;
	}
	function startNew(game: 'ce' | 'h2') {
		ed = {
			id: '',
			name: '',
			game,
			team_size: '2v2',
			series: 'bo5',
			gametypes: [],
			map_pool: [],
			notes: ''
		};
		baseline = JSON.stringify(ed);
		newPick = false;
	}
	const dirty = $derived(!!ed && JSON.stringify(ed) !== baseline);

	function unsignedIn(ids: string[]): GametypeRecord[] {
		return ids
			.map((id) => gtById.get(id))
			.filter((g): g is GametypeRecord => !!g && !g.save_bundle);
	}
	function rowSub(r: RulesetRecord): string {
		const pool = r.map_pool?.length ? `${r.map_pool.length}-map pool` : 'open pool';
		return `${r.game.toUpperCase()} · ${sizeLabel(r.team_size)} · ${seriesLabel(r.series)} · ${r.gametypes?.length ?? 0} gt · ${pool}`;
	}

	const items = $derived.by(() => {
		const query = q.trim().toLowerCase();
		return rows
			.filter((r) => fGames[r.game])
			.filter((r) => fSizes[r.team_size] !== false)
			.filter(
				(r) =>
					!query || `${r.name} ${r.game} ${r.team_size} ${r.series}`.toLowerCase().includes(query)
			)
			.toSorted((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
	});

	// Same-game pickers, minus what's already in the set.
	const gtOptions = $derived(
		ed
			? gametypes
					.filter((g) => g.title === ed!.game && !ed!.gametypes.includes(g.id))
					.toSorted((a, b) => a.name.localeCompare(b.name))
			: []
	);
	const mapOptions = $derived(
		ed
			? maps
					.filter((m) => m.game === ed!.game && !ed!.map_pool.includes(m.id))
					.toSorted((a, b) =>
						(a.display_name || a.filename).localeCompare(b.display_name || b.filename)
					)
			: []
	);
	const edUnsigned = $derived(ed ? unsignedIn(ed.gametypes) : []);

	async function load() {
		try {
			loading = true;
			[rows, gametypes, maps] = await Promise.all([
				pb.collection('rulesets').getFullList<RulesetRecord>({ sort: 'name' }),
				pb.collection('gametypes').getFullList<GametypeRecord>({ sort: 'title,name' }),
				listMapsCatalog()
			]);
		} catch (err) {
			toaster.error({ title: 'Load rulesets failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	async function save() {
		if (!ed) return;
		const name = ed.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'A ruleset name is required.' });
			return;
		}
		const payload = {
			name,
			game: ed.game,
			team_size: ed.team_size,
			series: ed.series,
			gametypes: ed.gametypes,
			map_pool: ed.map_pool,
			notes: ed.notes.trim(),
			...(ed.id ? {} : { created_by: auth.user?.id ?? '' })
		};
		try {
			busy = true;
			const rec = await toastPromise(
				ed.id
					? pb.collection('rulesets').update<RulesetRecord>(ed.id, payload)
					: pb.collection('rulesets').create<RulesetRecord>(payload),
				{
					loading: { title: 'Saving', description: name },
					success: { title: 'Saved', description: name },
					errorTitle: 'Save failed'
				}
			);
			ed.id = rec.id;
			baseline = JSON.stringify(ed);
			await load();
		} catch {
			/* toast shown */
		} finally {
			busy = false;
		}
	}
	function discard() {
		if (!ed) return;
		if (ed.id) {
			const r = rows.find((x) => x.id === ed!.id);
			if (r) select(r);
		} else {
			ed = null;
			baseline = '';
		}
	}

	async function remove() {
		if (!ed?.id) return;
		const r = rows.find((x) => x.id === ed!.id);
		if (!r) return;
		const ok = await confirmToast({
			title: 'Delete ruleset',
			description: `Remove "${r.name}" from the library?`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		try {
			busy = true;
			await toastPromise(pb.collection('rulesets').delete(r.id), {
				loading: { title: 'Deleting', description: r.name },
				success: { title: 'Deleted', description: r.name },
				errorTitle: 'Delete failed'
			});
			ed = null;
			baseline = '';
			await load();
		} catch {
			/* toast shown */
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage rulesets.' });
			return;
		}
		void load();
	});
</script>

{#snippet listPanel()}
	<Card size="flush" class="flex min-w-0 flex-col overflow-hidden">
		<div class="flex items-center justify-between gap-2 border-b border-surface-200-800 p-3">
			<span class="text-sm font-semibold">Library</span>
			<span class="font-mono text-[10px] opacity-50">{items.length} of {rows.length}</span>
			<button class="btn preset-filled btn-sm" onclick={() => (newPick = !newPick)}>+ New</button>
		</div>
		{#if newPick}
			<div class="flex items-center gap-1.5 border-b border-surface-200-800 p-2">
				<span class="text-xs opacity-60">New:</span>
				<button class="btn preset-tonal btn-sm" onclick={() => startNew('ce')}>Halo: CE</button>
				<button class="btn preset-tonal btn-sm" onclick={() => startNew('h2')}>Halo 2</button>
				<span class="flex-1"></span>
				<button class="btn-icon preset-tonal btn-sm" onclick={() => (newPick = false)}>✕</button>
			</div>
		{/if}
		<div class="flex flex-wrap items-center gap-1.5 border-b border-surface-200-800 p-2">
			<FilterChips
				chips={[
					{ key: 'ce', label: 'CE' },
					{ key: 'h2', label: 'H2' }
				]}
				bind:active={fGames}
			/>
			<span class="mx-0.5 h-4 w-px bg-surface-500/30"></span>
			<FilterChips
				chips={[
					{ key: '1v1', label: '1v1' },
					{ key: '2v2', label: '2v2' },
					{ key: '4v4', label: '4v4' },
					{ key: 'open', label: 'Open' }
				]}
				bind:active={fSizes}
			/>
		</div>
		<div class="flex items-center gap-2 border-b border-surface-200-800 px-3 py-2">
			<SearchIcon class="size-4 flex-none opacity-60" />
			<input
				type="search"
				class="w-full min-w-0 border-none bg-transparent text-sm outline-none"
				placeholder="Filter rulesets"
				bind:value={q}
			/>
		</div>
		<div class="flex max-h-[60vh] flex-col overflow-y-auto">
			{#if loading}
				<div class="flex items-center gap-2 p-4 text-sm text-surface-500">
					<LoaderIcon class="size-4 animate-spin" /> Loading…
				</div>
			{:else if items.length === 0}
				<p class="p-4 text-sm text-surface-500">
					{rows.length === 0 ? 'No rulesets yet — create one.' : 'No rulesets match the filters.'}
				</p>
			{:else}
				{#each items as r (r.id)}
					<button
						class="flex w-full items-center gap-2.5 border-b border-surface-100-900 px-3 py-2 text-left transition-colors hover:bg-surface-100-900
							{ed?.id === r.id
							? 'border-l-2 border-l-primary-500 bg-primary-500/10'
							: 'border-l-2 border-l-transparent'}"
						onclick={() => select(r)}
					>
						<ScrollTextIcon class="size-3.5 flex-none opacity-50" />
						<span class="min-w-0 flex-1">
							<span class="block truncate text-sm font-medium">{r.name}</span>
							<span class="block truncate text-xs opacity-50">{rowSub(r)}</span>
						</span>
						{#if unsignedIn(r.gametypes ?? []).length > 0}
							<span
								class="badge flex-none preset-tonal-error text-[10px]"
								title="a member gametype has no signed save"
							>
								No file
							</span>
						{/if}
					</button>
				{/each}
			{/if}
		</div>
	</Card>
{/snippet}

{#snippet detailPanel()}
	{#if !ed}
		<Card class="flex min-h-40 items-center justify-center">
			<p class="text-sm text-surface-500">Pick a ruleset — or + New to start one.</p>
		</Card>
	{:else}
		<Card class="flex min-w-0 flex-col gap-4">
			<!-- header -->
			<div class="flex flex-wrap items-center gap-2">
				<h3 class="min-w-0 truncate h4">{ed.id ? ed.name || 'untitled' : 'New ruleset'}</h3>
				<span class="badge preset-tonal">{ed.game === 'ce' ? 'Halo: CE' : 'Halo 2'}</span>
				<span class="badge preset-tonal-surface">
					{sizeLabel(ed.team_size)} · {seriesLabel(ed.series)}
				</span>
				<span class="flex-1"></span>
				{#if ed.id}
					<button
						class="btn-icon preset-tonal-error btn-sm"
						title="Delete"
						onclick={remove}
						disabled={busy}
					>
						<Trash2Icon class="size-4" />
					</button>
				{/if}
			</div>

			{#if edUnsigned.length > 0}
				<p
					class="flex items-start gap-2 rounded border border-error-500/40 bg-error-500/10 p-2 text-xs"
				>
					<AlertTriangleIcon class="mt-0.5 size-3.5 flex-none text-error-500" />
					<span>
						{edUnsigned.map((g) => g.name).join(', ')}
						{edUnsigned.length === 1 ? 'has' : 'have'} no signed save — generate it in Gametypes before
						this ruleset goes live.
					</span>
				</p>
			{/if}

			<!-- identity -->
			<div
				class="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1.4fr)_minmax(0,0.8fr)_minmax(0,0.8fr)]"
			>
				<label class="flex flex-col gap-1">
					<span class="text-sm font-medium">Name</span>
					<input
						class="input"
						bind:value={ed.name}
						maxlength={120}
						placeholder="e.g. NHE 2v2 — standard"
						disabled={busy}
					/>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-sm font-medium">Team size</span>
					<select class="select" bind:value={ed.team_size} disabled={busy}>
						{#each TEAM_SIZES as [key, label] (key)}
							<option value={key}>{label}</option>
						{/each}
					</select>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-sm font-medium">Series</span>
					<select class="select" bind:value={ed.series} disabled={busy}>
						{#each SERIES as s (s)}
							<option value={s}>{seriesLabel(s)}</option>
						{/each}
					</select>
				</label>
			</div>

			<!-- gametypes + map pool -->
			<div class="grid grid-cols-1 items-start gap-4 sm:grid-cols-2">
				<div class="flex min-w-0 flex-col gap-1.5">
					<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
						Gametypes
					</span>
					<div class="overflow-hidden rounded-lg border border-surface-500/20">
						{#each ed.gametypes as id, i (id)}
							{@const g = gtById.get(id)}
							<div
								class="flex items-center gap-2 border-t border-surface-500/10 px-3 py-2 first:border-t-0"
							>
								<span class="min-w-0 flex-1 truncate text-sm">{g?.name ?? id}</span>
								<span class="badge preset-tonal font-mono text-[9px]">{g?.engine ?? '?'}</span>
								{#if g?.save_bundle}
									<span class="size-2 flex-none rounded-full bg-success-500" title="signed"></span>
								{:else}
									<span class="badge flex-none preset-tonal-error text-[9px]">No file</span>
								{/if}
								<button
									class="btn-icon flex-none opacity-60 btn-sm hover:opacity-100"
									title="Remove"
									onclick={() => (ed!.gametypes = ed!.gametypes.filter((_, j) => j !== i))}
								>
									<XIcon class="size-3.5" />
								</button>
							</div>
						{:else}
							<div class="px-3 py-2.5 text-xs text-surface-500">No gametypes yet.</div>
						{/each}
					</div>
					<select
						class="select"
						disabled={busy || gtOptions.length === 0}
						onchange={(e) => {
							const v = e.currentTarget.value;
							if (v) ed!.gametypes = [...ed!.gametypes, v];
							e.currentTarget.value = '';
						}}
					>
						<option value="">+ add from the library…</option>
						{#each gtOptions as g (g.id)}
							<option value={g.id}>{g.name}{g.save_bundle ? '' : ' (no file)'}</option>
						{/each}
					</select>
				</div>

				<div class="flex min-w-0 flex-col gap-1.5">
					<span class="text-[10px] font-bold tracking-widest text-surface-500 uppercase">
						Map pool
					</span>
					<div class="overflow-hidden rounded-lg border border-surface-500/20">
						{#each ed.map_pool as id, i (id)}
							{@const m = mapById.get(id)}
							<div
								class="flex items-center gap-2 border-t border-surface-500/10 px-2.5 py-1.5 first:border-t-0"
							>
								{#if m && catalogArtURL(m)}
									<img
										src={catalogArtURL(m)}
										alt=""
										class="h-7 w-9 flex-none rounded object-cover"
										draggable="false"
									/>
								{:else}
									<span class="h-7 w-9 flex-none rounded bg-surface-100-900"></span>
								{/if}
								<span class="min-w-0 flex-1 truncate text-sm">
									{m ? m.display_name || m.filename : id}
								</span>
								<button
									class="btn-icon flex-none opacity-60 btn-sm hover:opacity-100"
									title="Remove"
									onclick={() => (ed!.map_pool = ed!.map_pool.filter((_, j) => j !== i))}
								>
									<XIcon class="size-3.5" />
								</button>
							</div>
						{:else}
							<div class="px-3 py-2.5 text-xs text-surface-500">
								Open pool — the organizer picks at the sticks.
							</div>
						{/each}
					</div>
					<select
						class="select"
						disabled={busy || mapOptions.length === 0}
						onchange={(e) => {
							const v = e.currentTarget.value;
							if (v) ed!.map_pool = [...ed!.map_pool, v];
							e.currentTarget.value = '';
						}}
					>
						<option value="">+ add from Maps…</option>
						{#each mapOptions as m (m.id)}
							<option value={m.id}>{m.display_name || m.filename}</option>
						{/each}
					</select>
				</div>
			</div>

			<label class="label">
				<span class="label-text">Notes <span class="opacity-50">(optional)</span></span>
				<textarea
					class="textarea"
					rows={2}
					placeholder="Anything the night's organizer should know"
					bind:value={ed.notes}
				></textarea>
			</label>

			<DraftBar
				{dirty}
				{busy}
				saveLabel={ed.id ? 'Save changes' : 'Save to library'}
				onsave={save}
				ondiscard={discard}
			/>
		</Card>
	{/if}
{/snippet}

<div class="flex flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Rulesets"
		description="The unit a night runs on: which gametypes, on which maps, at what team size and series length. Series and stations reference rulesets; unsigned member gametypes bubble a warning here until their save is generated."
	/>

	<MasterDetail
		open={!!ed}
		onback={() => {
			ed = null;
			baseline = '';
		}}
		backLabel="Library"
		list={listPanel}
		detail={detailPanel}
	/>
</div>
