<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildVizModel } from '$lib/utils/visualizer-view';
	import { teamMeta } from '$lib/utils/overlay-view';
	import { loadIconSet, emptyIconSet, type IconSet } from '$lib/utils/game-icons';
	import { loadBspMesh, meshKeyForScenario, type BspMesh } from '$lib/utils/game-geometry';
	import { buildFloorplan } from '$lib/utils/floorplan';
	import { computeFloorBands, bandColor } from '$lib/utils/floor-bands';
	import { mockStagedModel } from '$lib/utils/mock-map';
	import TopDownMap from '$lib/components/visualizer/TopDownMap.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	// CE today; when an H2 plugin lands its game id would come off the feed.
	const GAME = 'haloce';

	// Per-map demo mode (?map=<key>): no live feed/token — render that map's
	// cached geometry populated with mock players STAGED on different floors.
	const demoMap = untrack(() => data.map);
	const isDemo = demoMap.length > 0;

	const feed = createOverlayFeed();

	// Real Halo HUD icons for the map markers, decoded from the user's game files.
	let iconSet = $state<IconSet>(emptyIconSet(GAME));

	// Full structure-BSP mesh (positions/indices) → floorplan + elevation bands.
	let mesh = $state<BspMesh | null>(null);
	let loadedKey = '';

	// Live/mock: model off the WS feed. Demo: stage a 4v4 across the loaded map's
	// real floors (waits for the mesh so the staging sits on real geometry).
	const model = $derived(
		isDemo
			? mesh
				? mockStagedModel(mesh, demoMap)
				: buildVizModel(null, null, null, null)
			: buildVizModel(feed.game, feed.tick, feed.scenario, feed.objects)
	);

	// Floorplan + elevation bands derived from the mesh (pure; recomputed only when
	// the mesh changes).
	const floorplan = $derived(mesh ? buildFloorplan(mesh) : null);
	const bands = $derived(floorplan ? computeFloorBands(floorplan.floorZs, 5) : null);

	// Layer toggles (debug controls).
	let showFloorplan = $state(true);
	let showSpawns = $state(untrack(() => data.showSpawns));
	let showItems = $state(true);
	let showVehicles = $state(true);
	let showProjectiles = $state(true);
	let showNames = $state(true);

	// Readability controls.
	let shadeMode = $state<'absolute' | 'relative'>('absolute');
	let followIndex = $state<number | null>(null);
	let hiddenBands = $state<number[]>([]);

	const activeBands = $derived(
		bands ? bands.bands.map((b) => b.index).filter((i) => !hiddenBands.includes(i)) : null
	);
	const followZ = $derived(
		shadeMode === 'relative' && followIndex != null
			? (model.players.find((pl) => pl.index === followIndex)?.pos.z ?? null)
			: null
	);

	function toggleBand(i: number) {
		hiddenBands = hiddenBands.includes(i)
			? hiddenBands.filter((x) => x !== i)
			: [...hiddenBands, i];
	}
	function showAllBands() {
		hiddenBands = [];
	}
	function setShadeMode(mode: 'absolute' | 'relative') {
		shadeMode = mode;
		if (mode === 'relative' && followIndex == null) {
			const local = model.players.find((pl) => pl.isLocal) ?? model.players[0];
			followIndex = local ? local.index : null;
		}
	}

	onMount(() => {
		if (!isDemo)
			feed.start({
				instance: data.instance,
				token: data.token,
				mock: data.mock,
				classes: ['game', 'tick', 'scenario', 'objects']
			});
		loadIconSet(GAME).then((s) => (iconSet = s));
	});
	onDestroy(() => feed.stop());

	// Load the level mesh when the map first becomes known (and again if it
	// changes). Source is the demo map or the live feed's scenario.
	$effect(() => {
		const raw = isDemo ? demoMap : (feed.scenario?.map ?? '');
		const key = meshKeyForScenario(raw);
		if (!key || key === loadedKey) return;
		loadedKey = key;
		loadBspMesh(GAME, raw).then((m) => (mesh = m));
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
</script>

<svelte:head>
	<title>Visualizer · {data.instance}</title>
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
			<span class="tag2d">2D FLOORPLAN</span>
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
			No overlay token. Mint one at <code>/overlays/manage/</code> (scope
			<code>host:{data.instance}</code>) and append <code>?token=…</code>, or append
			<code>?mock=1</code> to preview.
		</div>
	{/if}

	<div class="body">
		<div class="mapwrap">
			<div class="mapbox">
				<TopDownMap
					{model}
					icons={iconSet}
					{floorplan}
					{bands}
					{showFloorplan}
					{shadeMode}
					{followZ}
					{activeBands}
					{showSpawns}
					{showItems}
					{showVehicles}
					{showProjectiles}
					{showNames}
				/>
			</div>
		</div>

		<aside class="legend">
			<div class="toggles">
				<label><input type="checkbox" bind:checked={showFloorplan} /> Floorplan</label>
				<label><input type="checkbox" bind:checked={showNames} /> Names</label>
				<label><input type="checkbox" bind:checked={showItems} /> Items</label>
				<label><input type="checkbox" bind:checked={showVehicles} /> Vehicles</label>
				<label><input type="checkbox" bind:checked={showProjectiles} /> Projectiles</label>
				<label><input type="checkbox" bind:checked={showSpawns} /> Spawns</label>
			</div>

			{#if bands}
				<section class="readability">
					<h3>Elevation shading</h3>
					<div class="seg">
						<button
							class:active={shadeMode === 'absolute'}
							onclick={() => setShadeMode('absolute')}
						>
							Absolute
						</button>
						<button
							class:active={shadeMode === 'relative'}
							onclick={() => setShadeMode('relative')}
						>
							Relative
						</button>
					</div>
					{#if shadeMode === 'relative'}
						<label class="follow">
							Follow
							<select bind:value={followIndex}>
								{#each model.players as pl (pl.index)}
									<option value={pl.index}>{pl.name}</option>
								{/each}
							</select>
						</label>
					{/if}

					<h3 class="filterhead">
						Floor filter
						<button class="small" onclick={showAllBands}>Show all</button>
					</h3>
					<ul class="bandlist">
						{#each [...bands.bands].reverse() as b (b.index)}
							<li>
								<label>
									<input
										type="checkbox"
										checked={!hiddenBands.includes(b.index)}
										onchange={() => toggleBand(b.index)}
									/>
									<span class="bswatch" style="--c: {bandColor(b.index, bands.count)}"></span>
									<span class="brange">{b.minZ.toFixed(1)}–{b.maxZ.toFixed(1)} wu</span>
								</label>
							</li>
						{/each}
					</ul>
				</section>
			{/if}

			<section>
				<h3>Players <span class="muted">{model.placedCount}</span></h3>
				{#if model.players.length === 0}
					<p class="note">No players placed yet.</p>
				{:else}
					<ul class="keys">
						{#each teamLegend as t (t.team)}
							<li>
								<span class="swatch dot" style="--c: {t.color}"></span>
								{model.isTeamGame ? t.name : 'Players'}
								<span class="muted">{t.count}</span>
							</li>
						{/each}
						<li class="sub"><span class="swatch ring"></span> ring = team · fill = floor height</li>
						<li class="sub"><span class="swatch local"></span> local player</li>
					</ul>
				{/if}
			</section>

			<section class="caps">
				<h3>Feed</h3>
				<ul class="capgrid">
					<li class:on={model.hasGame}>game</li>
					<li class:on={model.hasTick}>tick</li>
					<li class:on={model.hasScenario}>scenario</li>
					<li class:on={!!mesh} title="Real structure-BSP mesh decoded from game files">mesh</li>
					<li class:on={iconSet.loaded} title="Real Halo HUD icons (decoded from game files)">
						icons
					</li>
				</ul>
				<p class="note">
					Bounds: {model.bounds.source}{model.bounds.valid ? '' : ' (none)'}
				</p>
			</section>
		</aside>
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
		letter-spacing: 0.01em;
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
	.tag2d {
		font-size: 0.62rem;
		font-weight: 800;
		letter-spacing: 0.12em;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
		background: rgba(155, 200, 120, 0.18);
		color: #b6d98a;
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
	.hint code,
	.inst {
		font-family: 'JetBrains Mono', monospace;
	}
	.hint code {
		background: rgba(255, 255, 255, 0.1);
		padding: 0.05em 0.35em;
		border-radius: 0.25rem;
		font-size: 0.85em;
	}

	.body {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.mapwrap {
		flex: 1;
		display: grid;
		place-items: center;
		padding: 1rem;
		min-width: 0;
	}
	.mapbox {
		width: min(100%, calc(100vh - 6rem));
		aspect-ratio: 1;
		max-height: 100%;
	}

	.legend {
		width: 17rem;
		flex-shrink: 0;
		border-left: 1px solid rgba(255, 255, 255, 0.08);
		padding: 0.85rem 1rem;
		overflow-y: auto;
		font-size: 0.85rem;
	}
	.toggles {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 0.9rem;
		padding-bottom: 0.75rem;
		margin-bottom: 0.5rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);
	}
	.toggles label {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		cursor: pointer;
		user-select: none;
	}
	.legend section {
		margin-bottom: 0.9rem;
	}
	.legend h3 {
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		opacity: 0.6;
		margin: 0 0 0.35rem;
		font-weight: 700;
	}
	.readability .seg {
		display: inline-flex;
		border: 1px solid rgba(255, 255, 255, 0.14);
		border-radius: 0.4rem;
		overflow: hidden;
		margin-bottom: 0.5rem;
	}
	.readability .seg button {
		font: inherit;
		font-size: 0.78rem;
		cursor: pointer;
		background: transparent;
		color: #9aa4b2;
		border: none;
		padding: 0.2rem 0.7rem;
	}
	.readability .seg button.active {
		background: rgba(92, 200, 255, 0.22);
		color: #cdecff;
	}
	.follow {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		margin-bottom: 0.6rem;
	}
	.follow select {
		font: inherit;
		font-size: 0.8rem;
		background: rgba(8, 12, 20, 0.9);
		color: #e9edf2;
		border: 1px solid rgba(255, 255, 255, 0.14);
		border-radius: 0.3rem;
		padding: 0.1rem 0.3rem;
	}
	.filterhead {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.small {
		font: inherit;
		font-size: 0.68rem;
		text-transform: none;
		letter-spacing: 0;
		cursor: pointer;
		background: rgba(255, 255, 255, 0.08);
		color: #cbd5e1;
		border: none;
		border-radius: 0.25rem;
		padding: 0.05rem 0.4rem;
		opacity: 1;
	}
	.bandlist {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.bandlist label {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		cursor: pointer;
	}
	.bswatch {
		display: inline-block;
		width: 14px;
		height: 14px;
		border-radius: 3px;
		background: var(--c);
		flex-shrink: 0;
	}
	.brange {
		font-variant-numeric: tabular-nums;
		font-size: 0.8rem;
	}
	.muted {
		opacity: 0.5;
		font-variant-numeric: tabular-nums;
		margin-left: 0.2rem;
	}
	.note {
		margin: 0;
		opacity: 0.55;
		font-size: 0.8rem;
		line-height: 1.35;
	}
	.keys {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.28rem;
	}
	.keys li {
		display: flex;
		align-items: center;
		gap: 0.45rem;
	}
	.keys li.sub {
		opacity: 0.7;
		font-size: 0.8rem;
		padding-left: 0.1rem;
	}
	.swatch {
		display: inline-block;
		width: 12px;
		height: 12px;
		flex-shrink: 0;
	}
	.swatch.dot {
		border-radius: 50%;
		background: var(--c);
	}
	.swatch.ring {
		border-radius: 50%;
		border: 2px solid #5cc8ff;
		border-right-color: #36b55a;
		border-bottom-color: #36b55a;
	}
	.swatch.local {
		border-radius: 50%;
		border: 1.5px solid #fff;
		background: rgba(255, 255, 255, 0.15);
	}
	.capgrid {
		list-style: none;
		margin: 0 0 0.4rem;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.3rem;
	}
	.capgrid li {
		font-size: 0.72rem;
		font-family: 'JetBrains Mono', monospace;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
		background: rgba(255, 255, 255, 0.06);
		color: #6b7585;
	}
	.capgrid li.on {
		background: rgba(54, 181, 90, 0.18);
		color: #7ee2a0;
	}

	@media (max-width: 720px) {
		.body {
			flex-direction: column;
		}
		.legend {
			width: auto;
			border-left: none;
			border-top: 1px solid rgba(255, 255, 255, 0.08);
		}
		.mapbox {
			width: min(100%, 70vh);
		}
	}
</style>
