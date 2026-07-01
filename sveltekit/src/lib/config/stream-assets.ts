// Stream-assets registry — the single source of truth for the /studio/ hub.
//
// Each entry maps one browser-source stream asset (an OBS overlay or a full
// spectator scene) to its route, preview behaviour, and aspect. The hub reads
// this list to render the gallery: a live ?mock=1 preview + a one-click "copy
// OBS URL" (scoped overlay token) per asset.
//
// ADDING AN ASSET = a route + an entry here, nothing else:
//   1. Build the render page. Transparent OBS overlays live under
//      /overlays/[instance]/<x>/ so they inherit the chrome-hiding +
//      transparent-background treatment (the /overlays/ prefix is in
//      HIDDEN_LAYOUT_PATHS). Drive it from createOverlayFeed so ?mock=1 works.
//   2. Append a StreamAsset below. The hub picks it up automatically.
// See docs/STREAM-ASSETS.md.

import {
	ClockIcon,
	GaugeIcon,
	LayoutListIcon,
	MapIcon,
	BoxIcon,
	SkullIcon,
	UsersIcon
} from '@lucide/svelte';
import type { Component } from 'svelte';

/** OBS source kind. `overlay` = transparent, composited over the game capture;
 * `scene` = an opaque full-frame spectator surface (its own background). */
export type AssetKind = 'overlay' | 'scene';

export interface AssetAspect {
	w: number;
	h: number;
	/** Human-readable badge, e.g. "1920×1080 · 16:9". */
	label: string;
}

export interface StreamAsset {
	id: string;
	name: string;
	description: string;
	icon: Component;
	kind: AssetKind;
	/** Route path (no origin, no query) for an instance. Always trailing-slash. */
	path: (instance: string) => string;
	/** Whether ?mock=1 renders a meaningful, game-free preview. */
	preview: boolean;
	aspect: AssetAspect;
	/** Short OBS placement/sizing hint shown on the card. */
	obsHint: string;
	/** Whether the live OBS URL carries a scoped overlay token (the default).
	 * `false` for legacy surfaces that authenticate with a user JWT instead — the
	 * hub then offers a plain URL and skips the mint requirement. */
	tokenScoped?: boolean;
	/** Optional per-asset note (e.g. live-verification caveats). */
	note?: string;
	/** Extra query appended to BOTH the mock-preview and live OBS URLs (no leading
	 * `?`/`&`), e.g. `game=h2` to select a themed variant. */
	query?: string;
}

/** Standard 1080p browser source — every asset today is a 16:9 source (overlays
 * anchor within the canvas; scenes fill it). Kept as a shared constant so the
 * common case is one token and a future odd-sized asset just sets its own. */
const SOURCE_1080P: AssetAspect = { w: 16, h: 9, label: '1920×1080 · 16:9' };

/** Instance name used for every mock preview — matches overlay-mock's
 * MOCK_INSTANCE. Any name works in mock mode (the URL instance is ignored). */
export const MOCK_INSTANCE = 'demo';

export const STREAM_ASSETS: StreamAsset[] = [
	{
		id: 'broadcast-scoreboard',
		name: 'Broadcast scoreboard (CE)',
		description:
			'The hero scoreboard — themed header (gametype · map · match clock · score-to) over team columns or an FFA list, each row an armor-chipped player with K/D/A, score and vitals. Halo: CE visual language.',
		icon: LayoutListIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/scoreboard/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-center'
	},
	{
		id: 'broadcast-cards',
		name: 'Player cards (CE)',
		description:
			'A bottom strip of per-player cards — a Spartan tinted by the player’s armor colour, gamertag, K/D/A and a big score, clustered by team. Halo: CE visual language.',
		icon: UsersIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/cards/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor bottom-center'
	},
	{
		id: 'broadcast-scoreboard-h2',
		name: 'Broadcast scoreboard (H2 theme)',
		description:
			'The same scoreboard in Halo 2’s cool-blue menu language. Preview only for now — the H2 live scrape is still in progress.',
		icon: LayoutListIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/scoreboard/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-center',
		query: 'game=h2',
		note: 'Halo 2 theme. Live H2 data (roster / scores / emblems) awaits the H2 scraper — mock preview only today.'
	},
	{
		id: 'broadcast-cards-h2',
		name: 'Player cards (H2 theme)',
		description:
			'Player cards in Halo 2’s visual language — each Spartan (or Elite) carries the player’s emblem on the chest plus a corner badge. Preview only for now.',
		icon: UsersIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/cards/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor bottom-center',
		query: 'game=h2',
		note: 'Halo 2 theme. Live H2 data + emblems await the H2 scraper — mock preview only today.'
	},
	{
		id: 'scoreboard',
		name: 'Scoreboard (compact)',
		description:
			'Full roster grouped by team — names, alive/dead, health + shield bars, K/D/A and team scores. The original compact overlay.',
		icon: LayoutListIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-left'
	},
	{
		id: 'status-strip',
		name: 'Match-status strip',
		description:
			'Compact top bar: phase (LIVE/SETUP/IDLE), team scores, gametype/variant, map, and score limit.',
		icon: GaugeIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/status/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-center'
	},
	{
		id: 'timer',
		name: 'Game timer',
		description:
			'A match clock — elapsed time from the engine tick, or counting down to the time limit when the gametype sets one. Plus gametype + score-to.',
		icon: ClockIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/timer/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-center'
	},
	{
		id: 'killfeed',
		name: 'Kill feed',
		description:
			'A rolling stream of recent kills — killer ✕ victim with the weapon, betrayals flagged, suicides and falls called out. Driven by the live event feed.',
		icon: SkullIcon,
		kind: 'overlay',
		path: (i) => `/overlays/${i}/killfeed/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-right'
	},
	{
		id: 'players',
		name: 'Players (split-screen)',
		description:
			'Per-player cards for the local (split-screen) seats on one console — weapon, ammo, powerups, grenades. Legacy surface; uses a user JWT.',
		icon: UsersIcon,
		kind: 'overlay',
		path: () => `/overlays/players/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Transparent · anchor top-left',
		tokenScoped: false,
		note: 'Predates the overlay token — authenticates with a user JWT, not a scoped overlay token. Superseded by the scoreboard for the OBS path.'
	},
	{
		id: 'visualizer-2d',
		name: '2D visualizer',
		description:
			'Top-down live map — player dots with health/shield rings, items, vehicles, projectiles, spawns. A spectator/observer scene.',
		icon: MapIcon,
		kind: 'scene',
		path: (i) => `/visualizer/${i}/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Opaque scene · full 1920×1080'
	},
	{
		id: 'visualizer-3d',
		name: '3D visualizer',
		description:
			'Live markers over the real map BSP mesh (Blood Gulch today). A 3D spectator scene fed by the same WS feed.',
		icon: BoxIcon,
		kind: 'scene',
		path: (i) => `/visualizer3d/${i}/`,
		preview: true,
		aspect: SOURCE_1080P,
		obsHint: 'Opaque scene · full 1920×1080'
	}
];

export function assetById(id: string): StreamAsset | undefined {
	return STREAM_ASSETS.find((a) => a.id === id);
}

/** Mock-preview path for an asset (relative — same-origin). Renders without a
 * game or token via ?mock=1. Appends the asset's `query` (e.g. game=h2). */
export function mockPath(asset: StreamAsset, scale?: number): string {
	const base = asset.path(MOCK_INSTANCE);
	const s = scale && scale !== 1 ? `&scale=${scale}` : '';
	const extra = asset.query ? `&${asset.query}` : '';
	return `${base}?mock=1${s}${extra}`;
}

/** Absolute live OBS URL for an asset. Token-scoped assets (the default) carry
 * the scoped overlay token; legacy JWT surfaces get a plain URL. Appends the
 * asset's `query` (e.g. game=h2). */
export function liveURL(
	origin: string,
	asset: StreamAsset,
	instance: string,
	token: string
): string {
	const path = asset.path(instance);
	if (asset.tokenScoped === false) {
		return asset.query ? `${origin}${path}?${asset.query}` : `${origin}${path}`;
	}
	const extra = asset.query ? `&${asset.query}` : '';
	return `${origin}${path}?token=${encodeURIComponent(token)}${extra}`;
}

/** Absolute mock URL for an asset (for "open preview in new tab"). */
export function mockURL(origin: string, asset: StreamAsset): string {
	return `${origin}${mockPath(asset)}`;
}
