<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { TvIcon, CopyIcon, RefreshCwIcon, LoaderIcon, MonitorIcon } from '@lucide/svelte';
	import { toaster } from '$lib/stores/toaster';
	import { canManageOverlays, listConsoles, type OverlayConsole } from '$lib/utils/overlay-api';

	// Operator page: overlays target by CONSOLE NAME alone. No instance/container,
	// no token — the overlay resolves the console name to whichever host currently
	// sees it and re-resolves live. (Auth on the feed is deferred per the PoC.)
	const authorized = canManageOverlays();
	const origin = $derived(browser ? window.location.origin : '');

	let consoles = $state<OverlayConsole[]>([]);
	let loading = $state(true);
	let names = $state(''); // optional display-name overrides, applied to every URL

	// The overlay surfaces + their suggested OBS Browser Source sizes.
	const OVERLAYS = [
		{ id: 'overlay', label: 'POV', path: '/overlay/', size: '1440 × 1080' },
		{ id: 'scorebug', label: 'Scorebug', path: '/scorebug/', size: '~700 × 90' },
		{ id: 'leaderboard', label: 'Leaderboard', path: '/leaderboard/', size: '340 × rows' },
		{ id: 'postgame', label: 'Postgame', path: '/postgame/', size: '900 × rows' }
	];

	function url(console: string, path: string): string {
		const parts = [`console=${encodeURIComponent(console)}`];
		if (names.trim()) parts.push(`names=${encodeURIComponent(names.trim())}`);
		return `${origin}${path}?${parts.join('&')}`;
	}

	async function copy(u: string) {
		try {
			await navigator.clipboard.writeText(u);
			toaster.success({ title: 'Copied', description: 'Browser-source URL on the clipboard.' });
		} catch {
			toaster.error({ title: 'Copy failed', description: u });
		}
	}

	async function refresh() {
		loading = true;
		consoles = await listConsoles();
		loading = false;
	}

	onMount(refresh);
</script>

<div class="mx-auto flex w-full max-w-4xl flex-col gap-5 p-4 sm:p-6">
	<header class="flex items-center gap-2">
		<TvIcon class="size-5" />
		<h1 class="h4">OBS overlays (Studio)</h1>
		<button
			class="ml-auto btn preset-tonal btn-sm"
			onclick={refresh}
			disabled={loading}
			aria-label="Refresh consoles"
		>
			{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
					class="size-4"
				/>{/if}
			<span>Refresh</span>
		</button>
	</header>

	{#if !authorized}
		<div class="card preset-tonal p-6 text-sm">
			You don't have permission to view the OBS sources. Ask an admin for the
			<code>overlay_manager</code> role.
		</div>
	{:else}
		<ol class="flex list-decimal flex-col gap-1 card preset-tonal p-4 pl-8 text-sm">
			<li>
				Pick a <strong>console</strong> below — the overlay finds whichever host is showing it.
			</li>
			<li>
				Copy the overlay's URL and add it in OBS as a <strong>Browser Source</strong> at the size shown.
			</li>
			<li>
				Every overlay renders <strong>transparent</strong> and targets by console name — it survives the
				box being recreated.
			</li>
		</ol>

		<label class="label max-w-md">
			<span class="label-text text-xs">Name overrides (optional, applied to all)</span>
			<input class="input font-mono text-xs" bind:value={names} placeholder="SCRAPED:Display,…" />
		</label>

		{#if loading}
			<div class="flex items-center gap-2 p-4 text-sm text-surface-600-400">
				<LoaderIcon class="size-4 animate-spin" /> Loading live consoles…
			</div>
		{:else if consoles.length === 0}
			<div class="card preset-tonal p-6 text-sm text-surface-600-400">
				No live consoles right now. Start a game on a scraped host (or check its System Link lobby)
				and hit Refresh.
			</div>
		{:else}
			<div class="flex flex-col gap-3">
				{#each consoles as c (c.console)}
					<div class="flex flex-col gap-2 card preset-tonal p-4">
						<div class="flex items-center gap-2">
							<MonitorIcon class="size-4 opacity-70" />
							<span class="text-base font-semibold">{c.console}</span>
							{#if c.is_local}
								<span class="chip preset-filled-primary-500 text-xs">host</span>
							{:else}
								<span class="chip preset-tonal text-xs">lobby</span>
							{/if}
							<span class="ml-auto font-mono text-xs text-surface-500">on {c.instance}</span>
						</div>
						<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
							{#each OVERLAYS as o (o.id)}
								<button
									class="btn flex-col items-start gap-0 preset-tonal py-1.5 btn-sm"
									title={url(c.console, o.path)}
									onclick={() => copy(url(c.console, o.path))}
								>
									<span class="flex w-full items-center gap-1">
										<CopyIcon class="size-3.5" /><span class="font-medium">{o.label}</span>
									</span>
									<span class="text-[10px] text-surface-500">{o.size}</span>
								</button>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		{/if}

		<p class="max-w-prose text-xs text-surface-600-400">
			Overlays read cartographer's live feed by console name (no token — auth is deferred for now).
			Fonts currently load from Google Fonts; self-host for a fully offline LAN.
		</p>
	{/if}
</div>
