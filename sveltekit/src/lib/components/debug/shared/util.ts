// Helpers shared across the debug dashboard components.
// Halo CE engine ticks at 30Hz; "engine time" derived from a tick value is
// uptime-since-boot rather than match-relative (the match-start tick isn't
// surfaced in GameData yet — see plan note for the M6c+1 follow-up).

export const ENGINE_HZ = 30;

export function fmtTickAsTime(ticks: number | undefined): string {
	if (ticks === undefined) return '—';
	const totalSeconds = Math.floor(ticks / ENGINE_HZ);
	const h = Math.floor(totalSeconds / 3600);
	const m = Math.floor((totalSeconds % 3600) / 60);
	const s = totalSeconds % 60;
	const pad = (n: number) => n.toString().padStart(2, '0');
	return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

export function fmtTicksAsDuration(ticks: number | undefined | null): string {
	if (!ticks || ticks <= 0) return 'none';
	return fmtTickAsTime(ticks);
}

export function fmtScoreLimit(n: number | undefined | null): string {
	if (!n || n <= 0) return 'none';
	return String(n);
}

export function fmtPct(v: number | undefined | null): string {
	if (v === undefined || v === null) return '—';
	return `${Math.round(Math.max(0, Math.min(100, v * 100)))}%`;
}

export function pctClamped(v: number | undefined | null): number {
	if (v === undefined || v === null) return 0;
	return Math.round(Math.max(0, Math.min(100, v * 100)));
}

// Like fmtPct but without the 100% upper clamp — for raw values that can
// legitimately exceed 1.0 (e.g. shields when overshielded reads 0–3).
export function fmtPctRaw(v: number | undefined | null): string {
	if (v === undefined || v === null) return '—';
	return `${Math.round(Math.max(0, v) * 100)}%`;
}

// Splits a raw shield value (0–3 range when overshielded) into three 0–100
// layer widths for stacked-bar rendering. Layer A is the regular shield
// (0–1); B and C are the two overshield tiers (1–2 and 2–3) that overlay
// the base bar in the in-game HUD style.
export function overshieldLayers(shields: number | undefined | null): {
	a: number;
	b: number;
	c: number;
} {
	if (shields === undefined || shields === null) return { a: 0, b: 0, c: 0 };
	const s = Math.max(0, shields);
	return {
		a: Math.min(s, 1) * 100,
		b: Math.max(0, Math.min(s - 1, 1)) * 100,
		c: Math.max(0, Math.min(s - 2, 1)) * 100
	};
}

// Halo team mapping. Halo: CE shipped with Red/Blue (CTF/Slayer Team); Halo 2
// extended the palette to 8 teams: Red, Blue, Green, Yellow, Orange, Purple,
// Brown, Pink. Indices beyond 7 fall back to a generic label.
export function teamLabel(team: number): string {
	if (team === 0) return 'Red';
	if (team === 1) return 'Blue';
	if (team === 2) return 'Green';
	if (team === 3) return 'Yellow';
	if (team === 4) return 'Orange';
	if (team === 5) return 'Purple';
	if (team === 6) return 'Brown';
	if (team === 7) return 'Pink';
	return `Team ${team}`;
}

// Backed by theme-agnostic team tokens defined in routes/layout.css so team
// identity stays constant regardless of which Skeleton theme is active.
export function teamAccent(team: number): { bg: string; dot: string } {
	if (team === 0) return { bg: 'bg-team-red-500/20', dot: 'bg-team-red-500' };
	if (team === 1) return { bg: 'bg-team-blue-500/20', dot: 'bg-team-blue-500' };
	if (team === 2) return { bg: 'bg-team-green-500/20', dot: 'bg-team-green-500' };
	if (team === 3) return { bg: 'bg-team-yellow-500/20', dot: 'bg-team-yellow-500' };
	if (team === 4) return { bg: 'bg-team-orange-500/20', dot: 'bg-team-orange-500' };
	if (team === 5) return { bg: 'bg-team-purple-500/20', dot: 'bg-team-purple-500' };
	if (team === 6) return { bg: 'bg-team-brown-500/20', dot: 'bg-team-brown-500' };
	if (team === 7) return { bg: 'bg-team-pink-500/20', dot: 'bg-team-pink-500' };
	return { bg: 'bg-surface-500/20', dot: 'bg-surface-500' };
}

// Halo CE armor color palette — 18 entries, index 0–17. Source: c20.reclaimers.net
// hard-coded-data table. Halo 2 adds wider colors (steel/crimson/olive/mauve/
// rose/gold); when an H2 plugin lands, define a HALO_2_ARMOR_PALETTE alongside
// and pass it explicitly to armorLabel/armorAccent.
export const HALO_CE_ARMOR_PALETTE = [
	'white',
	'black',
	'red',
	'blue',
	'gray',
	'yellow',
	'green',
	'pink',
	'purple',
	'cyan',
	'cobalt',
	'orange',
	'teal',
	'sage',
	'brown',
	'tan',
	'maroon',
	'salmon'
] as const;

export function armorLabel(
	color: number,
	palette: readonly string[] = HALO_CE_ARMOR_PALETTE
): string {
	if (color < 0 || color >= palette.length) return 'Unknown';
	const name = palette[color];
	return name.charAt(0).toUpperCase() + name.slice(1);
}

// Static class lookup for the armor palette. Tailwind v4 only emits utilities
// for class names that appear literally in scanned source — `bg-armor-${name}`
// template literals are invisible to the JIT, so listing each combination by
// hand is what makes the swatches actually render. Covers CE (18) + H2 (6)
// names so the H2 plugin doesn't hit the same gotcha when it lands.
const ARMOR_CLASSES: Record<string, { bg: string; dot: string }> = {
	white: { bg: 'bg-armor-white/20', dot: 'bg-armor-white' },
	black: { bg: 'bg-armor-black/20', dot: 'bg-armor-black' },
	red: { bg: 'bg-armor-red/20', dot: 'bg-armor-red' },
	blue: { bg: 'bg-armor-blue/20', dot: 'bg-armor-blue' },
	gray: { bg: 'bg-armor-gray/20', dot: 'bg-armor-gray' },
	yellow: { bg: 'bg-armor-yellow/20', dot: 'bg-armor-yellow' },
	green: { bg: 'bg-armor-green/20', dot: 'bg-armor-green' },
	pink: { bg: 'bg-armor-pink/20', dot: 'bg-armor-pink' },
	purple: { bg: 'bg-armor-purple/20', dot: 'bg-armor-purple' },
	cyan: { bg: 'bg-armor-cyan/20', dot: 'bg-armor-cyan' },
	cobalt: { bg: 'bg-armor-cobalt/20', dot: 'bg-armor-cobalt' },
	orange: { bg: 'bg-armor-orange/20', dot: 'bg-armor-orange' },
	teal: { bg: 'bg-armor-teal/20', dot: 'bg-armor-teal' },
	sage: { bg: 'bg-armor-sage/20', dot: 'bg-armor-sage' },
	brown: { bg: 'bg-armor-brown/20', dot: 'bg-armor-brown' },
	tan: { bg: 'bg-armor-tan/20', dot: 'bg-armor-tan' },
	maroon: { bg: 'bg-armor-maroon/20', dot: 'bg-armor-maroon' },
	salmon: { bg: 'bg-armor-salmon/20', dot: 'bg-armor-salmon' },
	steel: { bg: 'bg-armor-steel/20', dot: 'bg-armor-steel' },
	crimson: { bg: 'bg-armor-crimson/20', dot: 'bg-armor-crimson' },
	olive: { bg: 'bg-armor-olive/20', dot: 'bg-armor-olive' },
	mauve: { bg: 'bg-armor-mauve/20', dot: 'bg-armor-mauve' },
	rose: { bg: 'bg-armor-rose/20', dot: 'bg-armor-rose' },
	gold: { bg: 'bg-armor-gold/20', dot: 'bg-armor-gold' }
};

// Backed by theme-agnostic --color-armor-* tokens in routes/layout.css.
// Fallback to surface token for any out-of-range index so unknown values
// render as a neutral swatch rather than throwing.
export function armorAccent(
	color: number,
	palette: readonly string[] = HALO_CE_ARMOR_PALETTE
): { bg: string; dot: string } {
	if (color < 0 || color >= palette.length) {
		return { bg: 'bg-surface-500/20', dot: 'bg-surface-500' };
	}
	return ARMOR_CLASSES[palette[color]] ?? { bg: 'bg-surface-500/20', dot: 'bg-surface-500' };
}

// Tailwind text-color utility mirroring armorAccent's bg-* tokens. Derived by
// swapping the bg- prefix for text-, so the same static class registry feeds
// both swatch backgrounds and inline player-name text colors.
export function armorTextClass(
	color: number,
	palette: readonly string[] = HALO_CE_ARMOR_PALETTE
): string {
	return armorAccent(color, palette).dot.replace('bg-', 'text-');
}

// Health bar color tier — matches Halo's red→yellow→green tradition rather
// than Skeleton's success-only bar.
export function meterTone(pct: number): string {
	if (pct <= 0) return 'bg-surface-500';
	if (pct < 25) return 'bg-error-500';
	if (pct < 60) return 'bg-warning-500';
	return 'bg-success-500';
}

// Shield bar color tier — cyan when healthy, falling through yellow/red as
// the shield breaks. Cyan matches Halo's HUD shield tint and is registered
// theme-agnostically via --color-shield-500 in routes/layout.css.
export function shieldMeterTone(pct: number): string {
	if (pct <= 0) return 'bg-surface-500';
	if (pct < 25) return 'bg-error-500';
	if (pct < 60) return 'bg-warning-500';
	return 'bg-shield-500';
}

// Scraper-phase pill colors. Tracks the Phase union from $lib/types/scraper.
export const PHASE_BADGE_CLASS: Record<string, string> = {
	idle: 'preset-tonal',
	ready: 'preset-filled-warning-500',
	live: 'preset-filled-success-500'
};
