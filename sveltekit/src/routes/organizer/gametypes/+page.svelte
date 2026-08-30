<script lang="ts">
	// Gametypes — the shared variant library + editor (absorbs the old
	// /organizer/creator/ workspace into the list-beside-editor layout). Every
	// variant is template-patched into a correctly-signed save; the preview
	// column shows exactly what lands on the HDD. A gametype is CE or H2 from
	// creation (picked on + New, baked in) so the editor tailors to it. The CE
	// setting surface stays SERVER-SCHEMA-DRIVEN (lanMeta sections/fields — the
	// live-verified byte map), not a hand-coded field list.
	import { onMount } from 'svelte';
	import {
		AlertTriangleIcon,
		CheckCircle2Icon,
		DownloadIcon,
		FileIcon,
		LoaderIcon,
		SearchIcon,
		SwordsIcon,
		Trash2Icon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import SchemaField from '$lib/components/creator/SchemaField.svelte';
	import MasterDetail from '$lib/components/organizer/MasterDetail.svelte';
	import FilterChips from '$lib/components/organizer/FilterChips.svelte';
	import DraftBar from '$lib/components/organizer/DraftBar.svelte';
	import { lanMeta, lanBuild, lanDownload } from '$lib/utils/lansaves';
	import type {
		BuildRequest,
		BuildResponse,
		CEField,
		CESection,
		LanMeta
	} from '$lib/types/lansaves';
	import type { GametypeRecord, GametypeSettings } from '$lib/types/gamertag';

	type SettingVal = boolean | number | undefined;
	interface EditorState {
		id: string; // '' = new
		title: 'ce' | 'h2';
		engine: string;
		name: string; // library name
		display_name: string; // in-game name
		settings: Record<string, SettingVal>;
	}

	let meta = $state<LanMeta | null>(null);
	let rows = $state<GametypeRecord[]>([]);
	let loading = $state(true);
	let busy = $state(false);
	let ed = $state<EditorState | null>(null);
	let baseline = $state(''); // JSON snapshot of the loaded editor state
	let q = $state('');
	let fGames = $state<Record<string, boolean>>({ ce: true, h2: true });
	let fSides = $state<Record<string, boolean>>({ team: true, ffa: true });
	let fEngines = $state<Record<string, boolean>>({
		slayer: true,
		ctf: true,
		oddball: true,
		king: true,
		race: true
	});
	let newPick = $state(false);

	const ceEngines = $derived(meta?.ce_engines ?? ['slayer', 'ctf', 'oddball', 'king', 'race']);
	const sections = $derived<CESection[]>(meta?.ce_gametype_sections ?? []);
	const scoreUnit = $derived(
		ed?.title === 'h2' ? 'points' : (meta?.ce_score_units?.[ed?.engine ?? 'slayer'] ?? 'points')
	);

	function newEditor(title: 'ce' | 'h2'): EditorState {
		return {
			id: '',
			title,
			engine: 'slayer',
			name: '',
			display_name: '',
			settings: { teams: true, radar: true, friend_indicators: true }
		};
	}
	function editorFor(r: GametypeRecord): EditorState {
		return {
			id: r.id,
			title: r.title,
			engine: r.engine || 'slayer',
			name: r.name,
			display_name: r.display_name || r.name,
			settings: { ...(r.settings ?? {}) }
		};
	}
	function select(r: GametypeRecord) {
		ed = editorFor(r);
		baseline = JSON.stringify(ed);
		newPick = false;
	}
	function startNew(title: 'ce' | 'h2') {
		ed = newEditor(title);
		baseline = JSON.stringify(ed);
		newPick = false;
	}
	const dirty = $derived(!!ed && JSON.stringify(ed) !== baseline);

	// ── Library rows (filter chips AND together; Team/FFA reads Teams) ──────
	const items = $derived.by(() => {
		const query = q.trim().toLowerCase();
		return rows
			.filter((r) => fGames[r.title])
			.filter((r) => (r.settings?.teams ? fSides.team : fSides.ffa))
			.filter((r) => fEngines[r.engine] !== false)
			.filter((r) => {
				if (!query) return true;
				const side = r.settings?.teams ? 'team' : 'ffa';
				return `${r.name} ${r.engine} ${r.title} ${side}`.toLowerCase().includes(query);
			})
			.toSorted((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
	});
	function rowSub(r: GametypeRecord): string {
		const side = r.settings?.teams ? 'team' : 'ffa';
		const ing = r.display_name && r.display_name !== r.name ? ` · “${r.display_name}”` : '';
		return `${r.title.toUpperCase()} · ${r.engine} · ${side}${ing}`;
	}

	/** Schema fields for a section, filtered to the current engine. */
	function fieldsFor(section: string): CEField[] {
		if (!ed) return [];
		const all = meta?.ce_gametype_fields ?? [];
		return all.filter(
			(f) => f.section === section && (!f.engines || f.engines.includes(ed!.engine))
		);
	}
	const relevantKeys = $derived.by(() => {
		if (!ed) return [] as string[];
		const keys: string[] = [];
		for (const f of meta?.ce_gametype_fields ?? []) {
			if (!f.engines || f.engines.includes(ed.engine)) keys.push(f.key);
		}
		return keys;
	});

	function cleanSettings(): GametypeSettings {
		const out: Record<string, SettingVal> = {};
		for (const [k, v] of Object.entries(ed?.settings ?? {})) {
			if (v === undefined || v === null) continue;
			if (ed!.title === 'ce' && k !== 'score_limit' && !relevantKeys.includes(k)) continue;
			out[k] = v;
		}
		return out as GametypeSettings;
	}
	function buildRequest(): BuildRequest {
		const inGame = (ed?.display_name ?? '').trim() || (ed?.name ?? '').trim() || 'Untitled';
		return {
			title: ed?.title ?? 'ce',
			kind: 'gametype',
			engine: ed?.title === 'h2' ? 'slayer' : (ed?.engine ?? 'slayer'),
			name: inGame,
			...cleanSettings()
		};
	}

	// CE pregame lobby list truncates in-game names past 11 chars.
	const inGameName = $derived((ed?.display_name ?? '').trim() || (ed?.name ?? '').trim() || '');
	const truncWarn = $derived(ed?.title === 'ce' && inGameName.length > 11);

	// ── Live save preview (debounced lanBuild) ──────────────────────────────
	let preview = $state<BuildResponse | null>(null);
	let previewErr = $state('');
	let previewing = $state(false);
	let previewTimer: ReturnType<typeof setTimeout> | undefined;
	const reqKey = $derived(ed ? JSON.stringify(buildRequest()) : '');
	$effect(() => {
		void reqKey;
		if (!ed || !inGameName) {
			preview = null;
			previewErr = '';
			return;
		}
		clearTimeout(previewTimer);
		previewTimer = setTimeout(runPreview, 300);
		return () => clearTimeout(previewTimer);
	});
	async function runPreview() {
		previewing = true;
		previewErr = '';
		try {
			preview = await lanBuild(buildRequest());
		} catch (e) {
			preview = null;
			previewErr = e instanceof Error ? e.message : String(e);
		} finally {
			previewing = false;
		}
	}

	// ── Data flow ───────────────────────────────────────────────────────────
	async function load() {
		try {
			loading = true;
			rows = await pb
				.collection('gametypes')
				.getFullList<GametypeRecord>({ expand: 'created_by', sort: 'title,name' });
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	async function save() {
		if (!ed || !auth.user) return;
		const name = ed.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'A library name is required.' });
			return;
		}
		const payload = {
			title: ed.title,
			engine: ed.title === 'h2' ? 'slayer' : ed.engine,
			name,
			display_name: ed.display_name.trim() || name,
			settings: cleanSettings(),
			created_by: auth.user.id
		};
		try {
			busy = true;
			const rec = await toastPromise(
				ed.id
					? pb.collection('gametypes').update<GametypeRecord>(ed.id, payload)
					: pb.collection('gametypes').create<GametypeRecord>(payload),
				{
					loading: { title: 'Saving', description: name },
					success: { title: 'Saved', description: `${name} — signed save generated.` },
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

	async function download() {
		if (!ed || !inGameName) return;
		try {
			await toastPromise(lanDownload(buildRequest(), 'tar'), {
				loading: { title: 'Building', description: inGameName },
				success: { title: 'Downloaded', description: `${inGameName}.tar` },
				errorTitle: 'Download failed'
			});
		} catch {
			/* toast shown */
		}
	}

	async function remove() {
		if (!ed?.id) return;
		const r = rows.find((x) => x.id === ed!.id);
		if (!r) return;
		const ok = await confirmToast({
			title: 'Delete gametype',
			description: `Remove "${r.name}" from the shared library? Rulesets referencing it will drop it.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		try {
			busy = true;
			await toastPromise(pb.collection('gametypes').delete(r.id), {
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

	function fmtBytes(n: number): string {
		if (n < 1024) return `${n} B`;
		if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
		return `${(n / 1024 / 1024).toFixed(1)} MiB`;
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to use the library.' });
			return;
		}
		void load();
		void lanMeta()
			.then((m) => (meta = m))
			.catch((e) => toaster.error({ title: 'Schema load failed', description: String(e) }));
	});
</script>

{#snippet newPicker()}
	{#if newPick}
		<div class="flex items-center gap-1.5 border-b border-surface-200-800 p-2">
			<span class="text-xs opacity-60">New:</span>
			<button class="btn preset-tonal btn-sm" onclick={() => startNew('ce')}>Halo: CE</button>
			<button class="btn preset-tonal btn-sm" onclick={() => startNew('h2')}>Halo 2</button>
			<span class="flex-1"></span>
			<button class="btn-icon preset-tonal btn-sm" onclick={() => (newPick = false)}>✕</button>
		</div>
	{/if}
{/snippet}

{#snippet listPanel()}
	<Card size="flush" class="flex min-w-0 flex-col overflow-hidden">
		<div class="flex items-center justify-between gap-2 border-b border-surface-200-800 p-3">
			<span class="text-sm font-semibold">Library</span>
			<span class="font-mono text-[10px] opacity-50">{items.length} of {rows.length}</span>
			<button class="btn preset-filled btn-sm" onclick={() => (newPick = !newPick)}>+ New</button>
		</div>
		{@render newPicker()}
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
					{ key: 'team', label: 'Team' },
					{ key: 'ffa', label: 'FFA' }
				]}
				bind:active={fSides}
			/>
			<span class="mx-0.5 h-4 w-px bg-surface-500/30"></span>
			<FilterChips
				chips={[
					{ key: 'slayer', label: 'Slayer' },
					{ key: 'ctf', label: 'CTF' },
					{ key: 'oddball', label: 'Ball' },
					{ key: 'king', label: 'King' }
				]}
				bind:active={fEngines}
			/>
		</div>
		<div class="flex items-center gap-2 border-b border-surface-200-800 px-3 py-2">
			<SearchIcon class="size-4 flex-none opacity-60" />
			<input
				type="search"
				class="w-full min-w-0 border-none bg-transparent text-sm outline-none"
				placeholder="Filter variants"
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
					{rows.length === 0 ? 'No variants yet — create one.' : 'No variants match the filters.'}
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
						<SwordsIcon class="size-3.5 flex-none opacity-50" />
						<span class="min-w-0 flex-1">
							<span class="block truncate text-sm font-medium">{r.name}</span>
							<span class="block truncate text-xs opacity-50">{rowSub(r)}</span>
						</span>
						{#if r.save_bundle}
							<span class="size-2 flex-none rounded-full bg-success-500" title="signed save"></span>
						{:else}
							<span class="badge flex-none preset-tonal-error text-[10px]">No file</span>
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
			<p class="text-sm text-surface-500">Pick a variant — or + New to start one.</p>
		</Card>
	{:else}
		<div class="grid min-w-0 grid-cols-1 items-start gap-4 xl:grid-cols-[minmax(0,1fr)_260px]">
			<Card class="flex min-w-0 flex-col gap-4">
				<!-- header -->
				<div class="flex flex-wrap items-center gap-2">
					<h3 class="min-w-0 truncate h4">{ed.id ? ed.name || 'untitled' : 'New gametype'}</h3>
					<span class="badge preset-tonal font-mono text-[10px] uppercase">
						{ed.title} · {ed.title === 'h2' ? 'slayer' : ed.engine}
					</span>
					{#if ed.id}
						{@const rec = rows.find((r) => r.id === ed!.id)}
						{#if rec?.save_bundle}
							<span class="badge preset-tonal-success">
								<CheckCircle2Icon class="size-3" /> signed
							</span>
						{:else}
							<span class="badge preset-tonal-error">No file</span>
						{/if}
					{/if}
					<span class="flex-1"></span>
					<button class="btn preset-tonal btn-sm" onclick={download} disabled={busy || !inGameName}>
						<DownloadIcon class="size-4" /><span>Download .tar</span>
					</button>
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

				<!-- identity -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
					<label class="flex flex-col gap-1">
						<span class="text-sm font-medium">Library name</span>
						<input
							class="input"
							bind:value={ed.name}
							maxlength={64}
							placeholder="e.g. Team Slayer 50"
							disabled={busy}
						/>
						<span class="text-xs opacity-60">What rulesets and this list show.</span>
					</label>
					<label class="flex flex-col gap-1">
						<span class="text-sm font-medium">In-game name</span>
						<input
							class="input"
							bind:value={ed.display_name}
							maxlength={ed.title === 'h2' ? 23 : 64}
							placeholder="written into the save"
							disabled={busy}
						/>
						<span class="text-xs opacity-60">
							What the lobby shows — blank rides the library name.
						</span>
					</label>
					<label class="flex flex-col gap-1">
						<span class="text-sm font-medium">{ed.title === 'h2' ? 'Mode' : 'Engine'}</span>
						{#if ed.title === 'h2'}
							<input class="input" value="slayer" disabled />
						{:else}
							<select class="select" bind:value={ed.engine} disabled={busy}>
								{#each ceEngines as e (e)}<option value={e}>{e}</option>{/each}
							</select>
						{/if}
					</label>
				</div>

				{#if truncWarn}
					<p class="flex items-center gap-2 text-xs text-warning-600-400">
						<AlertTriangleIcon class="size-3.5" />
						Truncates to “{inGameName.slice(0, 11)}…” on the CE pregame lobby list.
					</p>
				{/if}

				{#if ed.title === 'h2'}
					<p class="text-sm text-surface-500">
						Halo 2 gametype mapping currently covers name + score limit only; other settings are
						preserved from the template.
					</p>
					<SchemaField
						field={{
							key: 'score_limit',
							label: `Score limit (${scoreUnit})`,
							kind: 'int',
							section: 'game',
							min: 0
						}}
						bind:value={ed.settings.score_limit}
						disabled={busy}
					/>
				{:else}
					<!-- CE: server-schema-driven sections (the game's own menus) -->
					{#each sections as sec (sec.id)}
						{@const fields = fieldsFor(sec.id)}
						{#if fields.length}
							<section class="flex flex-col gap-3">
								<h4 class="text-xs font-semibold tracking-wide text-surface-500 uppercase">
									{sec.label}
								</h4>
								<div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
									{#each fields as f (f.key)}
										<SchemaField
											field={f.key === 'score_limit'
												? { ...f, label: `Score limit (${scoreUnit})` }
												: f}
											bind:value={ed.settings[f.key]}
											disabled={busy}
										/>
									{/each}
								</div>
							</section>
						{/if}
					{/each}
				{/if}

				<DraftBar
					{dirty}
					{busy}
					saveLabel={ed.id ? 'Save changes' : 'Save to library'}
					onsave={save}
					ondiscard={discard}
				/>
			</Card>

			<!-- save-file preview column -->
			<Card class="flex flex-col gap-3">
				<div class="flex items-center justify-between">
					<h4 class="text-sm font-semibold">Save preview</h4>
					{#if previewing}<LoaderIcon class="size-4 animate-spin text-surface-500" />{/if}
				</div>
				{#if previewErr}
					<p class="flex items-center gap-2 text-sm text-error-500">
						<AlertTriangleIcon class="size-4" />{previewErr}
					</p>
				{:else if !preview}
					<p class="text-sm text-surface-500">Enter a name to preview the generated save.</p>
				{:else}
					<div class="flex flex-wrap items-center gap-2 text-sm">
						{#if preview.digest.resolved}
							<span class="badge preset-tonal-success">
								<CheckCircle2Icon class="size-3.5" /> signed
							</span>
						{:else}
							<span class="badge preset-tonal-error">unsigned</span>
						{/if}
						<span class="badge preset-tonal">{preview.title_id}</span>
					</div>
					<code class="text-xs break-all text-surface-500">
						E:\{preview.fatx_dir.replace(/\//g, '\\')}
					</code>
					<div class="flex flex-col gap-1.5 text-sm">
						{#each preview.files as f (f.name)}
							<div class="flex items-center gap-1.5 rounded bg-surface-100-900 px-2 py-1">
								<FileIcon class="size-3.5 opacity-60" />
								<span class="truncate">{f.name}</span>
								<span class="ml-auto text-xs text-surface-500">{f.size} B</span>
							</div>
						{/each}
					</div>
					<p class="text-xs text-surface-500">
						On-disk footprint: {fmtBytes(preview.footprint_bytes)} (FATX cluster {preview.fatx_cluster}
						B)
					</p>
					{#if preview.warnings?.length}
						<ul class="flex flex-col gap-1">
							{#each preview.warnings as w (w)}
								<li class="flex items-start gap-2 text-xs text-warning-600-400">
									<AlertTriangleIcon class="mt-0.5 size-3.5 shrink-0" />{w}
								</li>
							{/each}
						</ul>
					{/if}
				{/if}
			</Card>
		</div>
	{/if}
{/snippet}

<div class="flex flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Gametypes"
		description="The shared variant library + editor. Every change is template-patched into a real, correctly-signed save — the preview shows exactly what lands on the HDD. A gametype is CE or H2 from creation; the editor tailors to it."
	/>

	<MasterDetail
		open={!!ed}
		onback={() => {
			ed = null;
			baseline = '';
		}}
		backLabel="Library"
		listWidth="270px"
		list={listPanel}
		detail={detailPanel}
	/>
</div>
