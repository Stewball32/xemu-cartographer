<script lang="ts">
	// Tick tab — live snapshot of the per-poll TickPayload. Section ids match
	// the TickPayload JSON keys (see internal/scraper/types.go) so the operator
	// can correlate UI to the wire shape — renaming would make it harder to
	// know what raw field a section corresponds to when debugging the scraper.
	// Every field of TickPayload has a home here; nothing falls back to Raw.
	//
	// Sections (mirroring TickPayload field order):
	//   game_globals   engine-wide flags (map_loaded, active, …)
	//   locals         per-local sub-structs (fp_weapon, observer_cam, …)
	//   network        client / server / game_data / machines / network_players
	//   players        TickPlayer[] list → Dialog → PlayerDetailPanel
	//   power_items    runtime PowerItemStatus[]
	//   ctf_flags      TickCTFFlag[] (CTF-only)
	//   data_queue     TickDataQueue scalar bag
	//   objects        TickObject[] — everything that isn't player/projectile/power item
	//   projectiles    TickProjectile[]
	//
	// Layout is a single-column stack of <details> panels (mobile-first). The
	// inner section components manage their own responsive grids, so wrapping
	// them in narrower outer columns on desktop would just compress tables.
	// Open/closed state persists to localStorage under 'debug.tick.collapsed'.

	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import { buildTickVm } from './tick-vm';
	import EngineFlagsSection from './EngineFlagsSection.svelte';
	import LocalsSection from './LocalsSection.svelte';
	import NetworkSection from './NetworkSection.svelte';
	import PlayersRuntimeSection from './PlayersRuntimeSection.svelte';
	import PowerItemsSection from './PowerItemsSection.svelte';
	import CTFFlagsSection from './CTFFlagsSection.svelte';
	import DataQueueSection from './DataQueueSection.svelte';
	import ObjectsSection from './ObjectsSection.svelte';
	import ProjectilesSection from './ProjectilesSection.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildTickVm(name, scraperWS, ctx));

	type SectionId =
		| 'game_globals'
		| 'locals'
		| 'network'
		| 'players'
		| 'power_items'
		| 'ctf_flags'
		| 'data_queue'
		| 'objects'
		| 'projectiles';

	const ALL_SECTIONS: readonly SectionId[] = [
		'game_globals',
		'locals',
		'network',
		'players',
		'power_items',
		'ctf_flags',
		'data_queue',
		'objects',
		'projectiles'
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

	function onToggle(id: SectionId, open: boolean) {
		if (open) collapsedSections.delete(id);
		else collapsedSections.add(id);
		saveCollapsed();
	}

	function setAll(open: boolean) {
		const next = new SvelteSet<string>();
		if (!open) {
			for (const id of ALL_SECTIONS) next.add(id);
		}
		collapsedSections = next;
		saveCollapsed();
	}

	// Subtitle helpers — surface array length / null-ness in the collapsed
	// header so the operator can tell at a glance whether the scraper is
	// reading a field. `[N]` for arrays, `null` for missing single records.
	function arrSub(arr: unknown[] | null | undefined): string {
		return `[${arr?.length ?? 0}]`;
	}
	function objSub(v: unknown): string {
		return v == null ? 'null' : '·';
	}

	onMount(loadCollapsed);
</script>

{#snippet panelHeader(id: string, subtitle?: string)}
	<summary
		class="group/sum flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2 select-none hover:bg-surface-200-800/30 [&::-webkit-details-marker]:hidden"
	>
		<span class="inline-flex items-baseline gap-2 truncate">
			<code class="font-mono text-xs font-semibold text-surface-900-100">{id}</code>
			{#if subtitle}
				<span class="text-surface-500-400 font-mono text-xs tabular-nums">{subtitle}</span>
			{/if}
		</span>
		<ChevronDownIcon
			class="text-surface-500-400 size-4 shrink-0 transition group-open/sum:rotate-180"
		/>
	</summary>
{/snippet}

{#if !vm.tick}
	<div class="card preset-tonal p-3 text-sm">
		<span class="font-mono">latest_tick</span> — no payload yet. The scraper publishes ticks at
		~30Hz during the live phase. Current phase: <span class="font-mono">{vm.phase}</span>.
	</div>
{:else}
	{@const tk = vm.tick}
	<div class="text-surface-600-300 mb-2 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-xs">
		<span class="font-mono">
			engine_tick:
			<span class="text-surface-900-100 tabular-nums">{vm.tickValue ?? '—'}</span>
		</span>
		<span class="font-mono">
			last_read: <span class="text-surface-900-100">{vm.tickStr}</span>
		</span>
		<span class="font-mono">
			player_count: <span class="text-surface-900-100 tabular-nums">{tk.player_count}</span>
		</span>
		<span class="font-mono">
			local_count: <span class="text-surface-900-100 tabular-nums">{tk.local_count}</span>
		</span>
		<span class="ml-auto flex gap-2">
			<button
				type="button"
				class="text-surface-500-400 font-mono text-[10px] uppercase hover:text-surface-900-100"
				onclick={() => setAll(true)}
			>
				expand all
			</button>
			<button
				type="button"
				class="text-surface-500-400 font-mono text-[10px] uppercase hover:text-surface-900-100"
				onclick={() => setAll(false)}
			>
				collapse all
			</button>
		</span>
	</div>

	<div class="flex flex-col gap-2">
		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('game_globals')}
			ontoggle={(e) => onToggle('game_globals', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('game_globals', objSub(tk.game_globals))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<EngineFlagsSection showHeader={false} gameGlobals={tk.game_globals} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('locals')}
			ontoggle={(e) => onToggle('locals', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('locals', arrSub(tk.locals))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<LocalsSection showHeader={false} locals={tk.locals} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('network')}
			ontoggle={(e) => onToggle('network', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('network', objSub(tk.network))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<NetworkSection showHeader={false} network={tk.network} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('players')}
			ontoggle={(e) => onToggle('players', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('players', arrSub(tk.players))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<PlayersRuntimeSection
					showHeader={false}
					tickPlayers={tk.players}
					playersByIndex={vm.playersByIndex}
					teamGame={vm.isTeamGame}
				/>
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('power_items')}
			ontoggle={(e) => onToggle('power_items', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('power_items', arrSub(tk.power_items))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<PowerItemsSection showHeader={false} powerItems={tk.power_items} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('ctf_flags')}
			ontoggle={(e) => onToggle('ctf_flags', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('ctf_flags', arrSub(tk.ctf_flags))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<CTFFlagsSection showHeader={false} flags={tk.ctf_flags} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('data_queue')}
			ontoggle={(e) => onToggle('data_queue', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('data_queue', objSub(tk.data_queue))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<DataQueueSection showHeader={false} dataQueue={tk.data_queue} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('objects')}
			ontoggle={(e) => onToggle('objects', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('objects', arrSub(tk.objects))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<ObjectsSection showHeader={false} objects={tk.objects} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('projectiles')}
			ontoggle={(e) => onToggle('projectiles', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('projectiles', arrSub(tk.projectiles))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<ProjectilesSection showHeader={false} projectiles={tk.projectiles} />
			</div>
		</details>
	</div>
{/if}
