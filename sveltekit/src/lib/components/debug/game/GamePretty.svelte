<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import type { Snippet } from 'svelte';
	import {
		ActivityIcon,
		ChevronDownIcon,
		ClockIcon,
		Gamepad2Icon,
		HashIcon,
		PowerIcon,
		RepeatIcon,
		TagIcon,
		TimerIcon,
		TrophyIcon,
		UsersIcon
	} from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { GamePayload } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import {
		buildGamePrettyVm,
		type FieldDisplay,
		type GameMachineRow,
		type GamePlayerRow,
		type GameTeamScoreRow
	} from './game-vm';

	let { payload }: { payload: GamePayload | null } = $props();

	const vm = $derived.by(() => buildGamePrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses 'config' here keeps it collapsed across reloads. Persisted
	// alongside the other tabs' collapse keys (debug.xbox.collapsed etc.).
	type SectionId = 'top-level' | 'config' | 'team_scores' | 'players' | 'machines' | 'network';
	const ALL_SECTIONS: readonly SectionId[] = [
		'top-level',
		'config',
		'team_scores',
		'players',
		'machines',
		'network'
	];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.game.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.game.collapsed', JSON.stringify([...collapsedSections]));
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

	// DataTable column groups for the three array sections. Keys are the
	// literal JSON field names so the table headers match what the operator
	// sees in the JSON view; that's why the headers stay snake_case.
	const teamScoreGroups: DataColumnGroup<GameTeamScoreRow>[] = [
		{
			columns: [{ key: 'team' }, { key: 'score' }]
		}
	];

	const playerGroups: DataColumnGroup<GamePlayerRow>[] = [
		{
			columns: [
				{ key: 'index' },
				{ key: 'name' },
				{ key: 'team' },
				{ key: 'armor_color' },
				{ key: 'score' },
				{ key: 'kills' },
				{ key: 'deaths' },
				{ key: 'assists' },
				{ key: 'ctf_score' },
				{ key: 'team_kills' },
				{ key: 'suicides' },
				{ key: 'kill_streak' },
				{ key: 'multikill' },
				{ key: 'shots_fired' },
				{ key: 'shots_hit' },
				{ key: 'is_local' },
				{ key: 'local_index' },
				{ key: 'machine_index' },
				{ key: 'controller_index' }
			]
		}
	];

	const machineGroups: DataColumnGroup<GameMachineRow>[] = [
		{
			columns: [{ key: 'index' }, { key: 'name' }, { key: 'is_local' }]
		}
	];
</script>

{#snippet sectionHeader(id: string)}
	<span
		class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		<code class="font-mono">{id}</code>
	</span>
{/snippet}

{#snippet activityIcon()}<ActivityIcon class="size-3.5" />{/snippet}
{#snippet powerIcon()}<PowerIcon class="size-3.5" />{/snippet}
{#snippet clockIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet timerIcon()}<TimerIcon class="size-3.5" />{/snippet}
{#snippet repeatIcon()}<RepeatIcon class="size-3.5" />{/snippet}
{#snippet gameIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
{#snippet tagIcon()}<TagIcon class="size-3.5" />{/snippet}
{#snippet usersIcon()}<UsersIcon class="size-3.5" />{/snippet}
{#snippet trophyIcon()}<TrophyIcon class="size-3.5" />{/snippet}
{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}

{#snippet tile(label: string, f: FieldDisplay, icon?: Snippet)}
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
				{@render tile('phase', vm.topLevel.phase, activityIcon)}
				{@render tile('started_at', vm.topLevel.started_at, powerIcon)}
				{@render tile('last_read_at', vm.topLevel.last_read_at, clockIcon)}
				{@render tile('engine_tick', vm.topLevel.engine_tick, timerIcon)}
				{@render tile('iterations', vm.topLevel.iterations, repeatIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="config">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('config')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
				{@render tile('gametype', vm.config.gametype, gameIcon)}
				{@render tile('variant_name', vm.config.variant_name, tagIcon)}
				{@render tile('is_team_game', vm.config.is_team_game, usersIcon)}
				{@render tile('score_limit', vm.config.score_limit, trophyIcon)}
				{@render tile('time_limit_ticks', vm.config.time_limit_ticks, timerIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="team_scores">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('team_scores')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.teamScores}
				groups={teamScoreGroups}
				defaultSort={{ key: 'team', dir: 'asc' }}
				emptyMessage="no team scores"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="players">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('players')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.players}
				groups={playerGroups}
				stickyFirst
				defaultSort={{ key: 'index', dir: 'asc' }}
				emptyMessage="no players"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="machines">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('machines')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.machines}
				groups={machineGroups}
				defaultSort={{ key: 'index', dir: 'asc' }}
				emptyMessage="no machines"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="network">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('network')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
				{@render tile('countdown.active', vm.network['countdown.active'], activityIcon)}
				{@render tile('countdown.paused', vm.network['countdown.paused'], timerIcon)}
				{@render tile(
					'countdown.seconds_to_start',
					vm.network['countdown.seconds_to_start'],
					clockIcon
				)}
				{@render tile('client.machine_index', vm.network['client.machine_index'], hashIcon)}
				{@render tile('client.average_ping', vm.network['client.average_ping'], activityIcon)}
				{@render tile('client.packets_sent', vm.network['client.packets_sent'], repeatIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
