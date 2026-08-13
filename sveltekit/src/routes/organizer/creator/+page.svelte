<script lang="ts">
	import { onMount } from 'svelte';
	import {
		LoaderIcon,
		PlusIcon,
		Trash2Icon,
		DownloadIcon,
		SaveIcon,
		SwordsIcon,
		CheckCircle2Icon,
		AlertTriangleIcon,
		FileIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import SchemaField from '$lib/components/creator/SchemaField.svelte';
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
		id: string;
		title: 'ce' | 'h2';
		engine: string;
		name: string;
		settings: Record<string, SettingVal>;
	}

	let meta = $state<LanMeta | null>(null);
	let rows = $state<GametypeRecord[]>([]);
	let loading = $state(true);
	let ed = $state<EditorState>(newEditor());
	let busy = $state(false);
	let preview = $state<BuildResponse | null>(null);
	let previewErr = $state('');
	let previewing = $state(false);

	function newEditor(): EditorState {
		// Defaults mirror a fresh CE variant (radar + friend indicators on).
		return {
			id: '',
			title: 'ce',
			engine: 'slayer',
			name: '',
			settings: { teams: true, radar: true, friend_indicators: true }
		};
	}

	const ceEngines = $derived(meta?.ce_engines ?? ['slayer', 'ctf', 'oddball', 'king', 'race']);
	const sections = $derived<CESection[]>(meta?.ce_gametype_sections ?? []);
	const scoreUnit = $derived(
		ed.title === 'h2' ? 'points' : (meta?.ce_score_units?.[ed.engine] ?? 'points')
	);

	/** Schema fields for a section, filtered to the current engine. */
	function fieldsFor(section: string): CEField[] {
		const all = meta?.ce_gametype_fields ?? [];
		return all.filter(
			(f) => f.section === section && (!f.engines || f.engines.includes(ed.engine))
		);
	}

	/** Schema keys relevant to the current engine (used to prune settings from
	 * other engines when building/saving). */
	const relevantKeys = $derived.by(() => {
		const keys: string[] = [];
		for (const f of meta?.ce_gametype_fields ?? []) {
			if (!f.engines || f.engines.includes(ed.engine)) keys.push(f.key);
		}
		return keys;
	});

	function cleanSettings(): GametypeSettings {
		const out: Record<string, SettingVal> = {};
		for (const [k, v] of Object.entries(ed.settings)) {
			if (v === undefined || v === null) continue;
			if (ed.title === 'ce' && k !== 'score_limit' && !relevantKeys.includes(k)) continue;
			out[k] = v;
		}
		return out as GametypeSettings;
	}

	function buildRequest(): BuildRequest {
		return {
			title: ed.title,
			kind: 'gametype',
			engine: ed.title === 'h2' ? 'slayer' : ed.engine,
			name: ed.name.trim() || 'Untitled',
			...cleanSettings()
		};
	}

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

	function selectNew() {
		ed = newEditor();
	}

	function selectRow(r: GametypeRecord) {
		ed = {
			id: r.id,
			title: r.title,
			engine: r.engine || 'slayer',
			name: r.name,
			settings: { ...(r.settings ?? {}) }
		};
	}

	function onTitleChange() {
		if (ed.title === 'h2') ed.engine = 'slayer';
		else if (!ceEngines.includes(ed.engine)) ed.engine = ceEngines[0] ?? 'slayer';
	}

	// ---- live preview (debounced) ----
	let previewTimer: ReturnType<typeof setTimeout> | undefined;
	const reqKey = $derived(JSON.stringify(buildRequest()));
	$effect(() => {
		void reqKey; // track
		if (!ed.name.trim()) {
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

	async function save() {
		const name = ed.name.trim();
		if (!name) {
			toaster.error({ title: 'Invalid', description: 'A variant name is required.' });
			return;
		}
		if (!auth.user) return;
		const payload = {
			title: ed.title,
			engine: ed.title === 'h2' ? 'slayer' : ed.engine,
			name,
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
			await load();
		} catch {
			/* toast shown */
		} finally {
			busy = false;
		}
	}

	async function download() {
		if (!ed.name.trim()) return;
		try {
			await toastPromise(lanDownload(buildRequest(), 'tar'), {
				loading: { title: 'Building', description: ed.name },
				success: { title: 'Downloaded', description: `${ed.name}.tar` },
				errorTitle: 'Download failed'
			});
		} catch {
			/* toast shown */
		}
	}

	async function remove(r: GametypeRecord, e: MouseEvent) {
		e.stopPropagation();
		const ok = await confirmToast({
			title: 'Delete gametype',
			description: `Remove "${r.name}" from the shared library?`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		try {
			await toastPromise(pb.collection('gametypes').delete(r.id), {
				loading: { title: 'Deleting', description: r.name },
				success: { title: 'Deleted', description: r.name },
				errorTitle: 'Delete failed'
			});
			if (ed.id === r.id) selectNew();
			await load();
		} catch {
			/* toast shown */
		}
	}

	function fmtBytes(n: number): string {
		if (n < 1024) return `${n} B`;
		if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
		return `${(n / 1024 / 1024).toFixed(1)} MiB`;
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to use the creator.' });
			return;
		}
		void load();
		void lanMeta()
			.then((m) => (meta = m))
			.catch((e) => toaster.error({ title: 'Schema load failed', description: String(e) }));
	});
</script>

<div class="flex flex-col gap-4">
	<PageHeader
		title="Gametype creator"
		description="Design a Halo: CE or Halo 2 multiplayer variant with the full in-game setting surface. Every change is template-patched into a real, correctly-signed save — preview it live, then save it to the shared library or download the .tar."
	/>

	<div class="grid grid-cols-1 gap-4 lg:grid-cols-[18rem_1fr]">
		<!-- Library sidebar -->
		<Card size="flush" class="flex flex-col overflow-hidden">
			<div class="flex items-center justify-between border-b border-surface-200-800 p-3">
				<span class="text-sm font-semibold">Library</span>
				<button class="btn preset-filled btn-sm" onclick={selectNew}>
					<PlusIcon class="size-4" /><span>New</span>
				</button>
			</div>
			<div class="flex max-h-[28rem] flex-col overflow-y-auto lg:max-h-[60vh]">
				{#if loading}
					<div class="flex items-center gap-2 p-4 text-sm text-surface-500">
						<LoaderIcon class="size-4 animate-spin" /> Loading…
					</div>
				{:else if rows.length === 0}
					<p class="p-4 text-sm text-surface-500">No variants yet — create one.</p>
				{:else}
					{#each rows as r (r.id)}
						<div
							class="flex items-center gap-2 border-b border-surface-100-900 hover:bg-surface-100-900 {ed.id ===
							r.id
								? 'bg-primary-500/10'
								: ''}"
						>
							<button
								class="flex min-w-0 flex-1 items-center gap-2 px-3 py-2 text-left"
								onclick={() => selectRow(r)}
							>
								<SwordsIcon class="size-3.5 shrink-0 opacity-60" />
								<span class="min-w-0 flex-1">
									<span class="block truncate text-sm font-medium">{r.name}</span>
									<span class="block text-xs text-surface-500">
										{r.title.toUpperCase()} · {r.engine}
									</span>
								</span>
								{#if !r.save_bundle}
									<span class="badge preset-tonal-error text-xs">no file</span>
								{/if}
							</button>
							<button
								class="mr-2 btn-icon shrink-0 preset-tonal-error btn-sm"
								title="Delete"
								onclick={(e) => remove(r, e)}
							>
								<Trash2Icon class="size-3.5" />
							</button>
						</div>
					{/each}
				{/if}
			</div>
		</Card>

		<!-- Editor + preview -->
		<div class="flex flex-col gap-4">
			<Card class="flex flex-col gap-4">
				<!-- header row -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<label class="flex flex-col gap-1">
						<span class="text-sm font-medium">Title</span>
						<select class="select" bind:value={ed.title} onchange={onTitleChange} disabled={busy}>
							<option value="ce">Halo: CE</option>
							<option value="h2">Halo 2</option>
						</select>
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
					<label class="flex flex-col gap-1">
						<span class="text-sm font-medium">Variant name</span>
						<input
							class="input"
							bind:value={ed.name}
							maxlength="64"
							placeholder="e.g. Team Slayer 50"
							disabled={busy}
						/>
					</label>
				</div>

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
					<!-- CE: schema-driven sections -->
					{#each sections as sec (sec.id)}
						{@const fields = fieldsFor(sec.id)}
						{#if fields.length}
							<section class="flex flex-col gap-3">
								<h3 class="text-xs font-semibold tracking-wide text-surface-500 uppercase">
									{sec.label}
								</h3>
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

				<div class="flex flex-wrap justify-end gap-2 border-t border-surface-200-800 pt-3">
					<button class="btn preset-tonal" onclick={download} disabled={busy || !ed.name.trim()}>
						<DownloadIcon class="size-4" /><span>Download .tar</span>
					</button>
					<button class="btn preset-filled" onclick={save} disabled={busy || !ed.name.trim()}>
						{#if busy}<LoaderIcon class="size-4 animate-spin" />{:else}<SaveIcon
								class="size-4"
							/>{/if}
						<span>{ed.id ? 'Save changes' : 'Save to library'}</span>
					</button>
				</div>
			</Card>

			<!-- live preview -->
			<Card class="flex flex-col gap-3">
				<div class="flex items-center justify-between">
					<h3 class="text-sm font-semibold">Live preview</h3>
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
						<code class="text-xs text-surface-500">E:\{preview.fatx_dir.replace(/\//g, '\\')}</code>
					</div>
					<div class="grid grid-cols-2 gap-2 text-sm sm:grid-cols-4">
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
	</div>
</div>
