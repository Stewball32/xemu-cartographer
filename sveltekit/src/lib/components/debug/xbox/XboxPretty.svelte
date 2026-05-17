<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		ChevronDownIcon,
		ClockIcon,
		DiscIcon,
		FileTextIcon,
		Gamepad2Icon,
		GlobeIcon,
		HardDriveIcon,
		HashIcon,
		MonitorIcon,
		PowerIcon,
		ServerIcon,
		TimerIcon,
		WifiIcon
	} from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { XboxPayload } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import { buildXboxPrettyVm, type FieldDisplay } from './xbox-vm';

	let { payload }: { payload: XboxPayload | null } = $props();

	const vm = $derived.by(() => buildXboxPrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses 'xbe' here keeps it collapsed across reloads. Persisted
	// alongside the other tabs' collapse keys (debug.game.collapsed etc.).
	type SectionId = 'top-level' | 'time_zone' | 'xbe' | 'kernel';
	const ALL_SECTIONS: readonly SectionId[] = ['top-level', 'time_zone', 'xbe', 'kernel'];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.xbox.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.xbox.collapsed', JSON.stringify([...collapsedSections]));
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
</script>

{#snippet sectionHeader(id: string)}
	<span
		class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		<code class="font-mono">{id}</code>
	</span>
{/snippet}

{#snippet titleIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}
{#snippet serverIcon()}<ServerIcon class="size-3.5" />{/snippet}
{#snippet wifiIcon()}<WifiIcon class="size-3.5" />{/snippet}
{#snippet monitorIcon()}<MonitorIcon class="size-3.5" />{/snippet}
{#snippet globeIcon()}<GlobeIcon class="size-3.5" />{/snippet}
{#snippet discIcon()}<DiscIcon class="size-3.5" />{/snippet}
{#snippet hardDriveIcon()}<HardDriveIcon class="size-3.5" />{/snippet}
{#snippet fileIcon()}<FileTextIcon class="size-3.5" />{/snippet}
{#snippet clockIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet powerIcon()}<PowerIcon class="size-3.5" />{/snippet}
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
				{@render tile('title', vm.topLevel.title, titleIcon)}
				{@render tile('title_id', vm.topLevel.title_id, hashIcon)}
				{@render tile('name', vm.topLevel.name, serverIcon)}
				{@render tile('serial_number', vm.topLevel.serial_number, hashIcon)}
				{@render tile('mac_address', vm.topLevel.mac_address, wifiIcon)}
				{@render tile('video_standard', vm.topLevel.video_standard, monitorIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="time_zone">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('time_zone')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
				{@render tile('bias_minutes', vm.timeZone.bias_minutes, globeIcon)}
				{@render tile('std_name', vm.timeZone.std_name, globeIcon)}
				{@render tile('dlt_name', vm.timeZone.dlt_name, globeIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="xbe">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('xbe')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
				{@render tile('title_name', vm.xbe.title_name, fileIcon)}
				{@render tile('version', vm.xbe.version, hashIcon)}
				{@render tile('game_region', vm.xbe.game_region, globeIcon)}
				{@render tile('disk_number', vm.xbe.disk_number, discIcon)}
				{@render tile('allowed_media', vm.xbe.allowed_media, hardDriveIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="kernel">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('kernel')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 lg:grid-cols-3">
				{@render tile('system_time', vm.kernel.system_time, clockIcon)}
				{@render tile('boot_time', vm.kernel.boot_time, powerIcon)}
				{@render tile('uptime_seconds', vm.kernel.uptime_seconds, timerIcon)}
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
