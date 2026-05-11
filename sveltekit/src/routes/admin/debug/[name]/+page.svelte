<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import {
		ActivityIcon,
		ArrowLeftIcon,
		Gamepad2Icon,
		ServerIcon,
		TagIcon,
		WifiIcon
	} from '@lucide/svelte';
	import { Tabs, Switch } from '@skeletonlabs/skeleton-svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import type { InstanceState } from '$lib/types/containers';
	import type { ScraperInspect } from '$lib/types/scraper';
	import { fieldAnnotations } from '$lib/stores/fieldAnnotations.svelte';
	import { toaster } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import TabsResponsive from '$lib/components/debug/TabsResponsive.svelte';
	import OverviewTab from '$lib/components/debug/overview/OverviewTab.svelte';
	import GameTab from '$lib/components/debug/game/GameTab.svelte';
	import TickTab from '$lib/components/debug/tick/TickTab.svelte';
	import PostgameTab from '$lib/components/debug/postgame/PostgameTab.svelte';
	import TabPlaceholder from '$lib/components/debug/TabPlaceholder.svelte';
	import { setDebugContext } from '$lib/components/debug/context.js';
	import { fetchDebugDetail } from '$lib/components/debug/refresh.js';

	let { data } = $props();
	const name = $derived(data.name);

	let scraper = $state<InstanceState | null>(null);
	let inspect = $state<ScraperInspect | null>(null);
	let inspectAt = $state<number | undefined>(undefined);
	let now = $state(Date.now());
	let showAll = $state(false);

	const TAB_VALUES = [
		'overview',
		'game',
		'tick',
		'postgame',
		'events',
		'probe',
		'raw',
		'logs',
		'controls'
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
		get showAll() {
			return showAll;
		},
		get now() {
			return now;
		},
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

	onMount(() => {
		try {
			const saved = localStorage.getItem('debug.showAll');
			if (saved !== null) showAll = saved === 'true';
		} catch {
			// localStorage unavailable; keep default.
		}
		if (auth.token) scraperWS.connect(auth.token);
		refreshDetail();
		pollTimer = setInterval(() => {
			if (document.visibilityState === 'visible') refreshDetail();
		}, 3000);
		nowTimer = setInterval(() => (now = Date.now()), 1000);
	});

	$effect(() => {
		if (!scraperWS.connected) return;
		scraperWS.ensureSubscribed([name]);
		scraperWS.requestState();
	});

	$effect(() => {
		try {
			localStorage.setItem('debug.showAll', String(showAll));
		} catch {
			// ignore
		}
	});

	onDestroy(() => {
		scraperWS.disconnect();
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

	const gameData = $derived(scraperWS.gameData[name] ?? inspect?.game_data ?? null);
	const tick = $derived(scraperWS.ticks[name] ?? inspect?.latest_tick ?? null);
	const runnerAttached = $derived(!!inspect?.running || !!scraper?.running);
	const phase = $derived(scraperWS.phases[name] ?? inspect?.phase ?? 'idle');

	// Combined scraper/phase tile colour: dot reflects whether the scraper is
	// running, text reflects the phase. When the scraper isn't running we
	// dim everything to 'off' since the phase value is stale.
	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';
	const phaseTileKind = $derived<StatusKind>(
		!scraper?.running ? 'off' : phase === 'live' ? 'on' : phase === 'ready' ? 'warn' : 'neutral'
	);

	// Variant tile prefers the scenario's variant_name (e.g. "Slayer",
	// "Team Slayer Pro") and falls back to the engine gametype string when
	// the variant is unnamed — same value the gametype tile in the Game
	// section shows, so the header still surfaces "what's being played".
	const variantDisplay = $derived(gameData?.variant_name || gameData?.gametype || '—');

	const tabs = [
		{ value: 'overview', label: 'Overview' },
		{ value: 'game', label: 'Game' },
		{ value: 'tick', label: 'Tick' },
		{ value: 'postgame', label: 'Postgame' },
		{ value: 'events', label: 'Events' },
		{ value: 'probe', label: 'Probe' },
		{ value: 'raw', label: 'Raw' },
		{ value: 'logs', label: 'Logs' },
		{ value: 'controls', label: 'Controls' }
	];
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-4">
	<a class="flex items-center gap-1 anchor text-sm" href={resolve('/admin/debug/')}>
		<ArrowLeftIcon class="size-4" />
		Back to debug
	</a>
	<PageHeader title={name}>
		{#snippet actions()}
			<label class="flex items-center gap-2 text-xs">
				<Switch
					checked={showAll}
					onCheckedChange={(d) => (showAll = d.checked)}
					name="debug-show-all"
				>
					<Switch.Control><Switch.Thumb /></Switch.Control>
					<Switch.HiddenInput />
				</Switch>
				<span>Show all data</span>
			</label>
			<button type="button" class="btn preset-tonal btn-sm" onclick={exportAnnotations}>
				Export annotations
			</button>
			<button type="button" class="btn preset-tonal-error btn-sm" onclick={clearAnnotations}>
				Clear
			</button>
		{/snippet}
	</PageHeader>
	{#snippet wsIcon()}<WifiIcon class="size-3.5" />{/snippet}
	{#snippet phaseIcon()}<ActivityIcon class="size-3.5" />{/snippet}
	{#snippet xboxIcon()}<ServerIcon class="size-3.5" />{/snippet}
	{#snippet titleIcon()}<TagIcon class="size-3.5" />{/snippet}
	{#snippet variantIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
	<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-5">
		<StatTile
			label="websockets"
			display={scraperWS.connected ? 'connected' : 'disconnected'}
			statusKind={scraperWS.connected ? 'on' : 'off'}
			title={scraperWS.lastError ?? (scraperWS.connected ? 'connected' : 'disconnected')}
			icon={wsIcon}
		/>
		<StatTile
			label="scraper phase"
			display={phase}
			statusKind={phaseTileKind}
			title={scraper?.running
				? `scraper running · phase ${phase}`
				: `scraper stopped · last phase ${phase}`}
			icon={phaseIcon}
		/>
		<StatTile
			label="xbox"
			display={scraper?.xbox_name || '—'}
			statusKind={scraper?.xbox_name ? 'on' : 'none'}
			title={scraper?.xbox_name}
			icon={xboxIcon}
		/>
		<StatTile
			label="title"
			display={scraper?.title || '—'}
			statusKind={scraper?.title ? 'on' : 'none'}
			title={scraper?.title}
			icon={titleIcon}
		/>
		<StatTile
			label="variant"
			display={variantDisplay}
			statusKind={variantDisplay === '—' ? 'none' : 'on'}
			title={variantDisplay}
			icon={variantIcon}
		/>
	</div>
	{#if scraperWS.lastError}
		<div class="text-right text-xs text-error-500">{scraperWS.lastError}</div>
	{/if}

	{#if !runnerAttached && !gameData && !tick}
		<div class="card preset-tonal p-3 text-sm">
			No scraper attached for this instance. Start it from
			<a class="anchor" href={resolve(`/containers/${name}/`)}>/containers/{name}/</a>.
		</div>
	{/if}

	<TabsResponsive value={topTab} onValueChange={setTab} items={tabs} ariaLabel="Debug tabs">
		<Tabs.Content value="overview" class="pt-4"><OverviewTab {name} /></Tabs.Content>
		<Tabs.Content value="game" class="pt-4"><GameTab {name} /></Tabs.Content>
		<Tabs.Content value="tick" class="pt-4"><TickTab {name} /></Tabs.Content>
		<Tabs.Content value="postgame" class="pt-4"><PostgameTab {name} /></Tabs.Content>
		<Tabs.Content value="events" class="pt-4">
			<TabPlaceholder
				title="Events"
				description="Live event feed for the active match — kills, captures, betrayals, double-kills, and similar, with per-type frequency stats. Streamed from the scraper event envelopes."
			/>
		</Tabs.Content>
		<Tabs.Content value="probe" class="pt-4">
			<TabPlaceholder
				title="Probe"
				description="Live memory-probe diagnostics — score-probe output and LastStateInputs() from the active GameReader. Used for offset hunting and verifying that the reader is converging on the right struct."
			/>
		</Tabs.Content>
		<Tabs.Content value="raw" class="pt-4">
			<TabPlaceholder
				title="Raw"
				description="Full JSON dump of the latest scraper inspect snapshot. The unfiltered source for everything the other tabs render — useful when a field looks wrong or missing."
			/>
		</Tabs.Content>
		<Tabs.Content value="logs" class="pt-4">
			<TabPlaceholder
				title="Logs"
				description="Container logs for this instance — xemu and Firefox-kiosk stdout, plus any other container-specific log sources worth surfacing. Will mirror the logs panel already on /containers/[name]/."
			/>
		</Tabs.Content>
		<Tabs.Content value="controls" class="pt-4">
			<TabPlaceholder
				title="Controls"
				description="Container controls for this instance — embedded kiosk view, Xbox controller for sending input over VNC, and start/stop buttons. Will mirror the kiosk + controller panel already on /containers/[name]/."
			/>
		</Tabs.Content>
	</TabsResponsive>
</div>
