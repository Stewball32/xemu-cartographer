<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { ActivityIcon } from '@lucide/svelte';
	import { adminGet, AdminFetchError } from '$lib/utils/admin-api';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { toaster } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/chrome/PageHeader.svelte';
	import Card from '$lib/components/chrome/Card.svelte';
	import type { ContainerInfo, ContainerDetail, InstanceState } from '$lib/types/containers';
	import type { ScraperInfo } from '$lib/types/scraper';

	let containers = $state<ContainerInfo[]>([]);
	let scrapers = $state<Record<string, InstanceState | null>>({});
	let orphans = $state<ScraperInfo[]>([]);
	let loading = $state(true);
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	function describeError(err: unknown): string {
		if (err instanceof AdminFetchError) return err.message;
		if (err instanceof Error) return err.message;
		return String(err);
	}

	async function loadAll() {
		try {
			loading = true;
			const list = await adminGet<ContainerInfo[] | null>('containers');
			containers = list ?? [];
			await refreshAll();
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeError(err) });
		} finally {
			loading = false;
		}
	}

	async function refreshOne(name: string) {
		try {
			const res = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
			scrapers = { ...scrapers, [name]: res.scraper };
		} catch (err) {
			console.warn('detail fetch failed for', name, err);
		}
	}

	async function refreshOrphans() {
		try {
			const list = await adminGet<ScraperInfo[] | null>('scraper');
			const knownNames = new Set(containers.map((c) => c.name));
			orphans = (list ?? []).filter((s) => !knownNames.has(s.name));
		} catch (err) {
			console.warn('scraper list fetch failed', err);
		}
	}

	async function refreshAll() {
		await Promise.all([...containers.map((c) => refreshOne(c.name)), refreshOrphans()]);
	}

	function startPolling() {
		stopPolling();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			refreshAll();
		}, 3000);
	}

	function stopPolling() {
		if (pollTimer !== null) {
			clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	onMount(() => {
		loadAll();
		startPolling();
		// Connecting here gives every row live phase + score from the
		// host:all aggregator (rendered alongside HTTP-derived state below).
		// The store's onopen joins host:all itself.
		if (auth.token) scraperWS.connect(auth.token);
	});

	onDestroy(() => {
		stopPolling();
		scraperWS.disconnect();
	});

	const phaseBadgeClass: Record<string, string> = {
		idle: 'preset-tonal',
		ready: 'preset-filled-warning-500',
		live: 'preset-filled-success-500'
	};
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6">
	<PageHeader
		title="Scraper debug"
		description="Pick a container to inspect every scraped field — the live game data, the latest tick, and recent events. Useful for verifying a freshly-added field is populated and for chasing offset drift."
	>
		{#snippet actions()}
			<ActivityIcon class="size-6" />
		{/snippet}
	</PageHeader>

	{#if loading && containers.length === 0 && orphans.length === 0}
		<div class="h-40 placeholder animate-pulse"></div>
	{:else if containers.length === 0 && orphans.length === 0}
		<Card size="lg" class="text-center">
			No containers yet. Provision one from the
			<a class="anchor" href={resolve('/containers/')}>Containers</a> page.
		</Card>
	{:else}
		<Card size="flush" class="overflow-hidden">
			<table class="table-hover w-full text-sm">
				<thead class="bg-surface-200-800">
					<tr>
						<th class="px-4 py-2 text-left">Name</th>
						<th class="px-4 py-2 text-left">Source</th>
						<th class="px-4 py-2 text-left">Scraper</th>
						<th class="px-4 py-2 text-left">Phase</th>
						<th class="px-4 py-2 text-left">Game</th>
						<th class="px-4 py-2 text-left">Xbox name</th>
						<th class="px-4 py-2 text-right">Action</th>
					</tr>
				</thead>
				<tbody>
					{#each containers as c (c.name)}
						{@const sc = scrapers[c.name]}
						{@const summary = scraperWS.hostSummaries[c.name]}
						<tr class="border-t border-surface-300-700">
							<td class="px-4 py-2 font-medium">{c.name}</td>
							<td class="px-4 py-2">
								<span class="badge preset-tonal text-xs">Container</span>
							</td>
							<td class="px-4 py-2">
								{#if sc?.running}
									<span class="badge preset-filled-success-500 text-xs">Running</span>
								{:else}
									<span class="badge preset-tonal text-xs">Idle</span>
								{/if}
							</td>
							<td class="px-4 py-2">
								{#if summary}
									<span class="badge {phaseBadgeClass[summary.phase]} text-[10px] uppercase">
										{summary.phase}
									</span>
								{:else}
									—
								{/if}
							</td>
							<td class="px-4 py-2">
								{summary?.title || sc?.title || '—'}
								{#if summary?.score_summary}
									<span class="text-surface-500-400 ml-1 font-mono text-xs"
										>· {summary.score_summary}</span
									>
								{/if}
							</td>
							<td class="px-4 py-2">{sc?.xbox_name || '—'}</td>
							<td class="px-4 py-2 text-right">
								<a
									class="btn preset-filled-primary-500 text-xs"
									href={resolve(`/admin/debug/${c.name}/`)}
								>
									Open
								</a>
							</td>
						</tr>
					{/each}
					{#each orphans as o (o.name)}
						{@const summary = scraperWS.hostSummaries[o.name]}
						<tr class="border-t border-surface-300-700">
							<td class="px-4 py-2 font-medium">{o.name}</td>
							<td class="px-4 py-2">
								<span class="badge preset-tonal-warning text-xs" title={o.sock}>External QMP</span>
							</td>
							<td class="px-4 py-2">
								<span class="badge preset-filled-success-500 text-xs">Running</span>
							</td>
							<td class="px-4 py-2">
								{#if summary}
									<span class="badge {phaseBadgeClass[summary.phase]} text-[10px] uppercase">
										{summary.phase}
									</span>
								{:else}
									—
								{/if}
							</td>
							<td class="px-4 py-2">
								{summary?.title || o.title || '—'}
								{#if summary?.score_summary}
									<span class="text-surface-500-400 ml-1 font-mono text-xs"
										>· {summary.score_summary}</span
									>
								{/if}
							</td>
							<td class="px-4 py-2">{o.xbox_name || '—'}</td>
							<td class="px-4 py-2 text-right">
								<a
									class="btn preset-filled-primary-500 text-xs"
									href={resolve(`/admin/debug/${o.name}/`)}
								>
									Open
								</a>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</Card>
	{/if}
</div>
