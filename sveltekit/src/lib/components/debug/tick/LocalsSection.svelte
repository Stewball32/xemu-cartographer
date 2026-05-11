<script lang="ts">
	// Renders the locals[] array — one TickLocal per local splitscreen player.
	// Each local contains a fistful of sub-structs (fp_weapon, observer_cam,
	// ias, gamepad, ui, player_control) plus two scalar look-rate fields.
	//
	// To keep all of TickLocal visible without burying anything, we render one
	// card per local-index with an inner Accordion of the sub-struct groups,
	// each backed by a KvGrid. The scalar look_yaw_rate / look_pitch_rate
	// fields sit at the top of the card so the operator sees them without
	// expanding any sub-group.

	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import type { TickLocal } from '$lib/types/scraper';
	import KvGrid from '../shared/KvGrid.svelte';
	import { recordToFields } from './tick-vm';

	let {
		locals,
		annotationPrefix = 'tick.locals',
		showHeader = true
	}: {
		locals: TickLocal[] | null | undefined;
		annotationPrefix?: string;
		showHeader?: boolean;
	} = $props();

	// Per-local-index open-state for the inner sub-struct accordion. Mirrors
	// OverviewTab's collapsed-set pattern but keyed by "<localIndex>:<subKey>"
	// so each local card tracks its own expansion state independently.
	let openSubGroups = $state(new SvelteSet<string>());

	function subGroupKey(localIndex: number, subKey: string): string {
		return `${localIndex}:${subKey}`;
	}

	// One sub-struct per accordion item. The label drives the trigger text;
	// `key` matches the TickLocal field name so the annotation prefix is
	// stable (debug.tick.locals.<idx>.fp_weapon.<field>).
	const SUB_GROUPS: Array<{ key: keyof TickLocal; label: string }> = [
		{ key: 'fp_weapon', label: 'fp_weapon' },
		{ key: 'observer_cam', label: 'observer_cam' },
		{ key: 'ias', label: 'ias (input abstract)' },
		{ key: 'gamepad', label: 'gamepad' },
		{ key: 'ui', label: 'ui_globals' },
		{ key: 'player_control', label: 'player_control' }
	];

	const list = $derived(locals ?? []);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Locals
		</div>
	{/if}

	{#if list.length === 0}
		<div class="text-surface-500-400 card preset-tonal p-3 text-sm">no local players</div>
	{:else}
		<div class="grid gap-3 lg:grid-cols-2">
			{#each list as local (local.local_index)}
				{@const localPrefix = `${annotationPrefix}.${local.local_index}`}
				{@const openIds = SUB_GROUPS.map((g) => subGroupKey(local.local_index, g.key)).filter(
					(id) => openSubGroups.has(id)
				)}
				<div class="card preset-tonal p-3">
					<header class="mb-2 flex items-baseline gap-2">
						<span class="text-xs font-semibold tracking-wide uppercase">
							Local #{local.local_index}
						</span>
					</header>

					<KvGrid
						fields={[
							{
								key: 'look_yaw_rate',
								value: local.look_yaw_rate,
								annotationKey: `${localPrefix}.look_yaw_rate`
							},
							{
								key: 'look_pitch_rate',
								value: local.look_pitch_rate,
								annotationKey: `${localPrefix}.look_pitch_rate`
							}
						]}
						class="mb-2"
					/>

					<Accordion
						value={openIds}
						onValueChange={(e) => {
							const open = new Set(e.value);
							for (const g of SUB_GROUPS) {
								const id = subGroupKey(local.local_index, g.key);
								if (open.has(id)) openSubGroups.add(id);
								else openSubGroups.delete(id);
							}
						}}
						multiple
					>
						{#each SUB_GROUPS as group (group.key)}
							{@const sub = local[group.key] as Record<string, unknown> | null | undefined}
							{@const id = subGroupKey(local.local_index, group.key)}
							<Accordion.Item value={id}>
								<Accordion.ItemTrigger
									class="group flex w-full items-center justify-between gap-2 py-1.5 text-left text-xs"
								>
									<span class="flex items-baseline gap-2 font-mono">
										{group.label}
										{#if !sub}
											<span class="text-surface-500-400 normal-case">— null</span>
										{/if}
									</span>
									<Accordion.ItemIndicator>
										<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
									</Accordion.ItemIndicator>
								</Accordion.ItemTrigger>
								<Accordion.ItemContent class="pb-2">
									<KvGrid
										fields={recordToFields(sub, `${localPrefix}.${group.key}`)}
										emptyMessage="not populated on current tick"
									/>
								</Accordion.ItemContent>
							</Accordion.Item>
						{/each}
					</Accordion>
				</div>
			{/each}
		</div>
	{/if}
</section>
