<script lang="ts">
	// SVG top-down render of a live VizModel. Pure drawing — all data shaping +
	// projection lives in $lib/utils/visualizer-view. Fixed 1000×1000 viewBox;
	// the projection fits the auto-derived world bounds into it (aspect-preserved,
	// Y-flipped so world +Y is screen-up / "north").
	import {
		makeProjection,
		healthColor,
		type VizModel,
		type VizPlayer
	} from '$lib/utils/visualizer-view';
	import type { Vec2, Vec3 } from '$lib/types/scraper-v2';

	let {
		model,
		showSpawns = false,
		showItems = true,
		showVehicles = true,
		showProjectiles = true,
		showNames = true
	}: {
		model: VizModel;
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
</script>

<svg
	viewBox="0 0 {VIEW} {VIEW}"
	class="map"
	preserveAspectRatio="xMidYMid meet"
	role="img"
	aria-label="Top-down view of the live match"
>
	<!-- backdrop + grid -->
	<rect x="0" y="0" width={VIEW} height={VIEW} class="bg" rx="10" />
	<g class="grid">
		{#each [1, 2, 3, 4, 5, 6, 7, 8, 9] as i (i)}
			<line x1={(VIEW / 10) * i} y1="0" x2={(VIEW / 10) * i} y2={VIEW} />
			<line x1="0" y1={(VIEW / 10) * i} x2={VIEW} y2={(VIEW / 10) * i} />
		{/each}
	</g>
	<rect x="1" y="1" width={VIEW - 2} height={VIEW - 2} class="frame" rx="10" />

	{#if !proj.valid}
		<text x={VIEW / 2} y={VIEW / 2} class="empty" text-anchor="middle">
			Waiting for spatial data…
		</text>
	{:else}
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
					<rect
						x={c.x - 8}
						y={c.y - 6}
						width="16"
						height="12"
						rx="3"
						class="veh"
						class:occupied={v.occupied}
					/>
					{#if showNames}
						<text x={c.x} y={c.y + 18} class="vlabel" text-anchor="middle">{v.label}</text>
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
					<path
						d="M {c.x} {c.y - 7} L {c.x + 7} {c.y} L {c.x} {c.y + 7} L {c.x - 7} {c.y} Z"
						fill={col}
						stroke="rgba(0,0,0,0.5)"
						stroke-width="1"
					/>
					{#if showNames}
						<text x={c.x} y={c.y - 11} class="ilabel" text-anchor="middle">{it.label}</text>
					{/if}
				{/each}
			</g>
		{/if}

		<!-- CTF flags -->
		<g class="flags">
			{#each model.flags as f, i (i)}
				{@const c = p(f.pos)}
				<line x1={c.x} y1={c.y + 8} x2={c.x} y2={c.y - 12} stroke="#e9edf2" stroke-width="2" />
				<path d="M {c.x} {c.y - 12} L {c.x + 12} {c.y - 8} L {c.x} {c.y - 4} Z" fill={f.color} />
				{#if f.carrier != null}
					<circle cx={c.x} cy={c.y} r="3" fill={f.color} opacity="0.6" />
				{/if}
			{/each}
		</g>

		<!-- players (drawn last, on top) -->
		<g class="players">
			{#each model.players as pl (pl.index)}
				{@const c = p(pl.pos)}
				<g class="player" class:dead={!pl.alive} opacity={pl.hasCamo && pl.alive ? 0.55 : 1}>
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
						<!-- shield + health rings -->
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
						<!-- body dot -->
						<circle
							cx={c.x}
							cy={c.y}
							r="6.5"
							fill={pl.color}
							stroke="rgba(0,0,0,0.55)"
							stroke-width="1"
						/>
						{#if pl.isLocal}
							<circle
								cx={c.x}
								cy={c.y}
								r="8.5"
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
						<text x={c.x} y={c.y - 14} class="respawn" text-anchor="middle">{respawnLabel(pl)}</text
						>
					{/if}

					{#if showNames}
						<text x={c.x} y={c.y + 26} class="pname" text-anchor="middle">{pl.name}</text>
					{/if}
				</g>
			{/each}
		</g>

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

<style>
	.map {
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
</style>
