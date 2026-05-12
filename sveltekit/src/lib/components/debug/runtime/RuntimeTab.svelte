<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import KvCard from '../shared/KvCard.svelte';
	import { buildRuntimeVm } from './runtime-vm';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildRuntimeVm(name, scraperWS, ctx));

	type SectionId = 'connection' | 'lifecycle' | 'summary' | 'freshness';

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.runtime.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.runtime.collapsed', JSON.stringify([...collapsedSections]));
		} catch {
			// ignore
		}
	}

	const visibleSections = $derived.by<SectionId[]>(() => {
		const visible: SectionId[] = ['connection', 'lifecycle'];
		if (vm.summary) visible.push('summary');
		visible.push('freshness');
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
	<Accordion.Item value="connection">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Connection', 'WebSocket transport')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<KvCard value={vm.connection} entriesSort="none" />
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="lifecycle">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Lifecycle', 'phase / uptime / counters')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<KvCard
				value={vm.lifecycle}
				emptyMessage="no current_state envelope yet"
				entriesSort="none"
			/>
		</Accordion.ItemContent>
	</Accordion.Item>

	{#if vm.summary}
		<Accordion.Item value="summary">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Cross-instance summary', 'this instance in host:all')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={vm.summary} entriesSort="none" />
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	<Accordion.Item value="freshness">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Envelope freshness', 'time since last receive')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<KvCard value={vm.freshness} entriesSort="none" />
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
