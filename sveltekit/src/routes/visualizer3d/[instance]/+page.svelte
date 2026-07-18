<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildVizModel } from '$lib/utils/visualizer-view';
	import { teamMeta } from '$lib/utils/overlay-view';
	import { loadBspMesh, meshKeyForScenario, type BspMesh } from '$lib/utils/game-geometry';
	import { mockStagedModel } from '$lib/utils/mock-map';
	import { buildRooms, classifyShellTriangles } from '$lib/utils/rooms';
	import Scene3D from '$lib/components/visualizer/Scene3D.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	// CE today; when an H2 plugin lands its game id would come off the feed.
	const GAME = 'haloce';

	// Per-map demo mode (?map=<key>): no live feed/token — render that map's
	// cached geometry populated with mock players placed within its real bounds.
	const demoMap = untrack(() => data.map);
	const isDemo = demoMap.length > 0;

	const feed = createOverlayFeed();

	// Structure-BSP mesh, decoded from the user's game files into the served
	// geometry cache. Loads best-effort once the map is known (feed scenario, or
	// the demo map): no cache → null → scene degrades to the world-bounds box.
	let mesh = $state<BspMesh | null>(null);
	let loadedKey = '';
	let geometryTried = $state(false);

	// Live/mock: model off the same WS feed. Demo: stage a 4v4 on the loaded map's
	// real floors (waits for the mesh so every body sits on geometry).
	const model = $derived(
		isDemo
			? mesh
				? mockStagedModel(mesh, mesh.scenario)
				: buildVizModel(null, null, null, null)
			: buildVizModel(feed.game, feed.tick, feed.scenario, feed.objects)
	);

	// Exterior shell (ceilings + perimeter walls) → culled so the interior is
	// visible. The remaining INTERIOR triangles are clustered into rooms (BSP-
	// cluster approximation) for the occupancy/occlusion fades. Cluster count
	// scales with mesh complexity so small indoor maps don't over-fragment.
	const shell = $derived(mesh ? classifyShellTriangles(mesh) : null);
	const shellTris = $derived(
		mesh && shell ? shell.map((s, i) => (s ? i : -1)).filter((i) => i >= 0) : null
	);
	const rooms = $derived.by(() => {
		if (!mesh) return null;
		const interior = shell ? shell.map((s, i) => (s ? -1 : i)).filter((i) => i >= 0) : undefined;
		const triCount = interior ? interior.length : mesh.indices.length / 3;
		const k = Math.max(6, Math.min(10, Math.round(triCount / 400)));
		return buildRooms(mesh, k, interior);
	});

	let showSpawns = $state(untrack(() => data.showSpawns));
	let showItems = $state(true);
	let showVehicles = $state(true);
	let showProjectiles = $state(true);
	let showNames = $state(true);

	// Readability toggles. Occupancy reveal + through-wall on by default; occlusion
	// fade is an opt-in (the shell cull already opens the interior, so leaving it
	// off keeps the interior fully solid + colored — only occupied rooms dim).
	let occupancyReveal = $state(true);
	let occlusionFade = $state(false);
	let throughWall = $state(true);
	let showOuterShell = $state(false);

	// Only the exported methods we call are typed here; bind:this fills it with the
	// full Scene3D instance (structurally assignable).
	let sceneRef = $state<{ recenter: () => void; overview: () => void } | undefined>(undefined);

	onMount(() => {
		// Demo mode drives everything from the cached geometry — no feed/token.
		if (!isDemo)
			feed.start({
				instance: data.instance,
				token: data.token,
				mock: data.mock,
				classes: ['game', 'tick', 'scenario', 'objects']
			});
	});
	onDestroy(() => feed.stop());

	// Load the level mesh when the map first becomes known (and again if it
	// changes). Source is the demo map or the live feed's scenario. Keyed on the
	// slugified name so a re-emit doesn't reload.
	$effect(() => {
		const raw = isDemo ? demoMap : (feed.scenario?.map ?? '');
		const key = meshKeyForScenario(raw);
		if (!key || key === loadedKey) return;
		loadedKey = key;
		geometryTried = false;
		loadBspMesh(GAME, raw).then((m) => {
			mesh = m;
			geometryTried = true;
		});
	});

	const teamLegend = $derived.by(() => {
		const acc: Record<number, { team: number; name: string; color: string; count: number }> = {};
		for (const pl of model.players) {
			const e = acc[pl.team];
			if (e) e.count += 1;
			else
				acc[pl.team] = { team: pl.team, name: teamMeta(pl.team).name, color: pl.color, count: 1 };
		}
		return Object.values(acc).sort((a, b) => a.team - b.team);
	});

	const phaseTitle = $derived(model.phase.charAt(0).toUpperCase() + model.phase.slice(1));
	const geometryLabel = $derived(
		mesh ? 'BSP mesh' : geometryTried ? 'bounds box' : model.bounds.valid ? 'bounds box' : '—'
	);
</script>

<svelte:head>
	<title>3D Visualizer · {data.instance}</title>
	<!-- Standalone debug/spectator surface: own solid background, no theme body
	     decorations bleeding through, no page scroll. -->
	<style>
		html,
		body {
			background: #060911 !important;
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

<div class="viz">
	<header class="bar">
		<div class="ident">
			<span class="map">{model.mapName || '—'}</span>
			{#if model.gametype}<span class="gametype">{model.gametype}</span>{/if}
			<span class="phase phase-{model.phase}">{phaseTitle}</span>
			<span class="tag3d">3D</span>
		</div>
		<div class="meta">
			<span class="stat" title="Players placed on the map / named in the roster">
				<strong>{model.placedCount}</strong>/{model.playerCount} players
			</span>
			<span class="conn" class:ok={feed.connected} class:mock={feed.mock || isDemo}>
				{#if feed.mock || isDemo}MOCK{:else if feed.connected}● LIVE{:else}○ Connecting…{/if}
			</span>
			<span class="inst">{data.instance}</span>
		</div>
	</header>

	{#if feed.missingToken}
		<div class="hint">
			No overlay token. Append <code>?token=…</code> with an overlay token scoped to
			<code>host:{data.instance}</code> (ask an admin to mint one via
			<code>/api/overlay-tokens</code>), or append <code>?mock=1</code> to preview.
		</div>
	{/if}

	<div class="body">
		<div class="scenewrap">
			<Scene3D
				bind:this={sceneRef}
				{model}
				{mesh}
				{rooms}
				{shellTris}
				{occupancyReveal}
				{occlusionFade}
				{throughWall}
				{showOuterShell}
				{showItems}
				{showVehicles}
				{showProjectiles}
				{showSpawns}
				{showNames}
			/>
			<div class="hud">
				<div class="toggles readability">
					<label title="Interior rooms fade when a player is inside them">
						<input type="checkbox" bind:checked={occupancyReveal} /> Occupancy reveal
					</label>
					<label title="Interior geometry between the camera and a player fades">
						<input type="checkbox" bind:checked={occlusionFade} /> Occlusion fade
					</label>
					<label title="Markers draw on top; dimmed when behind a wall">
						<input type="checkbox" bind:checked={throughWall} /> Through-wall markers
					</label>
					<label title="Bring the culled exterior shell back (faint, for context)">
						<input type="checkbox" bind:checked={showOuterShell} /> Show outer shell
					</label>
					<button class="recenter" onclick={() => sceneRef?.overview()}>Overview cam</button>
				</div>
				<div class="toggles">
					<label><input type="checkbox" bind:checked={showNames} /> Names</label>
					<label><input type="checkbox" bind:checked={showItems} /> Items</label>
					<label><input type="checkbox" bind:checked={showVehicles} /> Vehicles</label>
					<label><input type="checkbox" bind:checked={showProjectiles} /> Projectiles</label>
					<label><input type="checkbox" bind:checked={showSpawns} /> Spawns</label>
					<button class="recenter" onclick={() => sceneRef?.recenter()}>Recenter</button>
				</div>
				<div class="caps">
					<span class="cap" class:on={model.hasGame}>game</span>
					<span class="cap" class:on={model.hasTick}>tick</span>
					<span class="cap" class:on={model.hasScenario}>scenario</span>
					<span class="cap" class:on={!!rooms} title="Room clusters from the BSP mesh">
						rooms: {rooms?.length ?? 0}
					</span>
					<span
						class="cap"
						class:on={!!mesh}
						title="Real structure-BSP mesh decoded from game files"
					>
						geometry: {geometryLabel}
					</span>
				</div>
				{#if model.players.length > 0}
					<ul class="teams">
						{#each teamLegend as t (t.team)}
							<li>
								<span class="swatch" style="--c: {t.color}"></span>
								{model.isTeamGame ? t.name : 'Players'}
								<span class="muted">{t.count}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	</div>
</div>

<style>
	.viz {
		position: fixed;
		inset: 0;
		z-index: 40;
		display: flex;
		flex-direction: column;
		background: #060911;
		color: #e9edf2;
		font-family: 'Inter', system-ui, sans-serif;
	}
	.bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.6rem 1rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);
		flex-wrap: wrap;
	}
	.ident {
		display: flex;
		align-items: baseline;
		gap: 0.75rem;
		min-width: 0;
	}
	.map {
		font-size: 1.15rem;
		font-weight: 700;
	}
	.gametype {
		font-size: 0.85rem;
		opacity: 0.8;
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}
	.phase {
		font-size: 0.7rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		padding: 0.1rem 0.45rem;
		border-radius: 0.25rem;
		background: rgba(255, 255, 255, 0.1);
	}
	.phase-live {
		background: rgba(54, 181, 90, 0.9);
		color: #03210f;
	}
	.phase-ready {
		background: rgba(224, 163, 46, 0.9);
		color: #1a1300;
	}
	.tag3d {
		font-size: 0.62rem;
		font-weight: 800;
		letter-spacing: 0.12em;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
		background: rgba(92, 200, 255, 0.18);
		color: #5cc8ff;
	}
	.meta {
		display: flex;
		align-items: center;
		gap: 1rem;
		font-size: 0.85rem;
	}
	.stat strong {
		font-variant-numeric: tabular-nums;
		font-size: 1rem;
	}
	.conn {
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		padding: 0.12rem 0.45rem;
		border-radius: 0.25rem;
		background: rgba(255, 255, 255, 0.08);
		color: #9aa4b2;
	}
	.conn.ok {
		color: #7ee2a0;
	}
	.conn.mock {
		background: rgba(224, 163, 46, 0.9);
		color: #1a1300;
	}
	.inst {
		font-family: 'JetBrains Mono', monospace;
		font-size: 0.78rem;
		opacity: 0.6;
	}
	.hint {
		padding: 0.6rem 1rem;
		background: rgba(224, 163, 46, 0.12);
		border-bottom: 1px solid rgba(224, 163, 46, 0.3);
		font-size: 0.88rem;
	}
	.hint code {
		font-family: 'JetBrains Mono', monospace;
		background: rgba(255, 255, 255, 0.1);
		padding: 0.05em 0.35em;
		border-radius: 0.25rem;
		font-size: 0.85em;
	}
	.body {
		flex: 1;
		min-height: 0;
		display: flex;
	}
	.scenewrap {
		position: relative;
		flex: 1;
		min-width: 0;
		min-height: 0;
	}
	.hud {
		position: absolute;
		top: 0.75rem;
		left: 0.75rem;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		pointer-events: none;
	}
	.toggles {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 0.8rem;
		align-items: center;
		background: rgba(8, 12, 20, 0.72);
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: 0.5rem;
		padding: 0.5rem 0.7rem;
		pointer-events: auto;
		font-size: 0.82rem;
		max-width: 30rem;
	}
	.toggles.readability {
		border-color: rgba(92, 200, 255, 0.35);
		background: rgba(10, 20, 32, 0.82);
	}
	.toggles label {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		cursor: pointer;
		user-select: none;
	}
	.recenter {
		font: inherit;
		font-size: 0.78rem;
		cursor: pointer;
		background: rgba(92, 200, 255, 0.16);
		color: #cdecff;
		border: 1px solid rgba(92, 200, 255, 0.3);
		border-radius: 0.3rem;
		padding: 0.15rem 0.55rem;
	}
	.recenter:hover {
		background: rgba(92, 200, 255, 0.28);
	}
	.caps {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3rem;
		pointer-events: none;
	}
	.cap {
		font-size: 0.68rem;
		font-family: 'JetBrains Mono', monospace;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
		background: rgba(8, 12, 20, 0.72);
		color: #6b7585;
		border: 1px solid rgba(255, 255, 255, 0.06);
	}
	.cap.on {
		color: #7ee2a0;
		border-color: rgba(54, 181, 90, 0.3);
	}
	.teams {
		list-style: none;
		margin: 0;
		padding: 0.4rem 0.6rem;
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		background: rgba(8, 12, 20, 0.72);
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: 0.5rem;
		font-size: 0.8rem;
		width: max-content;
		pointer-events: none;
	}
	.teams li {
		display: flex;
		align-items: center;
		gap: 0.45rem;
	}
	.swatch {
		display: inline-block;
		width: 11px;
		height: 11px;
		border-radius: 50%;
		background: var(--c);
		flex-shrink: 0;
	}
	.muted {
		opacity: 0.5;
		font-variant-numeric: tabular-nums;
	}
</style>
