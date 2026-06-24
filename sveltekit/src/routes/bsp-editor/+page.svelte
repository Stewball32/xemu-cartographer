<script lang="ts">
	// BSP spectator-mesh editor (/bsp-editor). A purpose-built dev tool — NOT a
	// general 3D editor — for culling a Halo map's extracted structure-BSP down to
	// a clean "spectator mesh": drop the roof/ceilings + out-of-bounds clutter, keep
	// the walkable interior, then bake the result so BOTH visualizer views render it.
	//
	// It loads the SAME geometry cache the visualizers consume (loadBspMesh raw),
	// and its export is a culled BspMesh in the identical JSON schema — so the baked
	// asset just replaces the raw one through the loader (manifest `spectator_file`),
	// with no view changes. See BSP-EDITOR.md for the edit→export→visualizer loop.
	import { onMount, onDestroy } from 'svelte';
	import * as THREE from 'three';
	import { GLTFExporter } from 'three/examples/jsm/exporters/GLTFExporter.js';
	import {
		loadBspMesh,
		loadGeometryManifest,
		normalizeMesh,
		type BspMesh,
		type GeometryManifest
	} from '$lib/utils/game-geometry';
	import { haloToThree } from '$lib/utils/viz3d';
	import { computeFloorBands, bandColor } from '$lib/utils/floor-bands';
	import {
		buildTriMeta,
		connectedComponents,
		selectAbovePlane,
		selectComponent,
		selectByMaterial,
		defaultCullZ,
		floorMarkers as floorMarkersOf,
		exportSpectatorMesh,
		editStats,
		RemovedHistory,
		type TriMeta,
		type PickMods,
		type PlaneCullMode,
		type SpectatorBand
	} from '$lib/utils/bsp-edit';
	import EditorScene from '$lib/components/bsp-editor/EditorScene.svelte';

	const GAME = 'haloce';

	// --- Map catalogue + loaded mesh -------------------------------------------
	let manifest = $state<GeometryManifest | null>(null);
	let manifestTried = $state(false);
	let activeKey = $state('');
	let mesh = $state<BspMesh | null>(null);
	let loading = $state(false);
	let status = $state('');

	const mapList = $derived(
		manifest
			? Object.entries(manifest.meshes).map(([key, e]) => ({
					key,
					scenario: e.scenario ?? key,
					tris: e.triangle_count ?? 0,
					baked: !!e.spectator_file
				}))
			: []
	);

	// --- Edit state -------------------------------------------------------------
	let meta = $state<TriMeta[]>([]);
	let comp = $state<Int32Array>(new Int32Array(0));
	let removed = $state<Uint8Array>(new Uint8Array(0));
	let selected = $state<Set<number>>(new Set());
	let history: RemovedHistory | null = null;
	// Reactive mirror of history.canUndo/Redo — the method results change on commit
	// WITHOUT the `history` reference changing, so the buttons need an explicit signal.
	let canUndo = $state(false);
	let canRedo = $state(false);
	let cullZ = $state(0);

	function syncHistory() {
		canUndo = !!history?.canUndo();
		canRedo = !!history?.canRedo();
	}

	// --- Tools + view -----------------------------------------------------------
	type Tool = 'single' | 'connected' | 'material' | 'box';
	let tool = $state<Tool>('connected');
	let viewMode = $state<'top' | 'orbit'>('orbit');
	let planeMode = $state<PlaneCullMode>('centroid');
	let matTol = $state(0.12);

	let showPlane = $state(true);
	let showRemoved = $state(false);
	let showFloors = $state(true);
	let showColors = $state(true);

	let sceneRef = $state<{ recenter: () => void } | undefined>(undefined);

	// --- Derived view data ------------------------------------------------------
	const bands = $derived(
		mesh
			? computeFloorBands(
					floorMarkersOf(mesh).map((m) => m.z),
					5
				)
			: null
	);
	const coloredFloorMarkers = $derived.by(() => {
		if (!mesh || !bands) return [];
		return floorMarkersOf(mesh).map((m) => ({
			...m,
			color: bandColor(bands.bandForZ(m.z), bands.count)
		}));
	});
	const previewTris = $derived(mesh && showPlane ? selectAbovePlane(meta, cullZ, planeMode) : null);
	const stats = $derived(mesh ? editStats(mesh, removed) : null);
	const boxMode = $derived(tool === 'box');
	const cullRange = $derived(
		mesh ? { min: mesh.bounds.minZ, max: mesh.bounds.maxZ } : { min: 0, max: 1 }
	);

	// --- Load a map -------------------------------------------------------------
	async function loadMap(key: string, opts: { baked?: boolean } = {}) {
		if (!key) return;
		loading = true;
		status = `Loading ${key}…`;
		const m = await loadBspMesh(GAME, key, fetch, { raw: !opts.baked });
		loading = false;
		if (!m) {
			status = `Could not load ${key} — is the geometry cache present?`;
			return;
		}
		setMesh(m, key, `${opts.baked ? 'baked spectator' : 'raw'} mesh loaded`);
	}

	function setMesh(m: BspMesh, key: string, note: string) {
		mesh = m;
		activeKey = key;
		meta = buildTriMeta(m);
		comp = connectedComponents(m);
		removed = new Uint8Array(meta.length);
		history = new RemovedHistory(removed);
		selected = new Set();
		cullZ = defaultCullZ(m);
		syncHistory();
		status = `${key}: ${note} — ${meta.length} triangles.`;
	}

	// --- Selection --------------------------------------------------------------
	function applySelection(tris: number[], mods: PickMods) {
		const next = mods.shift
			? new Set([...selected, ...tris])
			: mods.alt
				? new Set([...selected].filter((t) => !tris.includes(t)))
				: new Set(tris);
		selected = next;
	}

	function handlePick(tri: number | null, mods: PickMods) {
		if (tri == null) {
			if (!mods.shift && !mods.alt) selected = new Set();
			return;
		}
		let tris: number[];
		if (tool === 'connected') tris = selectComponent(comp, tri);
		else if (tool === 'material') tris = selectByMaterial(meta, tri, matTol);
		else tris = [tri]; // single + box-click
		applySelection(tris, mods);
	}

	function handleBoxSelect(tris: number[], mods: PickMods) {
		applySelection(tris, mods);
	}

	// --- Edits (all go through history) ----------------------------------------
	function commit(next: Uint8Array) {
		removed = next;
		history?.commit(next);
		syncHistory();
	}

	function deleteSelection() {
		if (!history || selected.size === 0) return;
		const next = Uint8Array.from(removed);
		for (const t of selected) next[t] = 1;
		commit(next);
		status = `Removed ${selected.size} triangles.`;
		selected = new Set();
	}

	function restoreSelection() {
		if (!history || selected.size === 0) return;
		const next = Uint8Array.from(removed);
		for (const t of selected) next[t] = 0;
		commit(next);
		status = `Restored ${selected.size} triangles.`;
		selected = new Set();
	}

	function applyPlaneCull() {
		if (!history || !mesh) return;
		const tris = selectAbovePlane(meta, cullZ, planeMode);
		const next = Uint8Array.from(removed);
		let n = 0;
		for (const t of tris)
			if (!next[t]) {
				next[t] = 1;
				n++;
			}
		commit(next);
		status = `Cull plane removed ${n} triangles above Z=${cullZ.toFixed(2)}.`;
	}

	function selectPlanePreview() {
		if (!mesh) return;
		selected = new Set(selectAbovePlane(meta, cullZ, planeMode).filter((t) => !removed[t]));
		status = `Selected ${selected.size} triangles above the plane.`;
	}

	function undo() {
		const s = history?.undo();
		if (s) {
			removed = s;
			selected = new Set();
			syncHistory();
			status = 'Undo.';
		}
	}
	function redo() {
		const s = history?.redo();
		if (s) {
			removed = s;
			selected = new Set();
			syncHistory();
			status = 'Redo.';
		}
	}
	function clearSelection() {
		selected = new Set();
	}

	// --- Export -----------------------------------------------------------------
	function bandsForExport(): SpectatorBand[] | undefined {
		if (!bands) return undefined;
		return bands.bands.map((b) => ({
			index: b.index,
			minZ: b.minZ,
			maxZ: b.maxZ,
			midZ: b.midZ
		}));
	}

	function buildSpectator() {
		if (!mesh) return null;
		return exportSpectatorMesh(mesh, removed, {
			cullZ,
			sourceMesh: manifest?.meshes[activeKey]?.file,
			bands: bandsForExport(),
			generatedAt: new Date().toISOString()
		});
	}

	function downloadBlob(blob: Blob, filename: string) {
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = filename;
		a.click();
		URL.revokeObjectURL(url);
	}

	function exportJson() {
		const spec = buildSpectator();
		if (!spec) return;
		downloadBlob(
			new Blob([JSON.stringify(spec)], { type: 'application/json' }),
			`${activeKey}.spectator.json`
		);
		status = `Downloaded ${activeKey}.spectator.json (${spec.triangle_count} tris).`;
	}

	function exportGlb() {
		const spec = buildSpectator();
		if (!spec) return;
		// Kept geometry in Three (Y-up) space so the GLB is usable in external tools.
		const T = spec.indices.length / 3;
		const pos = new Float32Array(spec.indices.length * 3);
		const col = spec.colors ? new Float32Array(spec.indices.length * 3) : null;
		for (let t = 0; t < T; t++) {
			for (let v = 0; v < 3; v++) {
				const vi = spec.indices[t * 3 + v];
				const p = haloToThree({
					x: spec.positions[vi * 3],
					y: spec.positions[vi * 3 + 1],
					z: spec.positions[vi * 3 + 2]
				});
				const o = (t * 3 + v) * 3;
				pos[o] = p[0];
				pos[o + 1] = p[1];
				pos[o + 2] = p[2];
				if (col && spec.colors) {
					col[o] = spec.colors[vi * 3];
					col[o + 1] = spec.colors[vi * 3 + 1];
					col[o + 2] = spec.colors[vi * 3 + 2];
				}
			}
		}
		const g = new THREE.BufferGeometry();
		g.setAttribute('position', new THREE.BufferAttribute(pos, 3));
		if (col) g.setAttribute('color', new THREE.BufferAttribute(col, 3));
		g.computeVertexNormals();
		const m = new THREE.Mesh(
			g,
			new THREE.MeshStandardMaterial({ vertexColors: !!col, side: THREE.DoubleSide })
		);
		new GLTFExporter().parse(
			m,
			(result) => {
				const blob = new Blob([result as ArrayBuffer], { type: 'model/gltf-binary' });
				downloadBlob(blob, `${activeKey}.spectator.glb`);
				status = `Downloaded ${activeKey}.spectator.glb.`;
			},
			(err) => (status = `GLB export failed: ${err}`),
			{ binary: true }
		);
	}

	let saving = $state(false);
	async function saveToCartographer() {
		const spec = buildSpectator();
		if (!spec) return;
		saving = true;
		status = 'Saving baked mesh into the geometry cache…';
		try {
			const res = await fetch('/__bsp-save', {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ game: GAME, key: activeKey, mesh: spec })
			});
			if (!res.ok) {
				const txt = await res.text();
				status = `Save failed (${res.status}): ${txt}. Use Download + drop the file in static/game-geometry/${GAME}/ instead.`;
				saving = false;
				return;
			}
			const out = await res.json();
			status = `Saved ${out.file} and updated the manifest. Reload a visualizer to see the baked mesh.`;
			manifest = await loadGeometryManifest(GAME); // refresh "baked" badges
		} catch (e) {
			status = `Save endpoint unavailable (dev only): ${e}. Use Download instead.`;
		}
		saving = false;
	}

	// --- Re-import an exported asset --------------------------------------------
	let fileInput: HTMLInputElement | undefined;
	async function onImportFile(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		try {
			const raw = JSON.parse(await file.text());
			const m = normalizeMesh(GAME, activeKey || 'imported', raw);
			if (!m) {
				status = 'Imported file has no usable geometry.';
				return;
			}
			setMesh(m, activeKey || 'imported', 'imported mesh');
		} catch (err) {
			status = `Import failed: ${err}`;
		}
		input.value = '';
	}

	// --- Keyboard ---------------------------------------------------------------
	function onKey(e: KeyboardEvent) {
		const tag = (e.target as HTMLElement)?.tagName;
		if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return;
		if ((e.key === 'Delete' || e.key === 'Backspace') && selected.size > 0) {
			e.preventDefault();
			deleteSelection();
		} else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') {
			e.preventDefault();
			if (e.shiftKey) redo();
			else undo();
		} else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'y') {
			e.preventDefault();
			redo();
		} else if (e.key === 'Escape') {
			clearSelection();
		}
	}

	onMount(async () => {
		manifest = await loadGeometryManifest(GAME);
		manifestTried = true;
		// Default to Chill Out when present, else the first map.
		const keys = manifest ? Object.keys(manifest.meshes) : [];
		const first = keys.includes('chillout') ? 'chillout' : keys[0];
		if (first) await loadMap(first);
		window.addEventListener('keydown', onKey);
	});
	onDestroy(() => window.removeEventListener('keydown', onKey));
</script>

<svelte:head>
	<title>BSP spectator-mesh editor</title>
	<style>
		html,
		body {
			background: #070a11 !important;
			background-image: none !important;
			overflow: hidden !important;
			margin: 0 !important;
		}
		body::before,
		body::after {
			display: none !important;
			content: none !important;
		}
	</style>
</svelte:head>

<div class="editor">
	<header class="bar">
		<div class="ident">
			<span class="logo">BSP&nbsp;EDITOR</span>
			<select
				class="mapsel"
				value={activeKey}
				onchange={(e) => loadMap((e.currentTarget as HTMLSelectElement).value)}
			>
				{#each mapList as m (m.key)}
					<option value={m.key}>{m.scenario} ({m.tris} tris){m.baked ? ' · baked' : ''}</option>
				{/each}
			</select>
			{#if manifest?.meshes[activeKey]?.spectator_file}
				<button class="ghost" onclick={() => loadMap(activeKey, { baked: true })}>
					Load baked
				</button>
			{/if}
		</div>
		<div class="meta">
			{#if stats}
				<span class="stat"><strong>{stats.keptTris}</strong> kept</span>
				<span class="stat removed"><strong>{stats.removedTris}</strong> removed</span>
				<span class="stat sel"><strong>{selected.size}</strong> selected</span>
			{/if}
			<span class="tag">spectator mesh</span>
		</div>
	</header>

	<div class="body">
		<aside class="panel">
			<section>
				<h3>View</h3>
				<div class="seg">
					<button class:active={viewMode === 'orbit'} onclick={() => (viewMode = 'orbit')}
						>Orbit</button
					>
					<button class:active={viewMode === 'top'} onclick={() => (viewMode = 'top')}
						>Top-down</button
					>
				</div>
				<button class="wide" onclick={() => sceneRef?.recenter()}>Recenter camera</button>
				<label class="chk"
					><input type="checkbox" bind:checked={showColors} /> Material colours</label
				>
				<label class="chk"><input type="checkbox" bind:checked={showFloors} /> Floor markers</label>
				<label class="chk"><input type="checkbox" bind:checked={showRemoved} /> Ghost removed</label
				>
			</section>

			<section>
				<h3>Cull-height plane</h3>
				<label class="chk"
					><input type="checkbox" bind:checked={showPlane} /> Show plane + preview</label
				>
				<div class="slider">
					<input
						type="range"
						min={cullRange.min}
						max={cullRange.max}
						step="0.05"
						bind:value={cullZ}
						disabled={!mesh}
					/>
					<span class="zval">Z {cullZ.toFixed(2)}</span>
				</div>
				<div class="seg small">
					<button class:active={planeMode === 'centroid'} onclick={() => (planeMode = 'centroid')}
						>Centroid</button
					>
					<button class:active={planeMode === 'whole'} onclick={() => (planeMode = 'whole')}
						>Whole</button
					>
					<button class:active={planeMode === 'any'} onclick={() => (planeMode = 'any')}>Any</button
					>
				</div>
				<button class="wide" onclick={selectPlanePreview} disabled={!mesh}
					>Select above plane</button
				>
				<button class="wide danger" onclick={applyPlaneCull} disabled={!mesh}
					>Remove above plane</button
				>
				<p class="note">
					Default = highest walkable floor + headroom. Drag the slider (or the plane in Orbit view).
				</p>
			</section>

			<section>
				<h3>Select tool</h3>
				<div class="seg col">
					<button class:active={tool === 'connected'} onclick={() => (tool = 'connected')}
						>Connected piece</button
					>
					<button class:active={tool === 'material'} onclick={() => (tool = 'material')}
						>Same material</button
					>
					<button class:active={tool === 'single'} onclick={() => (tool = 'single')}
						>Single triangle</button
					>
					<button class:active={tool === 'box'} onclick={() => (tool = 'box')}>Box select</button>
				</div>
				{#if tool === 'material'}
					<div class="slider">
						<input type="range" min="0.02" max="0.4" step="0.01" bind:value={matTol} />
						<span class="zval">tol {matTol.toFixed(2)}</span>
					</div>
				{/if}
				<p class="note">
					Click to select · Shift+click adds · Alt+click removes. {tool === 'box'
						? 'Drag a box to select.'
						: 'Drag empties to orbit.'}
				</p>
			</section>

			<section>
				<h3>Edit</h3>
				<div class="row">
					<button class="danger" onclick={deleteSelection} disabled={selected.size === 0}
						>Delete / mark inaccessible</button
					>
				</div>
				<div class="row">
					<button onclick={restoreSelection} disabled={selected.size === 0}>Restore</button>
					<button onclick={clearSelection} disabled={selected.size === 0}>Clear sel.</button>
				</div>
				<div class="row">
					<button onclick={undo} disabled={!canUndo}>↶ Undo</button>
					<button onclick={redo} disabled={!canRedo}>↷ Redo</button>
				</div>
			</section>

			<section>
				<h3>Export baked asset</h3>
				<button class="wide primary" onclick={saveToCartographer} disabled={!mesh || saving}>
					{saving ? 'Saving…' : 'Save to cartographer (dev)'}
				</button>
				<div class="row">
					<button onclick={exportJson} disabled={!mesh}>Download JSON</button>
					<button onclick={exportGlb} disabled={!mesh}>Download GLB</button>
				</div>
				<button class="wide" onclick={() => fileInput?.click()}>Import .json (re-edit)</button>
				<input
					type="file"
					accept="application/json,.json"
					bind:this={fileInput}
					onchange={onImportFile}
					hidden
				/>
				<p class="note">
					JSON is what both visualizer views load (same schema as the raw mesh). GLB is for external
					3D tools.
				</p>
			</section>

			{#if bands}
				<section>
					<h3>Elevation bands</h3>
					<ul class="bandlist">
						{#each [...bands.bands].reverse() as b (b.index)}
							<li>
								<span class="bsw" style="--c: {bandColor(b.index, bands.count)}"></span>
								<span>{b.minZ.toFixed(1)}–{b.maxZ.toFixed(1)} wu</span>
							</li>
						{/each}
					</ul>
				</section>
			{/if}
		</aside>

		<div class="stage">
			{#if mesh}
				<EditorScene
					bind:this={sceneRef}
					{mesh}
					{meta}
					{removed}
					{selected}
					{previewTris}
					floorMarkers={coloredFloorMarkers}
					{viewMode}
					{boxMode}
					{cullZ}
					{showPlane}
					{showRemoved}
					{showFloors}
					{showColors}
					onPick={handlePick}
					onBoxSelect={handleBoxSelect}
					onCullZChange={(z) => (cullZ = z)}
				/>
			{:else}
				<div class="empty">
					{#if !manifestTried}
						<p>Loading geometry cache…</p>
					{:else if mapList.length === 0}
						<p>
							No extracted geometry found at <code>static/game-geometry/{GAME}/</code>.<br />
							Run <code>scripts/game-geometry/extract_bsp.py</code> against your Halo maps first.
						</p>
					{:else if loading}
						<p>Loading map…</p>
					{:else}
						<p>Select a map to begin.</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<footer class="status">{status}</footer>
</div>

<style>
	.editor {
		position: fixed;
		inset: 0;
		z-index: 40;
		display: flex;
		flex-direction: column;
		background: #070a11;
		color: #e9edf2;
		font-family: 'Inter', system-ui, sans-serif;
		font-size: 0.85rem;
	}
	.bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.5rem 0.85rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);
		flex-wrap: wrap;
	}
	.ident {
		display: flex;
		align-items: center;
		gap: 0.6rem;
	}
	.logo {
		font-weight: 800;
		letter-spacing: 0.06em;
		color: #8fd0ff;
	}
	.mapsel {
		font: inherit;
		background: rgba(8, 12, 20, 0.9);
		color: #e9edf2;
		border: 1px solid rgba(255, 255, 255, 0.16);
		border-radius: 0.35rem;
		padding: 0.2rem 0.4rem;
		max-width: 22rem;
	}
	.meta {
		display: flex;
		align-items: center;
		gap: 0.7rem;
	}
	.stat strong {
		font-variant-numeric: tabular-nums;
	}
	.stat.removed strong {
		color: #ff8d8b;
	}
	.stat.sel strong {
		color: #6fe0ff;
	}
	.tag {
		font-size: 0.62rem;
		font-weight: 800;
		letter-spacing: 0.12em;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
		background: rgba(92, 200, 255, 0.18);
		color: #8fd0ff;
		text-transform: uppercase;
	}
	.body {
		flex: 1;
		min-height: 0;
		display: flex;
	}
	.panel {
		width: 16.5rem;
		flex-shrink: 0;
		border-right: 1px solid rgba(255, 255, 255, 0.08);
		padding: 0.6rem 0.75rem;
		overflow-y: auto;
	}
	.panel section {
		margin-bottom: 0.85rem;
		padding-bottom: 0.7rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
	}
	.panel h3 {
		font-size: 0.66rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		opacity: 0.6;
		margin: 0 0 0.45rem;
		font-weight: 700;
	}
	.seg {
		display: flex;
		border: 1px solid rgba(255, 255, 255, 0.14);
		border-radius: 0.4rem;
		overflow: hidden;
		margin-bottom: 0.5rem;
	}
	.seg.col {
		flex-direction: column;
	}
	.seg.small button {
		font-size: 0.74rem;
		padding: 0.15rem 0.3rem;
	}
	.seg button {
		flex: 1;
		font: inherit;
		font-size: 0.8rem;
		cursor: pointer;
		background: transparent;
		color: #9aa4b2;
		border: none;
		padding: 0.28rem 0.4rem;
	}
	.seg.col button {
		text-align: left;
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
	}
	.seg button.active {
		background: rgba(92, 200, 255, 0.22);
		color: #cdecff;
	}
	button {
		font: inherit;
	}
	.wide {
		display: block;
		width: 100%;
		font-size: 0.8rem;
		cursor: pointer;
		background: rgba(255, 255, 255, 0.06);
		color: #cbd5e1;
		border: 1px solid rgba(255, 255, 255, 0.12);
		border-radius: 0.35rem;
		padding: 0.3rem 0.4rem;
		margin-bottom: 0.4rem;
	}
	.wide:hover:not(:disabled),
	.row button:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.12);
	}
	.row {
		display: flex;
		gap: 0.4rem;
		margin-bottom: 0.4rem;
	}
	.row button {
		flex: 1;
		cursor: pointer;
		background: rgba(255, 255, 255, 0.06);
		color: #cbd5e1;
		border: 1px solid rgba(255, 255, 255, 0.12);
		border-radius: 0.35rem;
		padding: 0.3rem 0.4rem;
		font-size: 0.8rem;
	}
	.primary {
		background: rgba(54, 181, 90, 0.2);
		border-color: rgba(54, 181, 90, 0.4);
		color: #9ef0b8;
	}
	.danger {
		background: rgba(227, 65, 63, 0.16);
		border-color: rgba(227, 65, 63, 0.38);
		color: #ffb0ae;
	}
	.ghost {
		font-size: 0.74rem;
		cursor: pointer;
		background: rgba(255, 255, 255, 0.06);
		color: #cbd5e1;
		border: 1px solid rgba(255, 255, 255, 0.12);
		border-radius: 0.3rem;
		padding: 0.15rem 0.4rem;
	}
	button:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.chk {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		cursor: pointer;
		margin-bottom: 0.25rem;
		font-size: 0.82rem;
	}
	.slider {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		margin-bottom: 0.5rem;
	}
	.slider input[type='range'] {
		flex: 1;
		accent-color: #5cc8ff;
	}
	.zval {
		font-variant-numeric: tabular-nums;
		font-size: 0.74rem;
		opacity: 0.8;
		min-width: 3.4rem;
	}
	.note {
		margin: 0.3rem 0 0;
		opacity: 0.5;
		font-size: 0.72rem;
		line-height: 1.35;
	}
	.bandlist {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}
	.bandlist li {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.78rem;
		font-variant-numeric: tabular-nums;
	}
	.bsw {
		width: 13px;
		height: 13px;
		border-radius: 3px;
		background: var(--c);
		flex-shrink: 0;
	}
	.stage {
		flex: 1;
		min-width: 0;
		position: relative;
	}
	.empty {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		text-align: center;
		opacity: 0.7;
		padding: 2rem;
		line-height: 1.6;
	}
	.empty code {
		font-family: 'JetBrains Mono', monospace;
		background: rgba(255, 255, 255, 0.08);
		padding: 0.05em 0.35em;
		border-radius: 0.25rem;
	}
	.status {
		padding: 0.3rem 0.85rem;
		border-top: 1px solid rgba(255, 255, 255, 0.08);
		font-size: 0.78rem;
		color: #9fb0c4;
		min-height: 1.4rem;
		font-variant-numeric: tabular-nums;
	}
</style>
