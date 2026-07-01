<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { ArrowLeftIcon } from '@lucide/svelte';
	import { Tabs } from '@skeletonlabs/skeleton-svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import type { EnvelopeTypeV2 } from '$lib/types/scraper-v2';
	import type { InstanceState } from '$lib/types/containers';
	import type { ScraperInspect } from '$lib/types/scraper';
	import { fieldAnnotations } from '$lib/stores/fieldAnnotations.svelte';
	import { toaster } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import LiveStatusBar, { type FeedClass } from '$lib/components/debug/LiveStatusBar.svelte';
	import TabsResponsive from '$lib/components/debug/TabsResponsive.svelte';
	import OverviewTab from '$lib/components/debug/overview/OverviewTab.svelte';
	import XboxTab from '$lib/components/debug/xbox/XboxTab.svelte';
	import ScenarioTab from '$lib/components/debug/scenario/ScenarioTab.svelte';
	import GameTab from '$lib/components/debug/game/GameTab.svelte';
	import TickTab from '$lib/components/debug/tick/TickTab.svelte';
	import ObjectsTab from '$lib/components/debug/objects/ObjectsTab.svelte';
	import DebugTab from '$lib/components/debug/debug/DebugTab.svelte';
	import PreviousGameTab from '$lib/components/debug/previous_game/PreviousGameTab.svelte';
	import EventsTab from '$lib/components/debug/events/EventsTab.svelte';
	import { setDebugContext, type ViewMode } from '$lib/components/debug/context.js';
	import { fetchDebugDetail } from '$lib/components/debug/refresh.js';

	let { data } = $props();
	const name = $derived(data.name);

	let scraper = $state<InstanceState | null>(null);
	let inspect = $state<ScraperInspect | null>(null);
	let inspectAt = $state<number | undefined>(undefined);
	let now = $state(Date.now());

	// Shared formatted/raw view preference for any tab that supports both
	// (currently xbox; scenario/game/etc. will adopt the same pattern).
	// Persisted under one page-level key so the choice carries across tabs.
	const VIEW_MODE_KEY = 'debug.view';
	let viewMode = $state<ViewMode>('pretty');
	function setViewMode(next: ViewMode) {
		viewMode = next;
		try {
			localStorage.setItem(VIEW_MODE_KEY, next);
		} catch {
			// ignore
		}
	}

	const TAB_VALUES = [
		'overview',
		'xbox',
		'scenario',
		'game',
		'tick',
		'objects',
		'debug',
		'previous_game',
		'events'
	] as const;
	type TabValue = (typeof TAB_VALUES)[number];
	const TAB_SET = new Set<string>(TAB_VALUES);

	function initialTab(): TabValue {
		if (typeof window === 'undefined') return 'overview';
		const h = window.location.hash.replace(/^#/, '').split('/')[0];
		return TAB_SET.has(h) ? (h as TabValue) : 'overview';
	}

	let topTab = $state<TabValue>(initialTab());

	function setTab(next: string) {
		if (!TAB_SET.has(next)) return;
		topTab = next as TabValue;
		if (typeof window === 'undefined') return;
		const newHash = `#${next}`;
		if (window.location.hash !== newHash) {
			history.replaceState(null, '', `${location.pathname}${location.search}${newHash}`);
		}
	}

	function relativeTime(ts: number | undefined): string {
		if (!ts) return 'never';
		const diffMs = now - ts;
		if (diffMs < 1000) return 'just now';
		if (diffMs < 60_000) return `${Math.floor(diffMs / 1000)}s ago`;
		if (diffMs < 3_600_000) return `${Math.floor(diffMs / 60_000)}m ago`;
		return `${Math.floor(diffMs / 3_600_000)}h ago`;
	}

	setDebugContext({
		get inspect() {
			return inspect;
		},
		get inspectAt() {
			return inspectAt;
		},
		get now() {
			return now;
		},
		get viewMode() {
			return viewMode;
		},
		setViewMode,
		relativeTime
	});

	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let nowTimer: ReturnType<typeof setInterval> | null = null;

	async function refreshDetail() {
		const r = await fetchDebugDetail(name, { scraper, inspect });
		scraper = r.scraper;
		inspect = r.inspect;
		if (r.inspectAt !== undefined) inspectAt = r.inspectAt;
	}

	// V2 debug page subscribes to every per-instance class room for the
	// named runner. The runner-side demand model (PR 15) skips the heavy
	// reads (ReadTick → tick/objects/debug) when no client is listening,
	// so subscribing here is what lights up the data flow for this page.
	const DEBUG_CLASSES: EnvelopeTypeV2[] = [
		'xbox',
		'scenario',
		'game',
		'tick',
		'objects',
		'debug',
		'event',
		'previous_game'
	];

	onMount(() => {
		try {
			const raw = localStorage.getItem(VIEW_MODE_KEY);
			if (raw === 'pretty' || raw === 'json') viewMode = raw;
		} catch {
			// localStorage unavailable; keep default.
		}
		if (auth.token) scraperWSV2.connect(auth.token);
		refreshDetail();
		pollTimer = setInterval(() => {
			if (document.visibilityState === 'visible') refreshDetail();
		}, 3000);
		nowTimer = setInterval(() => (now = Date.now()), 1000);
	});

	$effect(() => {
		// Subscribe regardless of connected state — subscribeInstance just
		// records intent if the socket isn't open yet and replays on
		// connect via the store's intendedRooms set (PR 20).
		scraperWSV2.subscribeInstance(name, DEBUG_CLASSES);
	});

	onDestroy(() => {
		scraperWSV2.unsubscribeInstance(name, DEBUG_CLASSES);
		scraperWSV2.disconnect();
		if (pollTimer) clearInterval(pollTimer);
		if (nowTimer) clearInterval(nowTimer);
	});

	const annPrefix = $derived(`${name}:`);

	async function exportAnnotations() {
		const json = fieldAnnotations.exportJSON(annPrefix);
		try {
			await navigator.clipboard.writeText(json);
			toaster.success({
				title: 'Annotations copied',
				description: 'Paste into an M19 follow-up note.'
			});
		} catch {
			const blob = new Blob([json], { type: 'application/json' });
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `${name}-field-annotations.json`;
			a.click();
			URL.revokeObjectURL(url);
		}
	}

	function clearAnnotations() {
		fieldAnnotations.clearPrefix(annPrefix);
		toaster.success({ title: 'Cleared', description: `${name} annotations` });
	}

	// Header-only derivations from the v2 store. Sub-tabs source their own
	// fields from scraperWSV2 (each tab's vm projects via v2→v1 adapters
	// where the section components still consume v1 shapes).
	const v2Game = $derived(scraperWSV2.game[name] ?? null);
	const v2Tick = $derived(scraperWSV2.tick[name] ?? null);
	const gameData = $derived(inspect?.game_data ?? null);
	const tick = $derived(v2Tick ?? inspect?.latest_tick ?? null);
	const runnerAttached = $derived(!!inspect?.running || !!scraper?.running);
	const phase = $derived(v2Game?.phase ?? inspect?.phase ?? 'idle');

	const running = $derived(!!scraper?.running);

	// Identity for the live status bar. Variant prefers the scenario's
	// variant_name (e.g. "Slayer", "Team Slayer Pro") and falls back to the
	// engine gametype string when the variant is unnamed.
	const variantDisplay = $derived(
		v2Game?.config?.variant_name ||
			v2Game?.config?.gametype ||
			gameData?.variant_name ||
			gameData?.gametype ||
			''
	);
	const mapDisplay = $derived(scraperWSV2.scenario[name]?.map || gameData?.map || '');
	const titleDisplay = $derived(scraperWSV2.xbox[name]?.title || scraper?.title || '');
	const xboxNameDisplay = $derived(scraperWSV2.xbox[name]?.name || scraper?.xbox_name || '');
	const playersCount = $derived(v2Game?.players?.length ?? gameData?.players?.length);
	const engineTickVal = $derived(scraperWSV2.engineTick[name] || undefined);

	// Compact team score line for the identity bar, e.g. "12 – 8". Only for
	// team games with reported scores; ordered by team index for stability.
	const scoreSummary = $derived.by(() => {
		if (!v2Game?.config?.is_team_game || !(v2Game.team_scores?.length ?? 0)) return undefined;
		return [...v2Game.team_scores]
			.sort((a, b) => a.team - b.team)
			.map((t) => t.score)
			.join(' – ');
	});

	// FEED strip — the debug analog of the visualizer's FEED capability grid:
	// which per-instance envelope classes have delivered at least one payload
	// this session (presence, not freshness — xbox/scenario/previous_game only
	// arrive on change, so freshness would read as stale on a healthy runner).
	const feedClasses = $derived<FeedClass[]>([
		{ label: 'xbox', live: scraperWSV2.xbox[name] != null, title: 'console identity' },
		{ label: 'scenario', live: scraperWSV2.scenario[name] != null, title: 'map / scenario tag' },
		{ label: 'game', live: scraperWSV2.game[name] != null, title: 'roster + config + scores' },
		{ label: 'tick', live: scraperWSV2.tick[name] != null, title: 'per-tick player state' },
		{
			label: 'objects',
			live: scraperWSV2.objects[name] != null,
			title: 'world objects + projectiles'
		},
		{ label: 'debug', live: scraperWSV2.debug[name] != null, title: 'raw debug fields' },
		{ label: 'event', live: (scraperWSV2.events[name]?.length ?? 0) > 0, title: 'game event log' },
		{
			label: 'previous',
			live: scraperWSV2.previousGame[name] != null,
			title: 'previous match summary'
		}
	]);

	const tabs = [
		{ value: 'overview', label: 'Overview' },
		{ value: 'xbox', label: 'Xbox' },
		{ value: 'scenario', label: 'Scenario' },
		{ value: 'game', label: 'Game' },
		{ value: 'tick', label: 'Tick' },
		{ value: 'objects', label: 'Objects' },
		{ value: 'debug', label: 'Debug' },
		{ value: 'previous_game', label: 'Previous' },
		{ value: 'events', label: 'Events' }
	];
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-4">
	<div class="flex items-center justify-between gap-2">
		<a class="flex items-center gap-1 anchor text-sm" href={resolve('/admin/pod/[name]', { name })}>
			<ArrowLeftIcon class="size-4" />
			Back to pod
		</a>
		<a class="btn preset-tonal btn-sm" href={resolve('/admin/pod/[name]/probe', { name })}
			>Probe →</a
		>
	</div>
	<PageHeader title={name}>
		{#snippet actions()}
			<button type="button" class="btn preset-tonal btn-sm" onclick={exportAnnotations}>
				Export annotations
			</button>
			<button type="button" class="btn preset-tonal-error btn-sm" onclick={clearAnnotations}>
				Clear
			</button>
		{/snippet}
	</PageHeader>
	<LiveStatusBar
		map={mapDisplay}
		gametype={variantDisplay}
		title={titleDisplay}
		xboxName={xboxNameDisplay}
		{phase}
		{running}
		connected={scraperWSV2.connected}
		lastError={scraperWSV2.lastError}
		players={playersCount}
		{scoreSummary}
		engineTick={engineTickVal}
		feed={feedClasses}
	/>
	{#if scraperWSV2.lastError}
		<div class="text-right text-xs text-error-500">{scraperWSV2.lastError}</div>
	{/if}

	{#if !runnerAttached && !gameData && !tick}
		<div class="card preset-tonal p-3 text-sm">
			No scraper attached for this instance. Start it from
			<a class="anchor" href={resolve('/admin/pod/[name]/view', { name })}
				>/admin/pod/[name]/view/</a
			>.
		</div>
	{/if}

	<TabsResponsive value={topTab} onValueChange={setTab} items={tabs} ariaLabel="Debug tabs">
		<Tabs.Content value="overview" class="pt-4"><OverviewTab {name} /></Tabs.Content>
		<Tabs.Content value="xbox" class="pt-4"><XboxTab {name} /></Tabs.Content>
		<Tabs.Content value="scenario" class="pt-4"><ScenarioTab {name} /></Tabs.Content>
		<Tabs.Content value="game" class="pt-4"><GameTab {name} /></Tabs.Content>
		<Tabs.Content value="tick" class="pt-4"><TickTab {name} /></Tabs.Content>
		<Tabs.Content value="objects" class="pt-4"><ObjectsTab {name} /></Tabs.Content>
		<Tabs.Content value="debug" class="pt-4"><DebugTab {name} /></Tabs.Content>
		<Tabs.Content value="previous_game" class="pt-4"><PreviousGameTab {name} /></Tabs.Content>
		<Tabs.Content value="events" class="pt-4"><EventsTab {name} /></Tabs.Content>
	</TabsResponsive>
</div>
