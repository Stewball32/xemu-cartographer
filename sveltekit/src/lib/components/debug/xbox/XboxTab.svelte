<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import KvCard from '../shared/KvCard.svelte';
	import { buildXboxVm } from './xbox-vm';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildXboxVm(name, scraperWS, ctx.inspect));

	type SectionId = 'identity' | 'xbe_cert' | 'eeprom' | 'kernel_clock';

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

	const visibleSections = $derived.by<SectionId[]>(() => {
		const visible: SectionId[] = ['identity'];
		if (vm.xbeCert) visible.push('xbe_cert');
		if (vm.eeprom) visible.push('eeprom');
		if (vm.kernelClock) visible.push('kernel_clock');
		return visible;
	});

	const openSections = $derived(visibleSections.filter((id) => !collapsedSections.has(id)));

	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const id of visibleSections) {
			if (!open.has(id)) newCollapsed.add(id);
		}
		collapsedSections = newCollapsed;
		saveCollapsed();
	}

	onMount(loadCollapsed);
</script>

{#snippet trigger(title: string, subtitle?: string)}
	<span
		class="text-surface-700-200 inline-flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		{title}
		{#if subtitle}
			<span class="text-surface-500-400 font-normal normal-case">{subtitle}</span>
		{/if}
	</span>
{/snippet}

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="identity">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Identity', 'title / xbe / console name')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<KvCard
				value={vm.identity}
				emptyMessage="no current_state envelope yet — waiting for first read"
				entriesSort="none"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	{#if vm.xbeCert}
		<Accordion.Item value="xbe_cert">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('XBE certificate', 'on-disk image metadata')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={vm.xbeCert} entriesSort="none" />
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	{#if vm.eeprom}
		<Accordion.Item value="eeprom">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('EEPROM', 'serial / mac / region / time zone')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={vm.eeprom} entriesSort="none" />
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	{#if vm.kernelClock}
		<Accordion.Item value="kernel_clock">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Kernel clock', 'KeSystemTime / KeInterruptTime')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={vm.kernelClock} entriesSort="none" />
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}
</Accordion>
