<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildVizModel } from '$lib/utils/visualizer-view';
	import { teamMeta } from '$lib/utils/overlay-view';
	import { loadIconSet, emptyIconSet, type IconSet } from '$lib/utils/game-icons';
	import {
		loadTopProjection,
		meshKeyForScenario,
		type TopProjection
	} from '$lib/utils/game-geometry';
	import TopDownMap from '$lib/components/visualizer/TopDownMap.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	// CE today; when an H2 plugin lands its game id would come off the feed.
	const GAME = 'haloce';

	const feed = createOverlayFeed();
	// One model off the same WS feed the OBS overlays consume.
	const model = $derived(buildVizModel(feed.game, feed.tick, feed.scenario, feed.objects));

	// Real Halo HUD icons for the map markers, decoded from the user's game files
	// into the served cache. Loads best-effort: if the cache was never
	// regenerated, this stays empty and every marker uses its generic shape.
	let iconSet = $state<IconSet>(emptyIconSet(GAME));

	// Real Blood Gulch BSP top-down projection drawn as the map background, so
	// dots sit on the actual layout instead of empty space. Best-effort: null →
	// blank grid (graceful degrade).
	let geometry = $state<TopProjection | null>(null);
	let loadedGeoKey = '';

	// Layer toggles (debug controls). The spawn layer seeds from the URL once
	// (untrack: a deliberate initial snapshot, not a live binding to the prop).
	let showGeometry = $state(true);
	let showSpawns = $state(untrack(() => data.showSpawns));
	let showItems = $state(true);
	let showVehicles = $state(true);
	let showProjectiles = $state(true);
	let showNames = $state(true);

	onMount(() => {
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			// game = roster/identity, tick = positions + health/shields, scenario =
			// map + spawns (stable bounds), objects = vehicles / dropped items /
			// projectiles (opt-in; absent → those layers stay empty placeholders).
			classes: ['game', 'tick', 'scenario', 'objects']
		});
		loadIconSet(GAME).then((s) => (iconSet = s));
	});
	onDestroy(() => feed.stop());

	// Load the map background when the scenario first becomes known (and again if
	// the map changes). Keyed on the slugified scenario so a re-emit doesn't reload.
	$effect(() => {
		const raw = feed.scenario?.map ?? '';
		const key = meshKeyForScenario(raw);
		if (!key || key === loadedGeoKey) return;
		loadedGeoKey = key;
		loadTopProjection(GAME, raw).then((g) => (geometry = g));
	});

	const teamLegend = $derived.by(() => {
		// Group placed players by team (plain object — transient, not reactive state).
		const acc: Record<number, { team: number; name: string; color: string; count: number }> = {};
		for (const pl of model.players) {
			const e = acc[pl.team];
			if (e) e.count += 1;
			else
				acc[pl.team] = { team: pl.team, name: teamMeta(pl.team).name, color: pl.color, count: 1 };
		}
		return Object.values(acc).sort((a, b) => a.team - b.team);
	});

	const weaponItems = $derived(
		model.items.filter((i) => i.kind === 'weapon' && i.heldBy == null).length
	);
	const powerupItems = $derived(
		model.items.filter((i) => i.kind === 'powerup' && i.heldBy == null).length
	);
	const phaseTitle = $derived(model.phase.charAt(0).toUpperCase() + model.phase.slice(1));
</script>

<svelte:head>
	<title>Visualizer · {data.instance}</title>
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
		</div>
		<div class="meta">
			<span class="stat" title="Players placed on the map / named in the roster">
				<strong>{model.placedCount}</strong>/{model.playerCount} players
			</span>
			<span class="conn" class:ok={feed.connected} class:mock={feed.mock}>
				{#if feed.mock}MOCK{:else if feed.connected}● LIVE{:else}○ Connecting…{/if}
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
					{geometry}
					{showGeometry}
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
				<label><input type="checkbox" bind:checked={showGeometry} /> Map</label>
				<label><input type="checkbox" bind:checked={showNames} /> Names</label>
				<label><input type="checkbox" bind:checked={showItems} /> Items</label>
				<label><input type="checkbox" bind:checked={showVehicles} /> Vehicles</label>
				<label><input type="checkbox" bind:checked={showProjectiles} /> Projectiles</label>
				<label><input type="checkbox" bind:checked={showSpawns} /> Spawns</label>
			</div>

			<section>
				<h3>Players <span class="muted">{model.placedCount}</span></h3>
				{#if !model.hasTick}
					<p class="note">No live tick — players not placed yet.</p>
				{:else if model.players.length === 0}
					<p class="note">Roster empty.</p>
				{:else}
					<ul class="keys">
						{#each teamLegend as t (t.team)}
							<li>
								<span class="swatch dot" style="--c: {t.color}"></span>
								{model.isTeamGame ? t.name : 'Players'}
								<span class="muted">{t.count}</span>
							</li>
						{/each}
						<li class="sub"><span class="swatch ring"></span> health / shield rings</li>
						<li class="sub"><span class="swatch local"></span> local player</li>
					</ul>
				{/if}
			</section>

			<section>
				<h3>Items <span class="muted">{weaponItems + powerupItems}</span></h3>
				<ul class="keys">
					<li>
						<span class="swatch diamond" style="--c: #e0a32e"></span> weapon
						<span class="muted">{weaponItems}</span>
					</li>
					<li>
						<span class="swatch diamond" style="--c: #9b51e0"></span> powerup
						<span class="muted">{powerupItems}</span>
					</li>
					{#if model.respawningItems > 0}
						<li class="sub">{model.respawningItems} respawning (off-map)</li>
					{/if}
				</ul>
			</section>

			<section>
				<h3>Vehicles <span class="muted">{model.vehicles.length}</span></h3>
				{#if !model.hasObjects}
					<p class="note">Objects class not in feed — vehicles &amp; dropped items unavailable.</p>
				{:else}
					<ul class="keys">
						<li><span class="swatch sq"></span> empty</li>
						<li><span class="swatch sq filled"></span> occupied</li>
					</ul>
				{/if}
			</section>

			<section>
				<h3>Flags <span class="muted">{model.flags.length}</span></h3>
				{#if model.flags.length === 0}
					<p class="note">None this gametype.</p>
				{:else}
					<ul class="keys">
						{#each model.flags as f, i (i)}
							<li>
								<span class="swatch dot" style="--c: {f.color}"></span>
								{f.label} — {f.status}
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="caps">
				<h3>Feed</h3>
				<ul class="capgrid">
					<li class:on={model.hasGame}>game</li>
					<li class:on={model.hasTick}>tick</li>
					<li class:on={model.hasScenario}>scenario</li>
					<li class:on={model.hasObjects}>objects</li>
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
		width: 16rem;
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
	.swatch.diamond {
		transform: rotate(45deg);
		background: var(--c);
		width: 10px;
		height: 10px;
	}
	.swatch.sq {
		border: 1.5px solid #cbd5e1;
		border-radius: 2px;
	}
	.swatch.sq.filled {
		background: rgba(203, 213, 225, 0.4);
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
