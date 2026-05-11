<script lang="ts">
	// Tick tab — live snapshot of the per-poll TickPayload. Mirrors the
	// Accordion-of-sections pattern used by OverviewTab so the operator can
	// collapse what they don't need without losing place. Persistence:
	// `debug.tick.collapsed` in localStorage.
	//
	// Sections:
	//   - Engine flags   game_globals (map_loaded, active, …)
	//   - Locals         per-local sub-structs (fp_weapon, observer_cam, …)
	//   - Network        client / server / game_data / machines / network_players
	//   - Players        TickPlayer[] list → Dialog → PlayerDetailPanel
	//
	// Other TickPayload fields (power_items, data_queue, ctf_flags, objects,
	// projectiles) are surfaced in other tabs (Game / Events / Raw); the
	// Tick tab focuses on the bits that move per-tick.

	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import { buildTickVm } from './tick-vm';
	import EngineFlagsSection from './EngineFlagsSection.svelte';
	import LocalsSection from './LocalsSection.svelte';
	import NetworkSection from './NetworkSection.svelte';
	import PlayersRuntimeSection from './PlayersRuntimeSection.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildTickVm(name, scraperWS, ctx));

	type SectionId = 'engine_flags' | 'locals' | 'network' | 'players';

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

	// Sections always render, even when their slice of the payload is null —
	// each sub-component shows its own empty state so the operator knows the
	// scraper isn't currently surfacing that struct.
	const SECTIONS: SectionId[] = ['engine_flags', 'locals', 'network', 'players'];

	const openSections = $derived(SECTIONS.filter((id) => !collapsedSections.has(id)));

	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const id of SECTIONS) {
			if (!open.has(id)) newCollapsed.add(id);
		}
		collapsedSections = newCollapsed;
		saveCollapsed();
	}

	onMount(loadCollapsed);

	const localsCount = $derived(vm.tick?.locals?.length ?? 0);
	const playersCount = $derived(vm.tick?.players?.length ?? 0);
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

{#if !vm.tick}
	<div class="card preset-tonal p-3 text-sm">
		No tick payload yet — the scraper publishes ticks at ~30Hz during the live phase. Current phase:
		<span class="font-mono">{vm.phase}</span>.
	</div>
{:else}
	<div class="text-surface-700-200 mb-3 flex flex-wrap items-baseline gap-x-4 gap-y-1 text-xs">
		<span>
			engine tick:
			<span class="font-mono text-surface-950-50 tabular-nums">
				{vm.tickValue ?? '—'}
			</span>
		</span>
		<span>
			last update: <span class="font-mono">{vm.tickStr}</span>
		</span>
		<span>
			player_count:
			<span class="font-mono tabular-nums">{vm.tick.player_count}</span>
		</span>
		<span>
			local_count:
			<span class="font-mono tabular-nums">{vm.tick.local_count}</span>
		</span>
	</div>

	<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
		<Accordion.Item value="engine_flags">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Engine flags', vm.tick.game_globals ? undefined : 'null')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<EngineFlagsSection showHeader={false} gameGlobals={vm.tick.game_globals} />
			</Accordion.ItemContent>
		</Accordion.Item>

		<Accordion.Item value="locals">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Locals', `(${localsCount})`)}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<LocalsSection showHeader={false} locals={vm.tick.locals} />
			</Accordion.ItemContent>
		</Accordion.Item>

		<Accordion.Item value="network">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Network', vm.tick.network ? undefined : 'null')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<NetworkSection showHeader={false} network={vm.tick.network} />
			</Accordion.ItemContent>
		</Accordion.Item>

		<Accordion.Item value="players">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Players runtime', `(${playersCount})`)}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<PlayersRuntimeSection
					showHeader={false}
					tickPlayers={vm.tick.players}
					playersByIndex={vm.playersByIndex}
					teamGame={vm.isTeamGame}
				/>
			</Accordion.ItemContent>
		</Accordion.Item>
	</Accordion>
{/if}
