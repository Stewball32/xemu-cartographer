<script lang="ts">
	import { browser } from '$app/environment';
	import { TvIcon, CopyIcon, LoaderIcon } from '@lucide/svelte';
	import { toaster } from '$lib/stores/toaster';
	import { canManageOverlays, mintOverlayToken } from '$lib/utils/overlay-api';

	// Who may view the OBS catalog + mint the overlay's token — admin/superuser
	// or the overlay_manager role, matching the previous overlay area.
	const authorized = canManageOverlays();
	const origin = $derived(browser ? window.location.origin : '');

	// Shared params for the ?ws sources (scorebug/leaderboard/postgame).
	let ws = $state('ws://localhost:8765');
	let names = $state('');

	// The POV overlay is cartographer-native: it subscribes to ONE instance's
	// live feed (game+tick) and AUTO-detects splitscreen (1/2/4) itself. It needs
	// the instance name + a scoped overlay token — minted here.
	let instance = $state('');
	let minting = $state(false);
	let overlayToken = $state('');
	let mintedInstance = $state('');

	async function mintOverlay() {
		const inst = instance.trim();
		if (!inst) {
			toaster.error({
				title: 'Instance required',
				description: 'Enter the container/instance name.'
			});
			return;
		}
		minting = true;
		try {
			const res = await mintOverlayToken(`host:${inst}`, 'POV overlay');
			overlayToken = res.token;
			mintedInstance = inst;
			toaster.success({ title: 'Overlay token minted', description: `host:${inst}` });
		} catch (e) {
			toaster.error({ title: 'Mint failed', description: String(e) });
		} finally {
			minting = false;
		}
	}

	function overlayURL(): string {
		const parts: string[] = [];
		if (mintedInstance) parts.push(`instance=${encodeURIComponent(mintedInstance)}`);
		if (overlayToken) parts.push(`token=${encodeURIComponent(overlayToken)}`);
		if (names.trim()) parts.push(`names=${encodeURIComponent(names.trim())}`);
		return `${origin}/overlay/${parts.length ? `?${parts.join('&')}` : ''}`;
	}

	// The three ?ws sources — each sized to its own content (no forced canvas).
	type WsSource = { id: string; title: string; path: string; size: string; blurb: string };
	const WS_SOURCES: WsSource[] = [
		{
			id: 'scorebug',
			title: 'Scorebug',
			path: '/scorebug/',
			size: '~700 × 90 · fit to content',
			blurb:
				'Match bug: team · clock/gametype/map · team (FFA duel at exactly 2 players). Sized to its own content — no padding.'
		},
		{
			id: 'leaderboard',
			title: 'Leaderboard',
			path: '/leaderboard/',
			size: '340 × (72 + 52·rows) · fit to content',
			blurb: 'Live standings with KDA, spree, shield/health bars; rows FLIP-animate on rank change.'
		},
		{
			id: 'postgame',
			title: 'Postgame',
			path: '/postgame/',
			size: '900 × (150 + 46·rows) · fit to content',
			blurb:
				'Full carnage report: winner banner, per-player ledger, team aggregates, footer totals.'
		}
	];

	function wsURL(path: string): string {
		const parts: string[] = [];
		if (ws.trim()) parts.push(`ws=${encodeURIComponent(ws.trim())}`);
		if (names.trim()) parts.push(`names=${encodeURIComponent(names.trim())}`);
		return `${origin}${path}${parts.length ? `?${parts.join('&')}` : ''}`;
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
			shown — every route renders transparent. Only the <strong>POV overlay</strong> is a full player-view
			size; the rest are sized to their own content.
		</p>

		<!-- POV overlay — cartographer-native, auto-splitscreen -->
		<div class="flex flex-col gap-2 card preset-tonal p-4">
			<div class="flex items-center gap-2">
				<h2 class="h6">POV overlay</h2>
				<span class="preset-filled-primary chip text-xs">full player view · 4:3</span>
				<span class="ml-auto font-mono text-xs text-surface-600-400">1440 × 1080 · transparent</span
				>
			</div>
			<p class="text-xs text-surface-600-400">
				Drop in ONE source. It subscribes to the instance's live feed and <strong
					>auto-detects splitscreen</strong
				> (1 / 2 / 4 players) — no per-player sources, no OBS cropping, no manual layout. Re-lays-out
				live if the split changes. Pick the machine + mint its overlay token:
			</p>
			<div class="flex flex-col gap-2 sm:flex-row sm:items-end">
				<label class="label flex-1">
					<span class="label-text text-xs">Container / instance</span>
					<input class="input text-xs" placeholder="e.g. pod-a" bind:value={instance} />
				</label>
				<button class="preset-filled-primary btn" disabled={minting} onclick={mintOverlay}>
					{#if minting}<LoaderIcon class="size-4 animate-spin" />{/if}
					<span>Mint token</span>
				</button>
			</div>
			<div class="flex items-center gap-2">
				<input class="input flex-1 font-mono text-xs" readonly value={overlayURL()} />
				<button
					class="btn-icon preset-tonal"
					aria-label="Copy URL"
					disabled={!overlayToken}
					onclick={() => copy(overlayURL())}
				>
					<CopyIcon class="size-4" />
				</button>
			</div>
			<span class="text-xs text-surface-600-400">
				The token isn't shown again — copy the URL now. No live game? Append <code>?mock=1</code> to preview
				the auto-split with sample data (no token needed).
			</span>
		</div>

		<!-- Shared ?ws params for the content-sized sources -->
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

		<div class="grid gap-3 lg:grid-cols-3">
			{#each WS_SOURCES as s (s.id)}
				<div class="flex flex-col gap-2 card preset-tonal p-4">
					<div class="flex items-center gap-2">
						<h2 class="h6">{s.title}</h2>
						<span class="ml-auto font-mono text-xs text-surface-600-400">{s.size}</span>
					</div>
					<p class="text-xs text-surface-600-400">{s.blurb}</p>
					<div class="flex items-center gap-2">
						<input class="input flex-1 font-mono text-xs" readonly value={wsURL(s.path)} />
						<button
							class="btn-icon preset-tonal"
							aria-label="Copy URL"
							onclick={() => copy(wsURL(s.path))}
						>
							<CopyIcon class="size-4" />
						</button>
					</div>
				</div>
			{/each}
		</div>

		<p class="max-w-prose text-xs text-surface-600-400">
			The POV overlay reads cartographer's live feed directly; the content-sized sources subscribe
			to the <code>?ws=</code> feed (see the pack README's WebSocket contract). Fonts load from Google
			Fonts; self-host for a fully offline LAN.
		</p>
	{/if}
</div>
