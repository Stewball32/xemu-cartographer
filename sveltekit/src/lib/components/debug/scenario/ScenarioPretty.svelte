<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		ChevronDownIcon,
		DropletIcon,
		GamepadIcon,
		HardDriveIcon,
		MapIcon,
		PaletteIcon,
		RulerIcon
	} from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type {
		ScenarioObjectType,
		ScenarioPayload,
		ScenarioPlayerSpawn,
		ScenarioPowerItemSpawn
	} from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import {
		buildScenarioPrettyVm,
		tagDefFields,
		type FieldDisplay,
		type MemoryRegionVm,
		type TagDefEntry
	} from './scenario-vm';

	let { payload }: { payload: ScenarioPayload | null } = $props();

	const vm = $derived.by(() => buildScenarioPrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses 'fog' here keeps it collapsed across reloads. Persisted
	// alongside the other tabs' collapse keys.
	type SectionId =
		| 'top-level'
		| 'fog'
		| 'memory_regions'
		| 'object_types'
		| 'player_spawns'
		| 'power_item_spawns'
		| 'tag_defs';
	const ALL_SECTIONS: readonly SectionId[] = [
		'top-level',
		'fog',
		'memory_regions',
		'object_types',
		'player_spawns',
		'power_item_spawns',
		'tag_defs'
	];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.scenario.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.scenario.collapsed', JSON.stringify([...collapsedSections]));
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

	// Vec3 + scalar formatters for DataTable cells. Tables get the raw v2
	// payload rows; column `format` does the rendering while `sortAccessor`
	// preserves the underlying numeric ordering.
	function fmtCoord(n: number | undefined | null): string {
		if (n === undefined || n === null || !Number.isFinite(n)) return '—';
		return n.toFixed(3);
	}

	function fmtFacing(n: number | undefined | null): string {
		if (n === undefined || n === null || !Number.isFinite(n)) return '—';
		return `${n.toFixed(3)} rad`;
	}

	function fmtGametypes(g: number[] | undefined | null): string {
		if (!g || g.length === 0) return '—';
		return g.join(', ');
	}

	const objectTypeGroups: DataColumnGroup<ScenarioObjectType>[] = [
		{
			columns: [
				{ key: 'type_index', label: 'type_index', align: 'right' },
				{ key: 'name', label: 'name' },
				{ key: 'datum_size', label: 'datum_size', align: 'right' }
			]
		}
	];

	const playerSpawnGroups: DataColumnGroup<ScenarioPlayerSpawn>[] = [
		{
			columns: [
				{ key: 'index', label: 'index', align: 'right' },
				{
					key: 'pos.x',
					label: 'pos.x',
					align: 'right',
					sortAccessor: (r) => r.pos?.x ?? null,
					format: (r) => fmtCoord(r.pos?.x)
				},
				{
					key: 'pos.y',
					label: 'pos.y',
					align: 'right',
					sortAccessor: (r) => r.pos?.y ?? null,
					format: (r) => fmtCoord(r.pos?.y)
				},
				{
					key: 'pos.z',
					label: 'pos.z',
					align: 'right',
					sortAccessor: (r) => r.pos?.z ?? null,
					format: (r) => fmtCoord(r.pos?.z)
				},
				{
					key: 'facing',
					label: 'facing',
					align: 'right',
					sortAccessor: (r) => r.facing,
					format: (r) => fmtFacing(r.facing)
				},
				{ key: 'team_index', label: 'team_index', align: 'right' },
				{ key: 'bsp_index', label: 'bsp_index', align: 'right' },
				{
					key: 'gametypes',
					label: 'gametypes',
					sortable: false,
					format: (r) => fmtGametypes(r.gametypes)
				}
			]
		}
	];

	const powerItemSpawnGroups: DataColumnGroup<ScenarioPowerItemSpawn>[] = [
		{
			columns: [
				{ key: 'spawn_id', label: 'spawn_id', align: 'right' },
				{ key: 'tag', label: 'tag' },
				{ key: 'interval_ticks', label: 'interval_ticks', align: 'right' },
				{
					key: 'pos.x',
					label: 'pos.x',
					align: 'right',
					sortAccessor: (r) => r.pos?.x ?? null,
					format: (r) => fmtCoord(r.pos?.x)
				},
				{
					key: 'pos.y',
					label: 'pos.y',
					align: 'right',
					sortAccessor: (r) => r.pos?.y ?? null,
					format: (r) => fmtCoord(r.pos?.y)
				},
				{
					key: 'pos.z',
					label: 'pos.z',
					align: 'right',
					sortAccessor: (r) => r.pos?.z ?? null,
					format: (r) => fmtCoord(r.pos?.z)
				}
			]
		}
	];

	onMount(loadCollapsed);
</script>

{#snippet sectionHeader(id: string)}
	<span
		class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		<code class="font-mono">{id}</code>
	</span>
{/snippet}

{#snippet mapIcon()}<MapIcon class="size-3.5" />{/snippet}
{#snippet difficultyIcon()}<GamepadIcon class="size-3.5" />{/snippet}
{#snippet colorIcon()}<PaletteIcon class="size-3.5" />{/snippet}
{#snippet dropletIcon()}<DropletIcon class="size-3.5" />{/snippet}
{#snippet rulerIcon()}<RulerIcon class="size-3.5" />{/snippet}
{#snippet hardDriveIcon()}<HardDriveIcon class="size-3.5" />{/snippet}

{#snippet tile(label: string, f: FieldDisplay, icon?: import('svelte').Snippet)}
	<StatTile
		{label}
		display={f.display}
		statusKind={f.present ? 'on' : 'none'}
		title={f.raw}
		{icon}
	/>
{/snippet}

{#snippet memoryRegionTiles(name: string, region: MemoryRegionVm)}
	<div class="flex flex-col gap-2 card preset-tonal p-3">
		<div class="text-surface-700-200 text-xs font-semibold tracking-wide uppercase">
			<code class="font-mono">{name}</code>
		</div>
		<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
			{@render tile('base', region.base, hardDriveIcon)}
			{@render tile('size', region.size, rulerIcon)}
		</div>
	</div>
{/snippet}

{#snippet tagDefCard(entry: TagDefEntry)}
	{@const fields = tagDefFields(entry.def)}
	<div class="flex flex-col gap-2 card preset-tonal p-3">
		<div class="flex items-start justify-between gap-2">
			<code class="truncate font-mono text-xs font-semibold text-surface-900-100" title={entry.tag}>
				{entry.tag}
			</code>
			<span
				class="rounded bg-secondary-500/15 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-secondary-700-300 uppercase"
			>
				{entry.def.kind}
			</span>
		</div>
		<dl class="grid grid-cols-[max-content_1fr] gap-x-3 gap-y-1 text-xs">
			{#each fields.slice(1) as f, i (i)}
				<dt class="text-surface-700-200 font-mono">{f.label}</dt>
				<dd class="font-mono tabular-nums {f.present ? '' : 'text-surface-500-400'}">
					{f.value}
				</dd>
			{/each}
			{#if fields.length === 1}
				<dt class="text-surface-500-400 col-span-2 text-[11px] italic">no kind-specific fields</dt>
			{/if}
		</dl>
	</div>
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
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{@render tile('map', vm.topLevel.map, mapIcon)}
				{@render tile('game_difficulty', vm.topLevel.game_difficulty, difficultyIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="fog">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('fog')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
				{@render tile('color', vm.fog.color, colorIcon)}
				{@render tile('max_density', vm.fog.max_density, dropletIcon)}
				{@render tile('atmo_min_dist', vm.fog.atmo_min_dist, rulerIcon)}
				{@render tile('atmo_max_dist', vm.fog.atmo_max_dist, rulerIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="memory_regions">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('memory_regions')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 lg:grid-cols-2">
				{@render memoryRegionTiles('game_state', vm.memoryRegions.game_state)}
				{@render memoryRegionTiles('tag_cache', vm.memoryRegions.tag_cache)}
				{@render memoryRegionTiles('texture_cache', vm.memoryRegions.texture_cache)}
				{@render memoryRegionTiles('sound_cache', vm.memoryRegions.sound_cache)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="object_types">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('object_types')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.objectTypes}
				groups={objectTypeGroups}
				rowKey={(r) => r.type_index}
				defaultSort={{ key: 'type_index', dir: 'asc' }}
				secondarySort={{ key: 'name', dir: 'asc' }}
				emptyMessage="no object types in current scenario"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="player_spawns">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('player_spawns')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.playerSpawns}
				groups={playerSpawnGroups}
				rowKey={(r) => r.index}
				defaultSort={{ key: 'index', dir: 'asc' }}
				secondarySort={{ key: 'team_index', dir: 'asc' }}
				stickyFirst
				emptyMessage="no player spawns in current scenario"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="power_item_spawns">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('power_item_spawns')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.powerItemSpawns}
				groups={powerItemSpawnGroups}
				rowKey={(r) => r.spawn_id}
				defaultSort={{ key: 'spawn_id', dir: 'asc' }}
				secondarySort={{ key: 'tag', dir: 'asc' }}
				stickyFirst
				emptyMessage="no power-item spawns in current scenario"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="tag_defs">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('tag_defs')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{#if vm.tagDefs.length === 0}
				<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
					no tag definitions in current scenario
				</div>
			{:else}
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
					{#each vm.tagDefs as entry (entry.tag)}
						{@render tagDefCard(entry)}
					{/each}
				</div>
			{/if}
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
