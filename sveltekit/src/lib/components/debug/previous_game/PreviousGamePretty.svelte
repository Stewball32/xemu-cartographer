<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		ActivityIcon,
		CalendarClockIcon,
		ChevronDownIcon,
		ClockIcon,
		Gamepad2Icon,
		HashIcon,
		ListChecksIcon,
		PlayIcon,
		RepeatIcon,
		ServerIcon,
		TargetIcon,
		TimerIcon,
		TrophyIcon,
		UsersIcon
	} from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { PreviousGamePayload } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import { buildPreviousGamePrettyVm, type EventRow, type FieldDisplay } from './previous-game-vm';

	let { payload }: { payload: PreviousGamePayload | null } = $props();

	const vm = $derived.by(() => buildPreviousGamePrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses 'events' here keeps it collapsed across reloads. Persisted
	// under a new key — older debug.postgame.collapsed entries are ignored
	// (the previous tab tracked headline/scoreboard/player_totals/event_log,
	// none of which map onto the new envelope-class layout).
	type SectionId = 'top-level' | 'game' | 'events';
	const ALL_SECTIONS: readonly SectionId[] = ['top-level', 'game', 'events'];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.previous_game.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.previous_game.collapsed', JSON.stringify([...collapsedSections]));
		} catch {
			// ignore
		}
	}

	const openSections = $derived(ALL_SECTIONS.filter((id) => !collapsedSections.has(id)));

	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const id of ALL_SECTIONS) {
			if (!open.has(id)) newCollapsed.add(id);
		}
		collapsedSections = newCollapsed;
		saveCollapsed();
	}

	onMount(loadCollapsed);

	// DataTable column groups for the events array. Mirrors the v2 Envelope
	// shape: top-level seq/tick/ts/type plus the inner event_type
	// discriminator (data.event_type). secondarySort on tick keeps rows
	// stable between renders when seq is monotonic.
	const eventGroups: DataColumnGroup<EventRow>[] = [
		{
			columns: [
				{ key: 'seq', label: 'seq', align: 'right' },
				{ key: 'tick', label: 'tick', align: 'right' },
				{ key: 'ts', label: 'ts' },
				{ key: 'type', label: 'type' },
				{ key: 'event_type', label: 'data.event_type' }
			]
		}
	];
</script>

{#snippet sectionHeader(id: string, subtitle?: string)}
	<span
		class="text-surface-700-200 inline-flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		<code class="font-mono">{id}</code>
		{#if subtitle}
			<span class="text-surface-500-400 font-mono normal-case tabular-nums">{subtitle}</span>
		{/if}
	</span>
{/snippet}

{#snippet calendarIcon()}<CalendarClockIcon class="size-3.5" />{/snippet}
{#snippet activityIcon()}<ActivityIcon class="size-3.5" />{/snippet}
{#snippet playIcon()}<PlayIcon class="size-3.5" />{/snippet}
{#snippet clockIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}
{#snippet repeatIcon()}<RepeatIcon class="size-3.5" />{/snippet}
{#snippet gamepadIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
{#snippet trophyIcon()}<TrophyIcon class="size-3.5" />{/snippet}
{#snippet usersIcon()}<UsersIcon class="size-3.5" />{/snippet}
{#snippet serverIcon()}<ServerIcon class="size-3.5" />{/snippet}
{#snippet listIcon()}<ListChecksIcon class="size-3.5" />{/snippet}
{#snippet targetIcon()}<TargetIcon class="size-3.5" />{/snippet}
{#snippet timerIcon()}<TimerIcon class="size-3.5" />{/snippet}

{#snippet tile(label: string, f: FieldDisplay, icon?: import('svelte').Snippet)}
	<StatTile
		{label}
		display={f.display}
		statusKind={f.present ? 'on' : 'none'}
		title={f.raw}
		{icon}
	/>
{/snippet}

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="top-level">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('top-level')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
				{@render tile('ended_at', vm.topLevel.ended_at, calendarIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="game">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('game', vm.game.present ? undefined : 'null')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
				{@render tile('phase', vm.game.phase, activityIcon)}
				{@render tile('started_at', vm.game.started_at, playIcon)}
				{@render tile('last_read_at', vm.game.last_read_at, clockIcon)}
				{@render tile('engine_tick', vm.game.engine_tick, hashIcon)}
				{@render tile('iterations', vm.game.iterations, repeatIcon)}
				{@render tile('gametype', vm.game.gametype, gamepadIcon)}
				{@render tile('variant_name', vm.game.variant_name, gamepadIcon)}
				{@render tile('is_team_game', vm.game.is_team_game, usersIcon)}
				{@render tile('score_limit', vm.game.score_limit, trophyIcon)}
				{@render tile('time_limit_ticks', vm.game.time_limit_ticks, timerIcon)}
				{@render tile('players_count', vm.game.players_count, usersIcon)}
				{@render tile('machines_count', vm.game.machines_count, serverIcon)}
				{@render tile('team_scores', vm.game.team_scores_summary, targetIcon)}
			</div>
			<!-- The full GamePayload is large; the Game tab is the canonical
				live view. The summary tiles above cover the fields most useful
				for "what game just ended"; deeper drill-down lives one tab
				over. -->
			<div class="text-surface-500-400 mt-3 text-xs">
				Full live game state lives in the <code class="font-mono">Game</code> tab.
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="events">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('events', `[${vm.events.count}]`)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="flex flex-col gap-2">
				<div
					class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
				>
					{@render listIcon()}
					{vm.events.count}
					{vm.events.count === 1 ? 'event' : 'events'} captured
				</div>
				<DataTable
					rows={vm.events.rows}
					groups={eventGroups}
					rowKey={(r) => `${r.seq}:${r.tick}:${r.event_type}`}
					defaultSort={{ key: 'tick', dir: 'asc' }}
					secondarySort={{ key: 'seq', dir: 'asc' }}
					emptyMessage="no events captured for the previous game"
				/>
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
