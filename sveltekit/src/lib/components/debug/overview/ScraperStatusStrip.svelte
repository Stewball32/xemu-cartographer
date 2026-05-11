<script lang="ts">
	import {
		ActivityIcon,
		DatabaseIcon,
		HashIcon,
		RadioTowerIcon,
		WifiIcon
	} from '@lucide/svelte';
	import type { Phase } from '$lib/types/scraper';
	import StatTile, { type StatRow } from '../shared/StatTile.svelte';
	import { fmtTickAsTime } from '../shared/util';

	let {
		phase,
		lastReadStr,
		gameDataStr,
		tickStr,
		tickValue,
		wsConnected,
		lastEventsReplyStr,
		lastEventsReplyPhase,
		showHeader = true
	}: {
		phase: Phase;
		lastReadStr: string;
		gameDataStr: string;
		tickStr: string;
		tickValue: number | undefined;
		wsConnected: boolean;
		lastEventsReplyStr: string;
		lastEventsReplyPhase: Phase | undefined;
		showHeader?: boolean;
	} = $props();

	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';

	const phaseKind = $derived<StatusKind>(
		phase === 'live' ? 'on' : phase === 'ready' ? 'warn' : 'neutral'
	);

	// "just now" / "Xs ago" → neutral, but "never" should be off (signal that
	// the runner / WS hasn't produced data yet).
	function freshnessKind(s: string): StatusKind {
		if (s === 'never' || s === '—') return 'off';
		return 'neutral';
	}

	// Reads tile: are we still pulling data from the runner?
	const readsRows = $derived<StatRow[]>([
		{
			label: 'last read',
			display: lastReadStr,
			statusKind: freshnessKind(lastReadStr),
			title: 'time since last successful memory read'
		},
		{
			label: 'game data',
			display: gameDataStr,
			statusKind: freshnessKind(gameDataStr),
			title: 'time since last game-data envelope'
		}
	]);

	// Sync tile: are the WS event streams flowing?
	const syncRows = $derived<StatRow[]>([
		{
			label: 'tick age',
			display: tickStr,
			statusKind: freshnessKind(tickStr),
			title: 'time since last tick envelope'
		},
		{
			label: 'events sync',
			display: lastEventsReplyStr,
			statusKind: freshnessKind(lastEventsReplyStr),
			title: lastEventsReplyPhase
				? `last events-reply received (runner phase at the time: ${lastEventsReplyPhase})`
				: 'no events-reply received yet'
		}
	]);

	const tickFooter = $derived(
		tickValue !== undefined && tickValue > 0 ? `engine time ${fmtTickAsTime(tickValue)}` : ''
	);
</script>

{#snippet wsIcon()}<WifiIcon class="size-3.5" />{/snippet}
{#snippet phaseIcon()}<ActivityIcon class="size-3.5" />{/snippet}
{#snippet readsIcon()}<DatabaseIcon class="size-3.5" />{/snippet}
{#snippet syncIcon()}<RadioTowerIcon class="size-3.5" />{/snippet}
{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Server
		</div>
	{/if}
	<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5">
		<StatTile
			label="ws"
			display={wsConnected ? 'connected' : 'disconnected'}
			statusKind={wsConnected ? 'on' : 'off'}
			title={wsConnected ? 'WebSocket connected' : 'WebSocket disconnected'}
			icon={wsIcon}
		/>
		<StatTile
			label="phase"
			display={phase}
			statusKind={phaseKind}
			title="scraper phase: {phase}"
			icon={phaseIcon}
		/>
		<StatTile label="reads" rows={readsRows} icon={readsIcon} />
		<StatTile label="sync" rows={syncRows} icon={syncIcon} />
		<StatTile
			label="tick #"
			display={tickValue !== undefined ? String(tickValue) : '—'}
			statusKind={tickValue !== undefined && tickValue > 0 ? 'pointer' : 'neutral'}
			title="engine tick number from most recent envelope"
			icon={hashIcon}
			footer={tickFooter || undefined}
		/>
	</div>
</section>
