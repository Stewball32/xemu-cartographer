<script lang="ts">
	import { browser } from '$app/environment';
	import { TvIcon, CopyIcon } from '@lucide/svelte';
	import { toaster } from '$lib/stores/toaster';
	import { canManageOverlays } from '$lib/utils/overlay-api';

	// Who may view the OBS catalog. The sources themselves are token-free
	// (?ws=…), so this only gates who SEES the copy-able URLs — admin/superuser or
	// the overlay_manager role, matching the previous overlay area.
	const authorized = canManageOverlays();
	const origin = $derived(browser ? window.location.origin : '');

	// Shared params applied to every source URL.
	//   ws    — the cartographer state feed the overlays subscribe to.
	//   names — optional SCRAPED:Display,… display-name overrides.
	let ws = $state('ws://localhost:8765');
	let names = $state('');
	// /overlay only: which POV layout to composite (1 fullscreen … 4 quad).
	let layout = $state(1);

	// The new OBS browser-source pack (LAN_OBS_Browser_Sources). Exactly one is a
	// true full player-view overlay (/overlay, transparent, sized to a player's
	// view); the other three are sized to their OWN content — no canvas padding.
	type Source = {
		id: string;
		title: string;
		path: string; // route path (trailingSlash always → ends with /)
		size: string; // native OBS source size (do NOT force a fixed canvas)
		overlay: boolean; // the full player-view overlay
		hasLayout: boolean;
		blurb: string;
	};

	const SOURCES: Source[] = [
		{
			id: 'overlay',
			title: 'POV overlay',
			path: '/overlay/',
			size: '1440 × 1080 · transparent',
			overlay: true,
			hasLayout: true,
			blurb:
				'The full player-view overlay: per-xbox POV stat bars + respawn rings composited over the game feed. Size the browser source to the player view; pick the layout below.'
		},
		{
			id: 'scorebug',
			title: 'Scorebug',
			path: '/scorebug/',
			size: '~700 × 90 · fit to content',
			overlay: false,
			hasLayout: false,
			blurb:
				'Match bug: team · clock/gametype/map · team (FFA duel at exactly 2 players, bare clock otherwise). Sized to its own content — no empty padding.'
		},
		{
			id: 'leaderboard',
			title: 'Leaderboard',
			path: '/leaderboard/',
			size: '340 × (72 + 52·rows) · fit to content',
			overlay: false,
			hasLayout: false,
			blurb:
				'Live standings (FFA flat or team-grouped) with KDA, spree, shield/health bars; rows FLIP-animate on rank change. Height grows with the player count.'
		},
		{
			id: 'postgame',
			title: 'Postgame',
			path: '/postgame/',
			size: '900 × (150 + 46·rows) · fit to content',
			overlay: false,
			hasLayout: false,
			blurb:
				'Full carnage report: winner banner, per-player ledger, best-of-stat highlights, team aggregates, footer totals; rows cascade in on scene switch.'
		}
	];

	function urlFor(s: Source): string {
		const parts: string[] = [];
		if (ws.trim()) parts.push(`ws=${encodeURIComponent(ws.trim())}`);
		if (s.hasLayout) parts.push(`layout=${layout}`);
		if (names.trim()) parts.push(`names=${encodeURIComponent(names.trim())}`);
		return `${origin}${s.path}${parts.length ? `?${parts.join('&')}` : ''}`;
	}

	async function copy(url: string) {
		try {
			await navigator.clipboard.writeText(url);
			toaster.success({ title: 'Copied', description: 'Browser-source URL on the clipboard.' });
		} catch {
			toaster.error({ title: 'Copy failed', description: url });
		}
	}
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<TvIcon class="size-5" />
		<h1 class="h4">OBS browser sources</h1>
	</header>

	{#if !authorized}
		<div class="card preset-tonal p-6 text-sm">
			You don't have permission to view the OBS sources. Ask an admin for the
			<code>overlay_manager</code> role.
		</div>
	{:else}
		<p class="max-w-prose text-sm text-surface-600-400">
			The Norcal Halo overlay pack. Add each as an OBS <strong>Browser Source</strong> at the size
			shown — every route renders on a transparent background. Only the <strong>POV overlay</strong> is
			a full player-view size; the rest are sized to their own content, so don't stretch them to a 1920×1080
			canvas.
		</p>

		<!-- Shared feed / display params -->
		<div class="grid gap-3 card preset-tonal p-4 sm:grid-cols-2">
			<label class="label">
				<span class="label-text">Feed WebSocket (<code>?ws=</code>)</span>
				<input class="input font-mono text-xs" bind:value={ws} placeholder="ws://localhost:8765" />
			</label>
			<label class="label">
				<span class="label-text">Name overrides (<code>?names=</code>, optional)</span>
				<input class="input font-mono text-xs" bind:value={names} placeholder="SCRAPED:Display,…" />
			</label>
		</div>

		<!-- Source catalog -->
		<div class="grid gap-3 lg:grid-cols-2">
			{#each SOURCES as s (s.id)}
				<div class="flex flex-col gap-2 card preset-tonal p-4">
					<div class="flex items-center gap-2">
						<h2 class="h6">{s.title}</h2>
						{#if s.overlay}
							<span class="preset-filled-primary chip text-xs">full player view</span>
						{/if}
						<span class="ml-auto font-mono text-xs text-surface-600-400">{s.size}</span>
					</div>
					<p class="text-xs text-surface-600-400">{s.blurb}</p>

					{#if s.hasLayout}
						<label class="label">
							<span class="label-text text-xs">Layout</span>
							<select class="select text-xs" bind:value={layout}>
								<option value={1}>1 — fullscreen</option>
								<option value={2}>2 — horizontal split</option>
								<option value={3}>3 — top-full + quads</option>
								<option value={4}>4 — quad</option>
							</select>
						</label>
					{/if}

					<div class="flex items-center gap-2">
						<input class="input flex-1 font-mono text-xs" readonly value={urlFor(s)} />
						<button
							class="btn-icon preset-tonal"
							aria-label="Copy URL"
							onclick={() => copy(urlFor(s))}
						>
							<CopyIcon class="size-4" />
						</button>
					</div>
				</div>
			{/each}
		</div>

		<p class="max-w-prose text-xs text-surface-600-400">
			The overlays subscribe to the <code>?ws=</code> feed for live match/player state (see the pack README's
			WebSocket contract). Fonts (Ultra + Inter) load from Google Fonts; self-host them for a fully offline
			LAN.
		</p>
	{/if}
</div>
