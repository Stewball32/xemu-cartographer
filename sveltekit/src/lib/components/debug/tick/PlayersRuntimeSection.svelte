<script lang="ts">
	// Interactive per-player runtime drilldown — left side is a list of
	// `shared/PlayerListItem`s (one per TickPlayer), tap one to open a
	// full-screen Dialog (constrained on `md+`) carrying a PlayerDetailPanel
	// for that player.
	//
	// The list is purely a navigation surface; everything visible on the
	// detail side (position / velocity / aim / health / weapons / extended
	// substructs / bones) lives in PlayerDetailPanel.

	import type { GamePlayer, TickPlayer } from '$lib/types/scraper';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import PlayerListItem from '../shared/PlayerListItem.svelte';
	import PlayerDetailPanel from './PlayerDetailPanel.svelte';

	let {
		tickPlayers,
		playersByIndex,
		teamGame = false,
		showHeader = true
	}: {
		tickPlayers: TickPlayer[] | null | undefined;
		playersByIndex: Map<number, GamePlayer>;
		teamGame?: boolean;
		showHeader?: boolean;
	} = $props();

	const list = $derived(tickPlayers ?? []);

	// Selected player index (TickPlayer.index). null when no row is open.
	let selectedIndex = $state<number | null>(null);

	const selected = $derived.by<TickPlayer | null>(() => {
		if (selectedIndex === null) return null;
		return list.find((p) => p.index === selectedIndex) ?? null;
	});

	function openPlayer(idx: number) {
		selectedIndex = idx;
	}

	function closePlayer() {
		selectedIndex = null;
	}
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Players runtime
		</div>
	{/if}

	{#if list.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no tick.players</div>
	{:else}
		<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
			{#each list as tp (tp.index)}
				<PlayerListItem
					tickPlayer={tp}
					gamePlayer={playersByIndex.get(tp.index) ?? null}
					{teamGame}
					selected={selectedIndex === tp.index}
					onSelect={() => openPlayer(tp.index)}
				/>
			{/each}
		</div>
	{/if}
</section>

{#if selected}
	<Dialog open={selected !== null} onClose={closePlayer} size="lg">
		<PlayerDetailPanel
			tickPlayer={selected}
			gamePlayer={playersByIndex.get(selected.index) ?? null}
			{teamGame}
		/>
	</Dialog>
{/if}
