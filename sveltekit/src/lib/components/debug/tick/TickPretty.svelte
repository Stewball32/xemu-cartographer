<script lang="ts">
	// Pretty view for the Tick debug tab — five accordion sections that
	// mirror the v2 TickPayload JSON shape exactly: players / power_items /
	// ctf_flags / game_globals / locals. Every section renders even when
	// empty so the operator can tell the difference between "no data yet"
	// (empty state in section) and "the field isn't on the wire".
	//
	// High-frequency caveat: tick envelopes land at ~30 Hz once live.
	// Svelte 5 reactive scoping handles the per-section re-renders without
	// the surrounding accordion shell flickering; DataTable's sort is
	// computed once per render and the rows are immutable per-tick objects
	// so identity churn is unavoidable but limited to the table body.

	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		ActivityIcon,
		ChevronDownIcon,
		FlagIcon,
		Gamepad2Icon,
		PackageIcon,
		RotateCwIcon,
		SettingsIcon,
		ShieldIcon,
		UsersIcon
	} from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { TickPayloadV2 } from '$lib/types/scraper-v2';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import {
		activeActionLabels,
		buildTickPrettyVm,
		type FieldDisplay,
		type PrettyCTFFlagRow,
		type PrettyLocalCard,
		type PrettyPlayerRow,
		type PrettyPowerItemRow
	} from './tick-vm';

	let { payload }: { payload: TickPayloadV2 | null } = $props();

	const vm = $derived(buildTickPrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses 'power_items' here keeps it collapsed across reloads.
	// Persisted alongside the other tabs' collapse keys.
	type SectionId = 'players' | 'power_items' | 'ctf_flags' | 'game_globals' | 'locals';
	const ALL_SECTIONS: readonly SectionId[] = [
		'players',
		'power_items',
		'ctf_flags',
		'game_globals',
		'locals'
	];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.tick.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.tick.collapsed', JSON.stringify([...collapsedSections]));
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

	// Per-section row counts for the header subtitle.
	const playersCount = $derived(vm.players.length);
	const powerItemsCount = $derived(vm.powerItems.length);
	const ctfFlagsCount = $derived(vm.ctfFlags.length);
	const localsCount = $derived(vm.locals.length);

	// ----- players DataTable groups -----
	const playerGroups: DataColumnGroup<PrettyPlayerRow>[] = [
		{
			label: 'identity',
			columns: [
				{ key: 'index', label: 'idx' },
				{ key: 'biped_tag', label: 'biped_tag' }
			]
		},
		{
			label: 'state',
			columns: [
				{ key: 'alive', label: 'alive' },
				{ key: 'respawn_in_ticks', label: 'respawn_in_ticks' },
				{ key: 'health', label: 'health' },
				{ key: 'shields', label: 'shields' },
				{ key: 'has_camo', label: 'has_camo' },
				{ key: 'has_overshield', label: 'has_overshield' }
			]
		},
		{
			label: 'pos',
			columns: [
				{ key: 'pos_x', label: 'x' },
				{ key: 'pos_y', label: 'y' },
				{ key: 'pos_z', label: 'z' }
			]
		},
		{
			label: 'vel',
			columns: [
				{ key: 'vel_x', label: 'x' },
				{ key: 'vel_y', label: 'y' },
				{ key: 'vel_z', label: 'z' }
			]
		},
		{
			label: 'aim',
			columns: [
				{ key: 'aim_x', label: 'x' },
				{ key: 'aim_y', label: 'y' },
				{ key: 'aim_z', label: 'z' },
				{ key: 'zoom_level', label: 'zoom_level' },
				{ key: 'crouch_scale', label: 'crouch_scale' }
			]
		},
		{
			label: 'combat',
			columns: [
				{ key: 'frags', label: 'frags' },
				{ key: 'plasmas', label: 'plasmas' },
				{ key: 'selected_weapon_slot', label: 'sel_slot' },
				{ key: 'actions', label: 'actions', sortable: false, cell: actionChips },
				{ key: 'weapons', label: 'weapons', sortable: false, cell: weaponsCell }
			]
		}
	];

	// ----- power_items DataTable groups -----
	const powerItemGroups: DataColumnGroup<PrettyPowerItemRow>[] = [
		{
			label: 'spawn',
			columns: [
				{ key: 'spawn_id', label: 'spawn_id' },
				{ key: 'status', label: 'status' },
				{ key: 'held_by', label: 'held_by' },
				{ key: 'respawn_in_ticks', label: 'respawn_in_ticks' }
			]
		},
		{
			label: 'pos',
			columns: [
				{ key: 'pos_x', label: 'x' },
				{ key: 'pos_y', label: 'y' },
				{ key: 'pos_z', label: 'z' }
			]
		}
	];

	// ----- ctf_flags DataTable groups -----
	const ctfFlagGroups: DataColumnGroup<PrettyCTFFlagRow>[] = [
		{
			label: 'flag',
			columns: [
				{ key: 'team', label: 'team' },
				{ key: 'carrier', label: 'carrier' },
				{ key: 'status', label: 'status' }
			]
		},
		{
			label: 'pos',
			columns: [
				{ key: 'pos_x', label: 'x' },
				{ key: 'pos_y', label: 'y' },
				{ key: 'pos_z', label: 'z' }
			]
		}
	];
</script>

<!-- Cells for the players table — flag chips for the actions struct and a -->
<!-- compact summary of the per-player weapons slots. -->
{#snippet actionChips({ row }: { row: PrettyPlayerRow })}
	{@const labels = activeActionLabels(row.actions)}
	{#if labels.length === 0}
		<span class="text-surface-500-400 font-mono">—</span>
	{:else}
		<div class="flex flex-wrap gap-1">
			{#each labels as l (l)}
				<span class="badge preset-tonal-primary font-mono text-[10px]">{l}</span>
			{/each}
		</div>
	{/if}
{/snippet}

{#snippet weaponsCell({ row }: { row: PrettyPlayerRow })}
	{#if row.weapons.length === 0}
		<span class="text-surface-500-400 font-mono">—</span>
	{:else}
		<div class="flex flex-col gap-0.5 font-mono text-[11px]">
			{#each row.weapons as w (w.slot)}
				<span class="truncate">
					<span class="text-surface-500-400">slot {w.slot}:</span>
					{w.tag || '—'}
					{#if w.ammo_mag !== null || w.ammo_pack !== null}
						<span class="text-surface-500-400">
							({w.ammo_mag ?? '—'}/{w.ammo_pack ?? '—'})
						</span>
					{/if}
				</span>
			{/each}
		</div>
	{/if}
{/snippet}

{#snippet sectionHeader(id: string, count?: number)}
	<span
		class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		<code class="font-mono">{id}</code>
		{#if count !== undefined}
			<span class="text-surface-500-400 font-mono normal-case tabular-nums">[{count}]</span>
		{/if}
	</span>
{/snippet}

{#snippet tile(label: string, f: FieldDisplay, icon?: import('svelte').Snippet)}
	<StatTile
		{label}
		display={f.display}
		statusKind={f.present ? 'on' : 'none'}
		title={f.raw}
		{icon}
	/>
{/snippet}

{#snippet activityIcon()}<ActivityIcon class="size-3.5" />{/snippet}
{#snippet flagIcon()}<FlagIcon class="size-3.5" />{/snippet}
{#snippet packageIcon()}<PackageIcon class="size-3.5" />{/snippet}
{#snippet rotateIcon()}<RotateCwIcon class="size-3.5" />{/snippet}
{#snippet settingsIcon()}<SettingsIcon class="size-3.5" />{/snippet}
{#snippet shieldIcon()}<ShieldIcon class="size-3.5" />{/snippet}
{#snippet gamepadIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="players">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('players', playersCount)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.players}
				groups={playerGroups}
				rowKey={(r) => r.index}
				emptyMessage="no players in current tick"
				defaultSort={{ key: 'index', dir: 'asc' }}
				secondarySort={{ key: 'index' }}
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="power_items">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('power_items', powerItemsCount)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.powerItems}
				groups={powerItemGroups}
				rowKey={(r) => r.spawn_id}
				emptyMessage="no live power-item statuses in current tick"
				defaultSort={{ key: 'spawn_id', dir: 'asc' }}
				secondarySort={{ key: 'spawn_id' }}
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="ctf_flags">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('ctf_flags', ctfFlagsCount)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.ctfFlags}
				groups={ctfFlagGroups}
				rowKey={(r, i) => `${r.team}:${i}`}
				emptyMessage="no CTF flags in current tick (non-CTF gametype?)"
				defaultSort={{ key: 'team', dir: 'asc' }}
				secondarySort={{ key: 'team' }}
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="game_globals">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('game_globals')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5">
				{@render tile('map_loaded', vm.gameGlobals.map_loaded, packageIcon)}
				{@render tile('active', vm.gameGlobals.active, activityIcon)}
				{@render tile(
					'game_loading_in_progress',
					vm.gameGlobals.game_loading_in_progress,
					rotateIcon
				)}
				{@render tile('precache_map_status', vm.gameGlobals.precache_map_status, settingsIcon)}
				{@render tile('stored_global_random', vm.gameGlobals.stored_global_random, shieldIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="locals">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('locals', localsCount)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{#if vm.locals.length === 0}
				<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
					no local players in current tick
				</div>
			{:else}
				<div class="grid gap-3 lg:grid-cols-2">
					{#each vm.locals as local (local.local_index)}
						{@render localCard(local)}
					{/each}
				</div>
			{/if}
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>

{#snippet localCard(local: PrettyLocalCard)}
	<div class="card preset-tonal p-3">
		<header class="mb-3 flex items-baseline gap-2">
			<UsersIcon class="text-surface-700-200 size-3.5" />
			<span class="text-xs font-semibold tracking-wide uppercase">
				local <span class="font-mono">#{local.local_index}</span>
			</span>
		</header>

		<div class="mb-3">
			<div class="text-surface-700-200 mb-1 text-[10px] font-semibold tracking-wide uppercase">
				<code class="font-mono">fp_weapon</code>
			</div>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
				{@render tile('state', local.fp_weapon.state, settingsIcon)}
				{@render tile('animation_id', local.fp_weapon.animation_id, settingsIcon)}
				{@render tile('animation_tick', local.fp_weapon.animation_tick, settingsIcon)}
			</div>
		</div>

		<div class="mb-3">
			<div class="text-surface-700-200 mb-1 text-[10px] font-semibold tracking-wide uppercase">
				<code class="font-mono">observer_cam</code>
			</div>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
				{@render tile('pos', local.observer_cam.pos, packageIcon)}
				{@render tile('aim', local.observer_cam.aim, flagIcon)}
				{@render tile('fov', local.observer_cam.fov, activityIcon)}
			</div>
		</div>

		<div class="mb-3">
			<div class="text-surface-700-200 mb-1 text-[10px] font-semibold tracking-wide uppercase">
				<code class="font-mono">input</code>
			</div>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
				{@render tile('left_stick', local.input.left_stick, gamepadIcon)}
				{@render tile('right_stick', local.input.right_stick, gamepadIcon)}
				{@render tile('buttons', local.input.buttons, gamepadIcon)}
			</div>
		</div>

		<div>
			<div class="text-surface-700-200 mb-1 text-[10px] font-semibold tracking-wide uppercase">
				<code class="font-mono">player_control</code>
			</div>
			<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
				{@render tile('desired_yaw', local.player_control.desired_yaw, rotateIcon)}
				{@render tile('desired_pitch', local.player_control.desired_pitch, rotateIcon)}
				{@render tile('zoom_level', local.player_control.zoom_level, settingsIcon)}
				{@render tile('aim_assist_target', local.player_control.aim_assist_target, flagIcon)}
			</div>
		</div>
	</div>
{/snippet}
