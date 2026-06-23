<script lang="ts">
	// SVG top-down render of a live VizModel. Pure drawing — all data shaping +
	// projection lives in $lib/utils/visualizer-view. Fixed 1000×1000 viewBox;
	// the projection fits the auto-derived world bounds into it (aspect-preserved,
	// Y-flipped so world +Y is screen-up / "north").
	//
	// Spectator-readability additions (gated on a precomputed `floorplan`/`bands`,
	// so the live path with no mesh behaves exactly as before):
	//   - FLOORPLAN: a canvas layer behind the SVG fills floor triangles + outlines
	//     walls with ceilings cut, so you see into rooms (replaces the flat PNG).
	//   - ELEVATION SHADING: floors AND player bodies are shaded by height (lower =
	//     darker, higher = lighter), absolute or relative to a followed player.
	//   - FLOOR FILTER: hide bands not in `activeBands` (floors + their players).
	import {
		makeProjection,
		healthColor,
		type VizModel,
		type VizPlayer
	} from '$lib/utils/visualizer-view';
	import { emptyIconSet, type IconSet } from '$lib/utils/game-icons';
	import type { TopProjection } from '$lib/utils/game-geometry';
	import type { Floorplan } from '$lib/utils/floorplan';
	import { shadeForZ, bandColor, type FloorBands } from '$lib/utils/floor-bands';
	import type { Vec2, Vec3 } from '$lib/types/scraper-v2';

	let {
		model,
		icons = emptyIconSet('haloce'),
		geometry = null,
		floorplan = null,
		bands = null,
		showFloorplan = true,
		shadeMode = 'absolute',
		followZ = null,
		activeBands = null,
		showSpawns = false,
		showItems = true,
		showVehicles = true,
		showProjectiles = true,
		showNames = true
	}: {
		model: VizModel;
		/** Real-game-icon set (decoded from the user's game files); when a marker's
		 * icon isn't available, that marker falls back to its generic shape. */
		icons?: IconSet;
		/** Real BSP top-down projection (PNG + world bounds) drawn as the map
		 * background; used only when no `floorplan` is supplied (graceful degrade). */
		geometry?: TopProjection | null;
		/** Precomputed floorplan (floors/walls) → the cut-roof canvas layer. */
		floorplan?: Floorplan | null;
		/** Elevation bands → floor + player shading and the legend. */
		bands?: FloorBands | null;
		showFloorplan?: boolean;
		/** Shade floors/players by absolute height, or relative to `followZ`. */
		shadeMode?: 'absolute' | 'relative';
		/** Followed player's Z (the relative-mode baseline). */
		followZ?: number | null;
		/** Visible band indices; null/empty = show all. */
		activeBands?: number[] | null;
		showSpawns?: boolean;
		showItems?: boolean;
		showVehicles?: boolean;
		showProjectiles?: boolean;
		showNames?: boolean;
	} = $props();

	const VIEW = 1000;
	const PAD = 44;

	const proj = $derived(makeProjection(model.bounds, VIEW, VIEW, PAD));
	const p = (pos: Vec3): Vec2 => proj.project(pos);

	const floorplanActive = $derived(!!floorplan && !!bands && showFloorplan && proj.valid);

	const bandActive = (b: number): boolean =>
		!activeBands || activeBands.length === 0 || activeBands.includes(b);

	function playerBand(pl: VizPlayer): number {
		return bands ? bands.bandForZ(pl.pos.z) : 0;
	}

	function elevShade(z: number): string {
		if (!bands) return '#8b97a7';
		// Absolute → discrete per-band step (crisp floor bands that match the legend
		// exactly). Relative → smooth gradient around the followed player's height.
		if (shadeMode === 'relative') {
			return shadeForZ(z, {
				mode: 'relative',
				minZ: bands.minZ,
				maxZ: bands.maxZ,
				focusZ: followZ ?? undefined
			});
		}
		return bandColor(bands.bandForZ(z), bands.count);
	}

	// --- Floorplan canvas layer (1000×1000 buffer; container is square so it
	//     aligns 1:1 with the SVG's [0,1000] coordinate space) ------------------
	let canvasEl: HTMLCanvasElement | undefined = $state();

	$effect(() => {
		// Depend on everything that changes the drawn floorplan; NOT on the model
		// tick (geometry is static), so this only redraws on map/mode/filter change.
		const cv = canvasEl;
		if (!cv) return;
		const fp = floorplan;
		const bd = bands;
		const ctx = cv.getContext('2d');
		if (!ctx) return;
		ctx.clearRect(0, 0, VIEW, VIEW);
		if (!fp || !bd || !showFloorplan || !proj.valid) return;

		// Floors filled low→high (already sorted) so upper floors land on top.
		for (const f of fp.floors) {
			const b = bd.bandForZ(f.z);
			if (!bandActive(b)) continue;
			const a = p({ x: f.pts[0].x, y: f.pts[0].y, z: f.z });
			const c2 = p({ x: f.pts[1].x, y: f.pts[1].y, z: f.z });
			const c3 = p({ x: f.pts[2].x, y: f.pts[2].y, z: f.z });
			ctx.beginPath();
			ctx.moveTo(a.x, a.y);
			ctx.lineTo(c2.x, c2.y);
			ctx.lineTo(c3.x, c3.y);
			ctx.closePath();
			ctx.fillStyle = elevShade(f.z);
			ctx.fill();
		}
		// Wall outlines on top (faint, so room boundaries read).
		ctx.lineWidth = 1;
		ctx.strokeStyle = 'rgba(8, 11, 18, 0.55)';
		ctx.beginPath();
		for (const w of fp.walls) {
			const b = bd.bandForZ(w.z);
			if (!bandActive(b)) continue;
			const a = p({ x: w.a.x, y: w.a.y, z: w.z });
			const c = p({ x: w.b.x, y: w.b.y, z: w.z });
			ctx.moveTo(a.x, a.y);
			ctx.lineTo(c.x, c.y);
		}
		ctx.stroke();
	});

	// Place the (PNG) geometry image by projecting its world-bounds corners through
	// the SAME projection the live dots use → pixel-perfect alignment. Only used
	// when there's no floorplan layer.
	const geoRect = $derived.by(() => {
		if (!geometry || !proj.valid) return null;
		const tl = proj.project({ x: geometry.bounds.minX, y: geometry.bounds.maxY, z: 0 });
		const br = proj.project({ x: geometry.bounds.maxX, y: geometry.bounds.minY, z: 0 });
		return { x: tl.x, y: tl.y, w: Math.max(0, br.x - tl.x), h: Math.max(0, br.y - tl.y) };
	});

	const ITEM_COLOR: Record<string, string> = {
		weapon: '#e0a32e',
		powerup: '#9b51e0',
		equipment: '#27c0c4',
		other: '#8b97a7'
	};

	// --- arc helper for the health / shield rings (start at top, sweep CW) ---
	function polar(cx: number, cy: number, r: number, a: number): Vec2 {
		return { x: cx + r * Math.cos(a), y: cy + r * Math.sin(a) };
	}
	function arc(cx: number, cy: number, r: number, frac: number): string {
		const f = Math.max(0, Math.min(1, frac));
		if (f <= 0) return '';
		if (f >= 1) {
			// full circle as two half-arcs
			return `M ${cx} ${cy - r} A ${r} ${r} 0 1 1 ${cx - 0.01} ${cy - r}`;
		}
		const start = -Math.PI / 2;
		const end = start + f * 2 * Math.PI;
		const s = polar(cx, cy, r, start);
		const e = polar(cx, cy, r, end);
		const large = f > 0.5 ? 1 : 0;
		return `M ${s.x} ${s.y} A ${r} ${r} 0 ${large} 1 ${e.x} ${e.y}`;
	}

	function respawnLabel(pl: VizPlayer): string {
		if (pl.alive) return '';
		if (pl.respawnIn == null) return '↻';
		return `↻ ${Math.max(0, Math.ceil(pl.respawnIn / 30))}s`;
	}

	// --- scale bar (bottom-left): pick a "nice" world distance ≈ 150px wide ---
	function niceNumber(n: number): number {
		if (n <= 0) return 1;
		const exp = Math.floor(Math.log10(n));
		const base = Math.pow(10, exp);
		const f = n / base;
		const nice = f < 1.5 ? 1 : f < 3.5 ? 2 : f < 7.5 ? 5 : 10;
		return nice * base;
	}
	const scaleBar = $derived.by(() => {
		if (!proj.valid || proj.scale <= 0) return null;
		const worldU = niceNumber(150 / proj.scale);
		return { px: worldU * proj.scale, label: `${worldU} wu` };
	});

	// Elevation legend rows (bottom-right), one per band, high→low.
	const legendRows = $derived.by(() => {
		if (!bands || !floorplanActive) return [];
		return bands.bands
			.map((b) => ({
				index: b.index,
				color: bandColor(b.index, bands.count),
				label: `${b.minZ.toFixed(1)}–${b.maxZ.toFixed(1)}`,
				on: bandActive(b.index)
			}))
			.reverse();
	});
</script>

<!-- A real-game-icon marker: the white HUD glyph on a dark disc with a thin
     category/team-colored ring, sized into a fixed box (aspect-preserved). -->
{#snippet iconMarker(cx: number, cy: number, href: string, ring: string, size: number)}
	<circle {cx} {cy} r={size * 0.6} class="icon-bg" />
	<image
		{href}
		x={cx - size / 2}
		y={cy - size / 2}
		width={size}
		height={size}
		preserveAspectRatio="xMidYMid meet"
	/>
	<circle
		{cx}
		{cy}
		r={size * 0.6}
		fill="none"
		stroke={ring}
		stroke-width="1.4"
		stroke-opacity="0.8"
	/>
{/snippet}

<div class="maproot">
	{#if floorplanActive}
		<canvas bind:this={canvasEl} class="floorcanvas" width={VIEW} height={VIEW}></canvas>
	{/if}
	<svg
		viewBox="0 0 {VIEW} {VIEW}"
		class="map"
		preserveAspectRatio="xMidYMid meet"
		role="img"
		aria-label="Top-down view of the live match"
	>
		<!-- backdrop + grid (suppressed when the floorplan canvas is the background) -->
		{#if !floorplanActive}
			<rect x="0" y="0" width={VIEW} height={VIEW} class="bg" rx="10" />
			<g class="grid">
				{#each [1, 2, 3, 4, 5, 6, 7, 8, 9] as i (i)}
					<line x1={(VIEW / 10) * i} y1="0" x2={(VIEW / 10) * i} y2={VIEW} />
					<line x1="0" y1={(VIEW / 10) * i} x2={VIEW} y2={(VIEW / 10) * i} />
				{/each}
			</g>
		{/if}
		<rect x="1" y="1" width={VIEW - 2} height={VIEW - 2} class="frame" rx="10" />

		{#if !proj.valid}
			<text x={VIEW / 2} y={VIEW / 2} class="empty" text-anchor="middle">
				Waiting for spatial data…
			</text>
		{:else}
			<!-- real BSP top-down projection (only when there's no floorplan layer) -->
			{#if !floorplanActive && showFloorplan && geometry && geoRect}
				<image
					href={geometry.url}
					x={geoRect.x}
					y={geoRect.y}
					width={geoRect.w}
					height={geoRect.h}
					preserveAspectRatio="none"
					opacity="0.95"
				/>
			{/if}

			<!-- static spawns (reference layer) -->
			{#if showSpawns}
				<g class="spawns">
					{#each model.spawns as s, i (i)}
						{@const c = p(s.pos)}
						<path
							d="M {c.x - 5} {c.y} L {c.x} {c.y - 5} L {c.x + 5} {c.y} L {c.x} {c.y + 5} Z"
							fill={s.color}
							opacity="0.28"
						/>
					{/each}
				</g>
			{/if}

			<!-- projectiles (faint) -->
			{#if showProjectiles}
				<g class="projectiles">
					{#each model.projectiles as pr (pr.id)}
						{@const c = p(pr.pos)}
						<circle cx={c.x} cy={c.y} r="2.5" fill="#ffd166" opacity="0.7" />
					{/each}
				</g>
			{/if}

			<!-- vehicles -->
			{#if showVehicles}
				<g class="vehicles">
					{#each model.vehicles as v (v.id)}
						{@const c = p(v.pos)}
						{@const href = icons.vehicleUrl(v.tag)}
						{#if href}
							{@render iconMarker(c.x, c.y, href, v.occupied ? '#e9edf2' : '#cbd5e1', 26)}
							{#if v.occupied}
								<circle cx={c.x} cy={c.y} r="2.5" fill="#e9edf2" opacity="0.85" />
							{/if}
						{:else}
							<rect
								x={c.x - 8}
								y={c.y - 6}
								width="16"
								height="12"
								rx="3"
								class="veh"
								class:occupied={v.occupied}
							/>
						{/if}
						{#if showNames}
							<text x={c.x} y={c.y + (href ? 22 : 18)} class="vlabel" text-anchor="middle"
								>{v.label}</text
							>
						{/if}
					{/each}
				</g>
			{/if}

			<!-- world items -->
			{#if showItems}
				<g class="items">
					{#each model.items.filter((it) => it.heldBy == null) as it (it.id)}
						{@const c = p(it.pos)}
						{@const col = ITEM_COLOR[it.kind] ?? ITEM_COLOR.other}
						{@const href = icons.itemUrl(it.tag, it.kind)}
						{#if href}
							{@render iconMarker(c.x, c.y, href, col, 24)}
						{:else}
							<path
								d="M {c.x} {c.y - 7} L {c.x + 7} {c.y} L {c.x} {c.y + 7} L {c.x - 7} {c.y} Z"
								fill={col}
								stroke="rgba(0,0,0,0.5)"
								stroke-width="1"
							/>
						{/if}
						{#if showNames}
							<text x={c.x} y={c.y - (href ? 15 : 11)} class="ilabel" text-anchor="middle"
								>{it.label}</text
							>
						{/if}
					{/each}
				</g>
			{/if}

			<!-- CTF flags -->
			<g class="flags">
				{#each model.flags as f, i (i)}
					{@const c = p(f.pos)}
					{@const href = icons.flagUrl()}
					{#if href}
						{@render iconMarker(c.x, c.y, href, f.color, 24)}
						{#if f.carrier != null}
							<circle cx={c.x} cy={c.y} r="2.5" fill={f.color} opacity="0.85" />
						{/if}
					{:else}
						<line x1={c.x} y1={c.y + 8} x2={c.x} y2={c.y - 12} stroke="#e9edf2" stroke-width="2" />
						<path
							d="M {c.x} {c.y - 12} L {c.x + 12} {c.y - 8} L {c.x} {c.y - 4} Z"
							fill={f.color}
						/>
						{#if f.carrier != null}
							<circle cx={c.x} cy={c.y} r="3" fill={f.color} opacity="0.6" />
						{/if}
					{/if}
				{/each}
			</g>

			<!-- players (drawn last, on top) -->
			<g class="players">
				{#each model.players as pl (pl.index)}
					{@const c = p(pl.pos)}
					{@const eb = playerBand(pl)}
					{@const dimmed = bands ? !bandActive(eb) : false}
					{@const bodyFill = bands ? elevShade(pl.pos.z) : pl.color}
					<g
						class="player"
						class:dead={!pl.alive}
						opacity={dimmed ? 0.18 : pl.hasCamo && pl.alive ? 0.55 : 1}
					>
						{#if pl.hasOvershield && pl.alive}
							<circle cx={c.x} cy={c.y} r="16" fill={pl.color} opacity="0.18" />
						{/if}

						<!-- heading tick -->
						{#if pl.heading != null && pl.alive}
							<line
								x1={c.x}
								y1={c.y}
								x2={c.x + Math.cos(pl.heading) * 17}
								y2={c.y + Math.sin(pl.heading) * 17}
								stroke={pl.color}
								stroke-width="2.5"
								stroke-linecap="round"
							/>
						{/if}

						{#if pl.alive}
							{#if !bands}
								<!-- shield + health rings (live mode without elevation shading) -->
								<circle cx={c.x} cy={c.y} r="13" class="ring-track" />
								{#if pl.shields > 0}
									<path d={arc(c.x, c.y, 13, pl.shields)} class="shield ring" fill="none" />
								{/if}
								<circle cx={c.x} cy={c.y} r="10" class="ring-track" />
								<path
									d={arc(c.x, c.y, 10, pl.health)}
									class="ring"
									fill="none"
									stroke={healthColor(pl.health)}
								/>
							{/if}
							<!-- body dot: fill = elevation shade (when banded), ring = team color -->
							<circle
								cx={c.x}
								cy={c.y}
								r={bands ? 8 : 6.5}
								fill={bodyFill}
								stroke={pl.color}
								stroke-width={bands ? 3 : 1}
							/>
							{#if pl.isLocal}
								<circle
									cx={c.x}
									cy={c.y}
									r={bands ? 11 : 8.5}
									fill="none"
									stroke="#fff"
									stroke-width="1.2"
									opacity="0.9"
								/>
							{/if}
						{:else}
							<!-- dead: hollow marker + respawn timer -->
							<circle
								cx={c.x}
								cy={c.y}
								r="6.5"
								fill="none"
								stroke={pl.color}
								stroke-width="1.5"
								stroke-dasharray="3 2"
							/>
							<text x={c.x} y={c.y - 14} class="respawn" text-anchor="middle"
								>{respawnLabel(pl)}</text
							>
						{/if}

						{#if showNames}
							<text x={c.x} y={c.y + 26} class="pname" text-anchor="middle">{pl.name}</text>
						{/if}
					</g>
				{/each}
			</g>

			<!-- elevation legend (bottom-right) -->
			{#if legendRows.length > 0}
				<g
					class="elegend"
					transform="translate({VIEW - 158}, {VIEW - 34 - legendRows.length * 22})"
				>
					<rect
						x="-12"
						y="-26"
						width="158"
						height={legendRows.length * 22 + 34}
						rx="8"
						class="elpanel"
					/>
					<text x="0" y="-9" class="eltitle">Elevation (wu)</text>
					{#each legendRows as row, i (row.index)}
						<g transform="translate(0, {i * 22})" opacity={row.on ? 1 : 0.35}>
							<rect
								x="0"
								y="0"
								width="16"
								height="16"
								fill={row.color}
								rx="2"
								stroke="rgba(0,0,0,0.5)"
							/>
							<text x="22" y="13" class="ellabel">{row.label}</text>
						</g>
					{/each}
				</g>
			{/if}

			<!-- scale bar + compass (screen-space) -->
			{#if scaleBar}
				<g class="scalebar">
					<line x1={PAD} y1={VIEW - 24} x2={PAD + scaleBar.px} y2={VIEW - 24} />
					<line x1={PAD} y1={VIEW - 28} x2={PAD} y2={VIEW - 20} />
					<line x1={PAD + scaleBar.px} y1={VIEW - 28} x2={PAD + scaleBar.px} y2={VIEW - 20} />
					<text x={PAD} y={VIEW - 32} class="sblabel">{scaleBar.label}</text>
				</g>
			{/if}
			<g class="compass">
				<line x1={VIEW - 30} y1={42} x2={VIEW - 30} y2={20} stroke="#9aa4b2" stroke-width="2" />
				<path d="M {VIEW - 30} {18} L {VIEW - 34} {26} L {VIEW - 26} {26} Z" fill="#9aa4b2" />
				<text x={VIEW - 30} y={56} class="sblabel" text-anchor="middle">N</text>
			</g>
		{/if}
	</svg>
</div>

<style>
	.maproot {
		position: relative;
		width: 100%;
		height: 100%;
	}
	.floorcanvas {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		background: #0b0f17;
		border-radius: 10px;
	}
	.map {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		display: block;
	}
	.bg {
		fill: #0b0f17;
	}
	.frame {
		fill: none;
		stroke: rgba(255, 255, 255, 0.1);
		stroke-width: 1.5;
	}
	.grid line {
		stroke: rgba(255, 255, 255, 0.04);
		stroke-width: 1;
	}
	.empty {
		fill: rgba(255, 255, 255, 0.4);
		font-size: 26px;
		font-family: 'Inter', system-ui, sans-serif;
	}

	.ring-track {
		fill: none;
		stroke: rgba(255, 255, 255, 0.1);
		stroke-width: 2.5;
	}
	.ring {
		stroke-width: 2.5;
		stroke-linecap: round;
	}
	.ring.shield {
		stroke: #5cc8ff;
	}

	.icon-bg {
		fill: rgba(8, 11, 18, 0.78);
	}

	.veh {
		fill: none;
		stroke: #cbd5e1;
		stroke-width: 1.6;
	}
	.veh.occupied {
		fill: rgba(203, 213, 225, 0.35);
	}

	.player.dead {
		opacity: 0.5;
	}

	text {
		font-family: 'Inter', system-ui, sans-serif;
		paint-order: stroke;
		stroke: rgba(0, 0, 0, 0.85);
		stroke-width: 3px;
		stroke-linejoin: round;
	}
	.pname {
		fill: #fff;
		font-size: 14px;
		font-weight: 600;
	}
	.respawn {
		fill: #ffd166;
		font-size: 13px;
		font-weight: 700;
	}
	.vlabel {
		fill: #cbd5e1;
		font-size: 11px;
	}
	.ilabel {
		fill: #e8d6a8;
		font-size: 11px;
		font-weight: 500;
	}
	.sblabel {
		fill: #9aa4b2;
		font-size: 13px;
		font-weight: 600;
	}
	.scalebar line {
		stroke: #9aa4b2;
		stroke-width: 2;
	}
	.elpanel {
		fill: rgba(8, 11, 18, 0.82);
		stroke: rgba(255, 255, 255, 0.12);
		stroke-width: 1;
	}
	.eltitle {
		fill: #cbd5e1;
		font-size: 13px;
		font-weight: 700;
	}
	.ellabel {
		fill: #cbd5e1;
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}
</style>
