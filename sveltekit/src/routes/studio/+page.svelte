<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { TvIcon, CopyIcon, LoaderIcon, KeyRoundIcon } from '@lucide/svelte';
	import { toaster } from '$lib/stores/toaster';
	import {
		canManageOverlays,
		listOverlayInstances,
		mintOverlayToken,
		type OverlayInstance
	} from '$lib/utils/overlay-api';

	// Who may view the OBS catalog + mint the overlay token — admin/superuser or
	// the overlay_manager role.
	const authorized = canManageOverlays();
	const origin = $derived(browser ? window.location.origin : '');

	// All four overlays are now cartographer-native: each subscribes to ONE
	// instance's live feed (game/tick/scenario) with a scoped, read-only overlay
	// token. ONE minted token serves every overlay for that instance.
	// `instance` is the CONTAINER id the overlay targets; the picker lets the
	// operator choose by friendly console name (xbox_name) instead of typing it.
	let instance = $state('');
	let names = $state('');
	let minting = $state(false);
	let token = $state('');
	let mintedInstance = $state('');

	// Live instances for the picker. Empty (or a non-admin caller) → fall back to
	// typing the container id manually.
	let instances = $state<OverlayInstance[]>([]);
	let manual = $state(false);
	const instanceLabel = (i: OverlayInstance) =>
		i.xbox_name && i.xbox_name !== i.name ? `${i.xbox_name}  (${i.name})` : i.name;

	onMount(() => {
		void listOverlayInstances().then((rows) => {
			instances = rows;
			if (rows.length === 0) manual = true;
			else if (rows.length === 1) instance = rows[0].name; // one console → preselect
		});
	});

	async function mint() {
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
			const res = await mintOverlayToken(`host:${inst}`, 'OBS overlays');
			token = res.token;
			mintedInstance = inst;
			toaster.success({ title: 'Overlay token minted', description: `host:${inst}` });
		} catch (e) {
			toaster.error({ title: 'Mint failed', description: String(e) });
		} finally {
			minting = false;
		}
	}

	type Source = { id: string; title: string; path: string; size: string; blurb: string };
	const SOURCES: Source[] = [
		{
			id: 'overlay',
			title: 'POV overlay',
			path: '/overlay/',
			size: '1440 × 1080',
			blurb:
				'Per-player POV bars + respawn rings over the game feed. Auto-detects splitscreen (1 / 2 / 4) — one source, no cropping.'
		},
		{
			id: 'scorebug',
			title: 'Scorebug',
			path: '/scorebug/',
			size: '~700 × 90 · fit to content',
			blurb: 'Match bug: team · gametype/map · team (FFA duel at exactly 2 players).'
		},
		{
			id: 'leaderboard',
			title: 'Leaderboard',
			path: '/leaderboard/',
			size: '340 × (72 + 52·rows) · fit to content',
			blurb: 'Live standings with KDA + spree; rows FLIP-animate on rank change.'
		},
		{
			id: 'postgame',
			title: 'Postgame',
			path: '/postgame/',
			size: '900 × (150 + 46·rows) · fit to content',
			blurb: 'Full carnage report: winner banner, per-player ledger, team aggregates, totals.'
		}
	];

	/** Build a source's OBS URL for the minted instance/token (+ optional name
	 * overrides). Append ?mock=1 (shown separately) to preview without a token. */
	function sourceURL(path: string): string {
		const parts: string[] = [];
		if (mintedInstance) parts.push(`instance=${encodeURIComponent(mintedInstance)}`);
		if (token) parts.push(`token=${encodeURIComponent(token)}`);
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

<div class="mx-auto flex w-full max-w-4xl flex-col gap-5 p-4 sm:p-6">
	<header class="flex items-center gap-2">
		<TvIcon class="size-5" />
		<h1 class="h4">OBS overlays (Studio)</h1>
	</header>

	{#if !authorized}
		<div class="card preset-tonal p-6 text-sm">
			You don't have permission to view the OBS sources. Ask an admin for the
			<code>overlay_manager</code> role.
		</div>
	{:else}
		<!-- How-to -->
		<ol class="flex list-decimal flex-col gap-1 card preset-tonal p-4 pl-8 text-sm">
			<li>Pick the live console and <strong>mint an overlay token</strong>.</li>
			<li>
				Copy each overlay's URL and add it in OBS as a <strong>Browser Source</strong> at the size shown.
			</li>
			<li>
				All overlays render on a <strong>transparent</strong> background — they composite straight over
				the game capture.
			</li>
		</ol>

		<!-- Step 1: instance + token -->
		<div class="flex flex-col gap-3 card preset-tonal p-4">
			<div class="flex flex-col gap-2 sm:flex-row sm:items-end">
				<label class="label flex-1">
					<span class="label-text flex items-center justify-between text-xs">
						<span>Console / instance</span>
						{#if instances.length}
							<button type="button" class="anchor text-[11px]" onclick={() => (manual = !manual)}>
								{manual ? 'Pick from live list' : 'Type manually'}
							</button>
						{/if}
					</span>
					{#if instances.length && !manual}
						<select class="select text-sm" bind:value={instance}>
							<option value="" disabled>Select a live console…</option>
							{#each instances as i (i.name)}
								<option value={i.name}>{instanceLabel(i)}</option>
							{/each}
						</select>
					{:else}
						<input
							class="input text-sm"
							placeholder="container id, e.g. beta-stream"
							bind:value={instance}
						/>
					{/if}
				</label>
				<label class="label flex-1">
					<span class="label-text text-xs">Name overrides (optional)</span>
					<input
						class="input font-mono text-xs"
						bind:value={names}
						placeholder="SCRAPED:Display,…"
					/>
				</label>
				<button class="preset-filled-primary btn" disabled={minting} onclick={mint}>
					{#if minting}<LoaderIcon class="size-4 animate-spin" />{:else}<KeyRoundIcon
							class="size-4"
						/>{/if}
					<span>Mint token</span>
				</button>
			</div>
			{#if token}
				<p class="text-xs text-success-600-400">
					Token minted for <code>host:{mintedInstance}</code> — it serves all four overlays below. It
					isn't shown again, so copy the URLs now.
				</p>
			{:else}
				<p class="text-xs text-surface-600-400">
					No live game yet? Append <code>?mock=1</code> to any overlay URL to preview it with sample data
					(no token needed).
				</p>
			{/if}
		</div>

		<!-- Step 2: per-overlay OBS URLs -->
		<div class="grid gap-3 sm:grid-cols-2">
			{#each SOURCES as s (s.id)}
				<div class="flex flex-col gap-2 card preset-tonal p-4">
					<div class="flex items-baseline gap-2">
						<h2 class="h6">{s.title}</h2>
						<span class="ml-auto font-mono text-xs text-surface-600-400">{s.size}</span>
					</div>
					<p class="text-xs text-surface-600-400">{s.blurb}</p>
					<div class="flex items-center gap-2">
						<input class="input flex-1 font-mono text-xs" readonly value={sourceURL(s.path)} />
						<button
							class="btn-icon preset-tonal"
							aria-label="Copy URL"
							disabled={!token}
							onclick={() => copy(sourceURL(s.path))}
						>
							<CopyIcon class="size-4" />
						</button>
					</div>
				</div>
			{/each}
		</div>

		<p class="max-w-prose text-xs text-surface-600-400">
			All four overlays read cartographer's native live feed (the minted token authorizes read-only
			access to this instance's rooms). Fonts currently load from Google Fonts — self-host them for
			a fully offline LAN.
		</p>
	{/if}
</div>
