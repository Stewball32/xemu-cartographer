// Per-game visual language for the broadcast browser-source graphics.
//
// The scoreboard + player-card OBS overlays render the SAME live data through a
// game-specific skin so a Halo: CE stream and a Halo 2 stream look native to
// their era:
//   - CE  — UNSC field kit. Warm amber + olive-green, chunky stencil caps,
//           chamfered "gun-metal" plates. Reads like the original Xbox HUD.
//   - H2  — the E3-blue menu language. Cool cyan/blue glass, thin sheens,
//           sharper diagonal cuts. Reads like Halo 2's sleeker UI.
//
// Everything is emitted as CSS custom properties on a wrapper element, so the
// route markup + shared card component are game-agnostic and just consume
// `var(--bc-accent)` etc. Pure (no IO, no Svelte) so it unit-tests + tree-shakes.
//
// The two palettes here are the OVERLAY chrome (frames, headers, accents). The
// per-player Spartan tint still comes from the armor palette in $lib/utils/emblem
// (CE_COLORS / H2_COLORS) — this file never recolours a player.

export type BroadcastGame = 'ce' | 'h2';

export interface BroadcastTheme {
	game: BroadcastGame;
	/** Short wordmark for headers, e.g. "HALO: CE" / "HALO 2". */
	label: string;
	/** Primary accent (rules, active glow, team-neutral highlights). */
	accent: string;
	/** Secondary accent (score numerals, secondary rules). */
	accent2: string;
	/** Main ink for names/values. */
	ink: string;
	/** Muted ink for labels/units. */
	inkMuted: string;
	/** Panel body fill (semi-opaque so it composites over gameplay). */
	panel: string;
	/** Panel body fill, denser (headers, score chips). */
	panelStrong: string;
	/** Hairline / plate edge. */
	edge: string;
	/** Accent glow colour (used in box-shadows / text-shadows). */
	glow: string;
	/** Corner radius for plates — CE is near-square, H2 is softer. */
	radius: string;
	/** Header background gradient. */
	headerGrad: string;
	/** Font stack (both ship Rajdhani; kept per-theme for future divergence). */
	font: string;
	/** Header tracking (letter-spacing) — H2 runs wider/airier. */
	tracking: string;
}

const CE_THEME: BroadcastTheme = {
	game: 'ce',
	label: 'HALO: CE',
	accent: '#f5c451', // amber
	accent2: '#5bbf7a', // UNSC green
	ink: '#f4f1e9',
	inkMuted: 'rgba(244, 241, 233, 0.62)',
	panel: 'rgba(14, 17, 12, 0.74)',
	panelStrong: 'rgba(9, 12, 8, 0.9)',
	edge: 'rgba(245, 196, 81, 0.42)',
	glow: 'rgba(245, 196, 81, 0.55)',
	radius: '3px',
	headerGrad: 'linear-gradient(180deg, rgba(31, 33, 20, 0.95), rgba(12, 14, 9, 0.9))',
	font: "'Rajdhani', 'Inter', system-ui, sans-serif",
	tracking: '0.08em'
};

const H2_THEME: BroadcastTheme = {
	game: 'h2',
	label: 'HALO 2',
	accent: '#4aa8ff', // E3 blue
	accent2: '#5fe6ff', // cyan
	ink: '#eef6ff',
	inkMuted: 'rgba(238, 246, 255, 0.6)',
	panel: 'rgba(9, 16, 26, 0.72)',
	panelStrong: 'rgba(6, 12, 22, 0.9)',
	edge: 'rgba(95, 200, 255, 0.42)',
	glow: 'rgba(74, 168, 255, 0.6)',
	radius: '7px',
	headerGrad: 'linear-gradient(180deg, rgba(14, 34, 58, 0.95), rgba(7, 16, 30, 0.9))',
	font: "'Rajdhani', 'Inter', system-ui, sans-serif",
	tracking: '0.14em'
};

const THEMES: Record<BroadcastGame, BroadcastTheme> = { ce: CE_THEME, h2: H2_THEME };

/** Normalise an arbitrary `?game=` value to a supported theme key. Defaults to
 * CE — the game whose live scrape is fully verified today. */
export function parseGame(raw: string | null | undefined): BroadcastGame {
	return raw != null && raw.toLowerCase() === 'h2' ? 'h2' : 'ce';
}

export function broadcastTheme(game: BroadcastGame): BroadcastTheme {
	return THEMES[game];
}

/** Serialise a theme to a `--bc-*` CSS-variable declaration string for a wrapper
 * element's `style=` — the markup/CSS then reads `var(--bc-accent)` and stays
 * game-agnostic. */
export function themeVars(t: BroadcastTheme): string {
	return [
		`--bc-accent:${t.accent}`,
		`--bc-accent2:${t.accent2}`,
		`--bc-ink:${t.ink}`,
		`--bc-ink-muted:${t.inkMuted}`,
		`--bc-panel:${t.panel}`,
		`--bc-panel-strong:${t.panelStrong}`,
		`--bc-edge:${t.edge}`,
		`--bc-glow:${t.glow}`,
		`--bc-radius:${t.radius}`,
		`--bc-header-grad:${t.headerGrad}`,
		`--bc-font:${t.font}`,
		`--bc-tracking:${t.tracking}`
	].join(';');
}
