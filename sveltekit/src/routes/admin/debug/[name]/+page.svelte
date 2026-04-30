<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { ArrowLeftIcon } from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { adminGet } from '$lib/utils/admin-api';
	import type { ContainerDetail, InstanceState } from '$lib/types/containers';
	import JsonTree from '$lib/components/JsonTree.svelte';

	let { data } = $props();
	const name = $derived(data.name);

	let scraper = $state<InstanceState | null>(null);
	let now = $state(Date.now());

	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let nowTimer: ReturnType<typeof setInterval> | null = null;

	async function refreshDetail() {
		try {
			const res = await adminGet<ContainerDetail>(
				`containers/${encodeURIComponent(name)}/detail`
			);
			scraper = res.scraper;
		} catch (err) {
			console.warn('detail fetch failed', err);
		}
	}

	onMount(() => {
		if (auth.token) {
			scraperWS.connect(auth.token);
		}
		refreshDetail();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			refreshDetail();
		}, 3000);
		nowTimer = setInterval(() => {
			now = Date.now();
		}, 1000);
	});

	onDestroy(() => {
		scraperWS.disconnect();
		if (pollTimer) clearInterval(pollTimer);
		if (nowTimer) clearInterval(nowTimer);
	});

	function relativeTime(ts: number | undefined): string {
		if (!ts) return 'never';
		const diffMs = now - ts;
		if (diffMs < 1000) return 'just now';
		if (diffMs < 60_000) return `${Math.floor(diffMs / 1000)}s ago`;
		if (diffMs < 3_600_000) return `${Math.floor(diffMs / 60_000)}m ago`;
		return `${Math.floor(diffMs / 3_600_000)}h ago`;
	}

	const snapshot = $derived(scraperWS.snapshots[name]);
	const tick = $derived(scraperWS.ticks[name]);
	const events = $derived(scraperWS.events[name] ?? []);
	const snapshotAt = $derived(scraperWS.snapshotsAt[name]);
	const tickAt = $derived(scraperWS.ticksAt[name]);

	const latestTick = $derived(tick ? '—' : '—');
	// Pull a tick number for headers when available. Snapshots/ticks don't carry
	// the envelope's tick field directly — that's outer-envelope metadata —
	// so we approximate via "received N seconds ago".
</script>

<div class="container mx-auto max-w-5xl p-4">
	<header class="mb-6">
		<a class="anchor mb-2 flex items-center gap-1 text-sm" href={resolve('/admin/debug/')}>
			<ArrowLeftIcon class="size-4" />
			Back to debug
		</a>
		<div class="flex items-center justify-between gap-4">
			<div>
				<h1 class="h2">{name}</h1>
				<p class="text-surface-700-200 text-sm">
					{#if scraper?.running}
						<span class="badge preset-filled-success-500 mr-2 text-xs">Scraper running</span>
					{:else}
						<span class="badge preset-tonal mr-2 text-xs">Scraper idle</span>
					{/if}
					{scraper?.game_title || '—'} · {scraper?.xbox_name || '—'}
				</p>
			</div>
			<div class="text-right text-xs">
				<div>WS: <span class:text-success-500={scraperWS.connected}>{scraperWS.connected ? 'connected' : 'disconnected'}</span></div>
				{#if scraperWS.lastError}
					<div class="text-error-500">{scraperWS.lastError}</div>
				{/if}
			</div>
		</div>
	</header>

	{#if !snapshot && !tick}
		<div class="card preset-tonal p-6 text-center">
			<p class="mb-2">No live data — scraper not running for this container.</p>
			<p class="text-surface-700-200 text-sm">
				Start it from <a class="anchor" href={resolve(`/containers/${name}/`)}>/containers/{name}/</a>.
			</p>
		</div>
	{:else}
		<section class="mb-2">
			<div class="text-surface-700-200 mb-1 text-xs">
				Snapshot · received {relativeTime(snapshotAt)}
			</div>
			{#if snapshot}
				<JsonTree value={snapshot} label="Snapshot" />
			{:else}
				<div class="card preset-tonal text-surface-500-400 p-3 text-sm">no snapshot yet</div>
			{/if}
		</section>

		<section class="mb-2">
			<div class="text-surface-700-200 mb-1 text-xs">
				Latest tick · received {relativeTime(tickAt)}
			</div>
			{#if tick}
				<JsonTree value={tick} label="Latest tick" defaultOpen={false} />
			{:else}
				<div class="card preset-tonal text-surface-500-400 p-3 text-sm">
					no tick yet — only emitted while in a match
				</div>
			{/if}
		</section>

		<section class="mb-2">
			<div class="text-surface-700-200 mb-1 text-xs">
				Recent events · {events.length} buffered (newest first)
			</div>
			{#if events.length > 0}
				<JsonTree value={events} label="Recent events" defaultOpen={false} />
			{:else}
				<div class="card preset-tonal text-surface-500-400 p-3 text-sm">
					no events yet — fired sparsely on kills, pickups, grenades, etc.
				</div>
			{/if}
		</section>
	{/if}
</div>
