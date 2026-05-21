<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { BoxesIcon, ChevronDownIcon, RocketIcon } from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { ObjectsPayload } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup } from '$lib/components/ui/data-table';
	import { buildObjectsPrettyVm, type ObjectRow, type ProjectileRow } from './objects-vm';

	let { payload }: { payload: ObjectsPayload | null } = $props();

	const vm = $derived.by(() => buildObjectsPrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses 'objects' here keeps it collapsed across reloads. Persisted
	// alongside the other tabs' collapse keys (debug.xbox.collapsed etc.).
	type SectionId = 'summary' | 'objects' | 'projectiles';
	const ALL_SECTIONS: readonly SectionId[] = ['summary', 'objects', 'projectiles'];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.objects.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.objects.collapsed', JSON.stringify([...collapsedSections]));
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

	// DataTable column groups for the world-objects array. pos.x/y/z and
	// ang_vel.x/y/z are split into per-axis cells so each axis sorts
	// independently. Hex IDs sort on their underlying *_raw uint so asc/desc
	// ordering stays numerically correct (lex sort would put 0x1F0 after 0x1FF).
	const objectGroups: DataColumnGroup<ObjectRow>[] = [
		{
			label: 'Object',
			columns: [
				{ key: 'object_id', label: 'object_id', sortAccessor: (r) => r.object_id_raw },
				{ key: 'tag', label: 'tag' },
				{ key: 'type', label: 'type', align: 'right' },
				{ key: 'flags', label: 'flags', sortAccessor: (r) => r.flags_raw }
			]
		},
		{
			label: 'pos',
			columns: [
				{ key: 'pos_x', label: 'x', align: 'right' },
				{ key: 'pos_y', label: 'y', align: 'right' },
				{ key: 'pos_z', label: 'z', align: 'right' }
			]
		},
		{
			label: 'ang_vel',
			columns: [
				{ key: 'ang_vel_x', label: 'x', align: 'right' },
				{ key: 'ang_vel_y', label: 'y', align: 'right' },
				{ key: 'ang_vel_z', label: 'z', align: 'right' }
			]
		},
		{
			label: 'lifetime',
			columns: [{ key: 'time_existing', label: 'time_existing', align: 'right' }]
		},
		{
			label: 'ownership',
			columns: [
				{ key: 'owner_unit', label: 'owner_unit', sortAccessor: (r) => r.owner_unit_raw },
				{ key: 'owner_object', label: 'owner_object', sortAccessor: (r) => r.owner_object_raw },
				{
					key: 'ultimate_parent',
					label: 'ultimate_parent',
					sortAccessor: (r) => r.ultimate_parent_raw
				}
			]
		}
	];

	const projectileGroups: DataColumnGroup<ProjectileRow>[] = [
		{
			label: 'Projectile',
			columns: [
				{ key: 'object_id', label: 'object_id', sortAccessor: (r) => r.object_id_raw },
				{ key: 'tag', label: 'tag' }
			]
		},
		{
			label: 'pos',
			columns: [
				{ key: 'pos_x', label: 'x', align: 'right' },
				{ key: 'pos_y', label: 'y', align: 'right' },
				{ key: 'pos_z', label: 'z', align: 'right' }
			]
		},
		{
			label: 'state',
			columns: [
				{ key: 'flags', label: 'flags', sortAccessor: (r) => r.flags_raw },
				{ key: 'action', label: 'action', align: 'right' },
				{ key: 'detonation_timer', label: 'detonation_timer', align: 'right' },
				{ key: 'distance_traveled', label: 'distance_traveled', align: 'right' }
			]
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

{#snippet boxesIcon()}<BoxesIcon class="size-3.5" />{/snippet}
{#snippet rocketIcon()}<RocketIcon class="size-3.5" />{/snippet}

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="summary">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('summary')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				<StatTile
					label="objects"
					display={String(vm.objectCount)}
					statusKind={vm.objectCount > 0 ? 'on' : 'none'}
					title="world-object array length"
					icon={boxesIcon}
				/>
				<StatTile
					label="projectiles"
					display={String(vm.projectileCount)}
					statusKind={vm.projectileCount > 0 ? 'on' : 'none'}
					title="projectile array length"
					icon={rocketIcon}
				/>
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="objects">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('objects')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.objects}
				groups={objectGroups}
				rowKey={(r) => r.object_id_raw}
				defaultSort={{ key: 'object_id', dir: 'asc' }}
				secondarySort={{ key: 'tag', dir: 'asc' }}
				emptyMessage="no objects in this envelope"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="projectiles">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('projectiles')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<DataTable
				rows={vm.projectiles}
				groups={projectileGroups}
				rowKey={(r) => r.object_id_raw}
				defaultSort={{ key: 'object_id', dir: 'asc' }}
				secondarySort={{ key: 'tag', dir: 'asc' }}
				emptyMessage="no projectiles in this envelope"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
