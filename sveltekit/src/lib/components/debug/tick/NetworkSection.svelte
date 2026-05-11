<script lang="ts">
	// TickNetwork: per-tick network snapshot — split into the four
	// sub-objects it carries on the wire so each one has a clear home.
	//
	//   - client (machine_index, ping, packets, etc.)
	//   - server (countdown)
	//   - game_data (player / machine counts)
	//   - machines[] (table by machine index)
	//   - network_players[] (table by player list index)

	import type { TickNetwork } from '$lib/types/scraper';
	import KvGrid from '../shared/KvGrid.svelte';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { ColGroup } from '../shared/col-grouped-table';
	import { recordToFields } from './tick-vm';

	let {
		network,
		annotationPrefix = 'tick.network',
		showHeader = true
	}: {
		network: TickNetwork | null | undefined;
		annotationPrefix?: string;
		showHeader?: boolean;
	} = $props();

	const clientFields = $derived(recordToFields(network?.client, `${annotationPrefix}.client`));
	const serverFields = $derived(recordToFields(network?.server, `${annotationPrefix}.server`));
	const gameDataFields = $derived(
		recordToFields(network?.game_data, `${annotationPrefix}.game_data`)
	);

	// Cast through unknown so the typed scraper structs (TickNetMachine, ...)
	// satisfy ColGroupedTable's open-ended Record<string, unknown> rows prop.
	const machineRows = $derived(
		(network?.machines ?? []) as unknown as ReadonlyArray<Record<string, unknown>>
	);
	const networkPlayerRows = $derived(
		(network?.network_players ?? []) as unknown as ReadonlyArray<Record<string, unknown>>
	);

	const machinesGroups: ColGroup[] = [
		{
			label: 'machine',
			columns: [
				{ key: 'index', label: 'idx' },
				{ key: 'name', label: 'name' }
			]
		}
	];

	const playersGroups: ColGroup[] = [
		{
			label: 'identity',
			columns: [
				{ key: 'list_index', label: 'list' },
				{ key: 'name', label: 'name' },
				{ key: 'team', label: 'team' }
			]
		},
		{
			label: 'routing',
			columns: [
				{ key: 'machine_index', label: 'machine' },
				{ key: 'controller_index', label: 'ctrl' }
			]
		},
		{
			label: 'misc',
			columns: [
				{ key: 'color', label: 'color' },
				{ key: 'unused', label: 'unused' }
			]
		}
	];
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Network
		</div>
	{/if}

	{#if !network}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
			no network struct on current tick
		</div>
	{:else}
		<div class="grid gap-3 md:grid-cols-3">
			<div class="card preset-tonal p-3">
				<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
					client
				</header>
				<KvGrid fields={clientFields} emptyMessage="no client struct" />
			</div>
			<div class="card preset-tonal p-3">
				<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
					server
				</header>
				<KvGrid fields={serverFields} emptyMessage="no server struct" />
			</div>
			<div class="card preset-tonal p-3">
				<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
					game_data
				</header>
				<KvGrid fields={gameDataFields} emptyMessage="no game_data struct" />
			</div>
		</div>

		<div class="mt-3">
			<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
				machines ({network.machines?.length ?? 0})
			</header>
			<ColGroupedTable
				rows={machineRows}
				groups={machinesGroups}
				emptyMessage="no machines"
				annotationPrefix="{annotationPrefix}.machines"
			/>
		</div>

		<div class="mt-3">
			<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
				network_players ({network.network_players?.length ?? 0})
			</header>
			<ColGroupedTable
				rows={networkPlayerRows}
				groups={playersGroups}
				emptyMessage="no network_players"
				annotationPrefix="{annotationPrefix}.network_players"
			/>
		</div>
	{/if}
</section>
