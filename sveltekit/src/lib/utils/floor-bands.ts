// Reach-style elevation banding for the spectator-readability visualizer.
//
// "Discrete floor bands clustered from the geometry's elevations" + the shared
// shade ramp the 2D floorplan paints its floors with AND the player icons tint
// to (a player's shade matches their floor). Pure / IO-free so the clustering +
// the colour math — the part that has to be CORRECT and reproducible — unit-test
// without a browser.
//
// Source data is the already-extracted BSP mesh (game-geometry cache): we cluster
// the WALKABLE-floor elevations (1D k-means over floor-triangle mid-Z) rather than
// every vertex, so tall walls / ceilings don't smear the bands. The result is a
// small set of contiguous Z ranges (low → high) + a deterministic colour ramp.

const RAMP_LO = { r: 0x1b, g: 0x24, b: 0x34 }; // deep slate (lowest floor)
const RAMP_HI = { r: 0xc4, g: 0xcf, b: 0xde }; // soft light blue-grey (highest floor)

export interface FloorBand {
	index: number;
	/** Data Z-range of the floors assigned to this band (for the legend). */
	minZ: number;
	maxZ: number;
	/** Cluster centroid Z (the band's representative height). */
	midZ: number;
	/** How many input samples landed in this band. */
	count: number;
}

export interface FloorBands {
	count: number;
	bands: FloorBand[];
	/** Overall Z range across all bands. */
	minZ: number;
	maxZ: number;
	/** Band index (0 = lowest) for a world Z, by nearest contiguous boundary. */
	bandForZ(z: number): number;
}

/** Deterministic 1D k-means: quantile-seeded centroids, fixed iteration cap, no
 *  RNG (so the same mesh always yields the same bands). Returns sorted centroids
 *  with no empty clusters (k is reduced to the count of distinct values). */
function kmeans1d(values: number[], k: number): number[] {
	const xs = values.filter((v) => Number.isFinite(v)).sort((a, b) => a - b);
	if (xs.length === 0) return [];
	const distinct = Array.from(new Set(xs));
	const kk = Math.max(1, Math.min(k, distinct.length));
	if (kk === distinct.length) return distinct.slice();

	// Quantile seeds — evenly spaced through the sorted data, stable across runs.
	let centroids: number[] = [];
	for (let i = 0; i < kk; i++) {
		const q = (i + 0.5) / kk;
		centroids.push(xs[Math.min(xs.length - 1, Math.floor(q * xs.length))]);
	}

	for (let iter = 0; iter < 40; iter++) {
		const sums = new Array(kk).fill(0);
		const counts = new Array(kk).fill(0);
		for (const x of xs) {
			let best = 0;
			let bestD = Infinity;
			for (let c = 0; c < kk; c++) {
				const d = Math.abs(x - centroids[c]);
				if (d < bestD) {
					bestD = d;
					best = c;
				}
			}
			sums[best] += x;
			counts[best] += 1;
		}
		let moved = false;
		const next = centroids.slice();
		for (let c = 0; c < kk; c++) {
			if (counts[c] > 0) {
				const m = sums[c] / counts[c];
				if (Math.abs(m - centroids[c]) > 1e-6) moved = true;
				next[c] = m;
			}
		}
		centroids = next;
		if (!moved) break;
	}
	return centroids.sort((a, b) => a - b);
}

/**
 * Cluster a set of walkable-floor elevations into up to `maxBands` discrete
 * floor bands (low → high). Pass floor-triangle mid-Z values; empty input yields
 * a single degenerate band so callers never divide by zero.
 */
export function computeFloorBands(floorZs: number[], maxBands = 5): FloorBands {
	const finite = floorZs.filter((z) => Number.isFinite(z));
	if (finite.length === 0) {
		const band: FloorBand = { index: 0, minZ: 0, maxZ: 0, midZ: 0, count: 0 };
		return { count: 1, bands: [band], minZ: 0, maxZ: 0, bandForZ: () => 0 };
	}

	const centroids = kmeans1d(finite, maxBands);
	const k = centroids.length;
	// Contiguous boundaries = midpoints between consecutive centroids.
	const boundaries: number[] = [];
	for (let i = 0; i < k - 1; i++) boundaries.push((centroids[i] + centroids[i + 1]) / 2);

	const bandForZ = (z: number): number => {
		if (!Number.isFinite(z)) return 0;
		let i = 0;
		while (i < boundaries.length && z >= boundaries[i]) i++;
		return i;
	};

	// Per-band data ranges (for the legend) from the actual assignments.
	const mins = new Array(k).fill(Infinity);
	const maxs = new Array(k).fill(-Infinity);
	const counts = new Array(k).fill(0);
	for (const z of finite) {
		const b = bandForZ(z);
		if (z < mins[b]) mins[b] = z;
		if (z > maxs[b]) maxs[b] = z;
		counts[b] += 1;
	}
	const bands: FloorBand[] = [];
	for (let i = 0; i < k; i++) {
		const has = counts[i] > 0;
		bands.push({
			index: i,
			minZ: has ? mins[i] : centroids[i],
			maxZ: has ? maxs[i] : centroids[i],
			midZ: centroids[i],
			count: counts[i]
		});
	}
	let lo = Infinity;
	let hi = -Infinity;
	for (const z of finite) {
		if (z < lo) lo = z;
		if (z > hi) hi = z;
	}
	return { count: k, bands, minZ: lo, maxZ: hi, bandForZ };
}

function mixChannel(a: number, b: number, t: number): number {
	return Math.round(a + (b - a) * Math.max(0, Math.min(1, t)));
}

function rampColor(t: number): string {
	const r = mixChannel(RAMP_LO.r, RAMP_HI.r, t);
	const g = mixChannel(RAMP_LO.g, RAMP_HI.g, t);
	const b = mixChannel(RAMP_LO.b, RAMP_HI.b, t);
	return `rgb(${r}, ${g}, ${b})`;
}

/** Absolute-mode shade for a floor band: lower = darker, higher = lighter,
 *  spread evenly across the band count. */
export function bandColor(index: number, count: number): string {
	if (count <= 1) return rampColor(0.5);
	return rampColor(index / (count - 1));
}

/**
 * Shade ramp value for a world Z. In `absolute` mode the whole map's Z range maps
 * onto the ramp; in `relative` mode the followed player's Z sits at the ramp's
 * mid-tone and nearby floors read neutral while higher/lower ones lighten/darken
 * (so a spectator following one player sees that player's plane as the baseline).
 */
export function shadeForZ(
	z: number,
	opts: {
		mode: 'absolute' | 'relative';
		minZ: number;
		maxZ: number;
		focusZ?: number;
		/** Half-range (world units) mapped to the ramp edges in relative mode. */
		relativeRange?: number;
	}
): string {
	if (!Number.isFinite(z)) return rampColor(0.5);
	if (opts.mode === 'relative') {
		const focus = Number.isFinite(opts.focusZ ?? NaN)
			? (opts.focusZ as number)
			: (opts.minZ + opts.maxZ) / 2;
		const range =
			opts.relativeRange && opts.relativeRange > 0
				? opts.relativeRange
				: Math.max(1, (opts.maxZ - opts.minZ) / 2);
		const t = 0.5 + (z - focus) / (2 * range);
		return rampColor(t);
	}
	const span = opts.maxZ - opts.minZ;
	const t = span > 1e-6 ? (z - opts.minZ) / span : 0.5;
	return rampColor(t);
}
