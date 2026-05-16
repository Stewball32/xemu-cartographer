<script lang="ts">
	// Game tab — surfaces every field of the per-game GameData payload that
	// isn't already on the Tick tab. Section labels are rendered two ways:
	//   - real GameData JSON keys (team_scores, players, fog, …) render as
	//     monospace `code` so the operator can correlate UI to the wire
	//     shape in internal/scraper/types.go;
	//   - synthetic UI groupings render as plain title-case text so they
	//     can't be mistaken for wire fields. game_info is one such grouping
	//     — no wire field by that name; it bundles the top-level GameData
	//     scalars into themed FieldTiles (Settings / Gametype / Limits /
	//     Difficulty / Progress) so related wire fields share one card
	//     while still naming the JSON variable per row. The Progress tile
	//     is where live-moving values like engine_tick live.
	//
	// The `players` section is the merged home for everything roster-shaped:
	// the GamePlayer[] roster, the per-player stat roll-up, and the
	// team_scores payload (surfaced in each team's accordion header). It
	// renders the wire-key label `players` even though it carries more than
	// that one field — it's still rooted in GameData.players.
	//
	// Layout: single-column stack of <details> panels. Mobile-first; the
	// inner section components already provide their own responsive grids,
	// so wrapping them in narrower columns on desktop would just compress
	// player cards / table rows. <details> over Skeleton's Accordion because
	// every section opens independently and the native element keeps the
	// markup lean. Open/closed state persists to localStorage under
	// 'debug.game.collapsed' as a JSON array of collapsed section ids.

	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { ChevronDownIcon } from '@lucide/svelte';
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import { useDebugContext } from '../context.js';
	import { buildGameVm } from './game-vm';
	import GameInfoSection from './GameInfoSection.svelte';
	import PlayersSection from './PlayersSection.svelte';
	import SpawnsSection from './SpawnsSection.svelte';
	import FogSection from './FogSection.svelte';
	import PowerItemsSection from './PowerItemsSection.svelte';
	import MachinesSection from './MachinesSection.svelte';
	import ObjectTypesSection from './ObjectTypesSection.svelte';
	import TagCacheSection from './TagCacheSection.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildGameVm(name, scraperWSV2, ctx));

	type SectionId =
		| 'game_info'
		| 'players'
		| 'player_spawns'
		| 'fog'
		| 'power_item_spawns'
		| 'machines'
		| 'object_types'
		| 'tag_cache';

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

	const ALL_SECTIONS: readonly SectionId[] = [
		'game_info',
		'players',
		'player_spawns',
		'fog',
		'power_item_spawns',
		'machines',
		'object_types',
		'tag_cache'
	];

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

{#snippet panelHeader(id: string, subtitle?: string, label?: string)}
	<summary
		class="group/sum flex cursor-pointer list-none items-center justify-between gap-2 px-3 py-2 select-none hover:bg-surface-200-800/30 [&::-webkit-details-marker]:hidden"
	>
		<span class="inline-flex items-baseline gap-2 truncate">
			{#if label}
				<span class="text-xs font-semibold text-surface-900-100">{label}</span>
			{:else}
				<code class="font-mono text-xs font-semibold text-surface-900-100">{id}</code>
			{/if}
			{#if subtitle}
				<span class="text-surface-500-400 font-mono text-xs tabular-nums">{subtitle}</span>
			{/if}
		</span>
		<ChevronDownIcon
			class="text-surface-500-400 size-4 shrink-0 transition group-open/sum:rotate-180"
		/>
	</summary>
{/snippet}

{#if !vm.gameData}
	<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
		<span class="font-mono">game_data</span> — waiting for first read
	</div>
{:else}
	{@const gd = vm.gameData}
	<div class="mb-2 flex justify-end gap-2">
		<button type="button" class="btn preset-tonal btn-sm" onclick={() => setAll(true)}>
			Expand all
		</button>
		<button type="button" class="btn preset-tonal btn-sm" onclick={() => setAll(false)}>
			Collapse all
		</button>
	</div>

	<div class="flex flex-col gap-2">
		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('game_info')}
			ontoggle={(e) => onToggle('game_info', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('game_info', undefined, 'Game Info')}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<GameInfoSection showHeader={false} gameData={gd} tickValue={vm.tickValue} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('players')}
			ontoggle={(e) => onToggle('players', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('players', arrSub(gd.players))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<PlayersSection
					showHeader={false}
					playerTotalsByTeam={vm.playerTotalsByTeam}
					playerTotalsFlat={vm.playerTotalsFlat}
					scoreRows={vm.scoreRows}
					isTeamGame={vm.isTeamGame}
				/>
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('player_spawns')}
			ontoggle={(e) => onToggle('player_spawns', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('player_spawns', arrSub(gd.player_spawns))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<SpawnsSection showHeader={false} spawns={vm.playerSpawns} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('fog')}
			ontoggle={(e) => onToggle('fog', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('fog', objSub(gd.fog))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<FogSection showHeader={false} fog={vm.fog} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('power_item_spawns')}
			ontoggle={(e) => onToggle('power_item_spawns', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('power_item_spawns', arrSub(gd.power_item_spawns))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<PowerItemsSection showHeader={false} spawns={vm.powerItemSpawns} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('machines')}
			ontoggle={(e) => onToggle('machines', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('machines', arrSub(gd.machines))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<MachinesSection showHeader={false} machines={vm.machines} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('object_types')}
			ontoggle={(e) => onToggle('object_types', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('object_types', arrSub(gd.object_types))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<ObjectTypesSection showHeader={false} objectTypes={vm.objectTypes} />
			</div>
		</details>

		<details
			class="overflow-hidden card preset-tonal"
			open={!collapsedSections.has('tag_cache')}
			ontoggle={(e) => onToggle('tag_cache', (e.currentTarget as HTMLDetailsElement).open)}
		>
			{@render panelHeader('tag_cache', objSub(gd.tag_cache))}
			<div class="border-t border-surface-300-700/40 px-3 py-3">
				<TagCacheSection showHeader={false} tagCache={vm.tagCache} />
			</div>
		</details>
	</div>
{/if}
