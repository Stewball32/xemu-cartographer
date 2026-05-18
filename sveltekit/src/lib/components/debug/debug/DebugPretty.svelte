<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		ActivityIcon,
		ChevronDownIcon,
		CompassIcon,
		HashIcon,
		LayersIcon,
		ShieldIcon,
		TargetIcon,
		TimerIcon
	} from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { DebugPayload } from '$lib/types/scraper-v2';
	import StatTile from '$lib/components/debug/shared/StatTile.svelte';
	import {
		buildDebugPrettyVm,
		type BoneRow,
		type FieldDisplay,
		type NullableState,
		type PlayerVm
	} from './debug-vm';

	let { payload }: { payload: DebugPayload | null } = $props();

	const vm = $derived.by(() => buildDebugPrettyVm(payload));

	// Section ids match the JSON-shape group names so an operator who
	// collapses one here keeps it collapsed across reloads. Persisted
	// alongside the other tabs' collapse keys (debug.xbox.collapsed etc.).
	type SectionId = 'players' | 'state_inputs' | 'score_probe';
	const ALL_SECTIONS: readonly SectionId[] = ['players', 'state_inputs', 'score_probe'];

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.debug.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.debug.collapsed', JSON.stringify([...collapsedSections]));
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

	function nullableLabel(state: NullableState): string {
		switch (state) {
			case 'present':
				return 'present';
			case 'null':
				return 'null';
			default:
				return 'missing';
		}
	}

	function nullableKind(state: NullableState): 'on' | 'off' | 'none' {
		switch (state) {
			case 'present':
				return 'on';
			case 'null':
				return 'off';
			default:
				return 'none';
		}
	}

	function fmtKvValue(v: unknown): string {
		if (v === null) return 'null';
		if (v === undefined) return '—';
		if (typeof v === 'number') {
			if (!Number.isFinite(v)) return String(v);
			return Number.isInteger(v) ? String(v) : v.toFixed(4);
		}
		if (typeof v === 'boolean') return v ? 'true' : 'false';
		if (typeof v === 'string') return v;
		return JSON.stringify(v);
	}

	function kvKind(v: unknown): 'on' | 'off' | 'none' {
		if (v === null || v === undefined) return 'off';
		return 'on';
	}

	onMount(loadCollapsed);
</script>

{#snippet sectionHeader(id: string, count?: number)}
	<span
		class="text-surface-700-200 inline-flex items-center gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		<code class="font-mono">{id}</code>
		{#if count !== undefined}
			<span class="text-surface-500-400 normal-case">({count})</span>
		{/if}
	</span>
{/snippet}

{#snippet layersIcon()}<LayersIcon class="size-3.5" />{/snippet}
{#snippet compassIcon()}<CompassIcon class="size-3.5" />{/snippet}
{#snippet activityIcon()}<ActivityIcon class="size-3.5" />{/snippet}
{#snippet timerIcon()}<TimerIcon class="size-3.5" />{/snippet}
{#snippet shieldIcon()}<ShieldIcon class="size-3.5" />{/snippet}
{#snippet targetIcon()}<TargetIcon class="size-3.5" />{/snippet}

{#snippet tile(label: string, f: FieldDisplay, icon?: import('svelte').Snippet)}
	<StatTile
		{label}
		display={f.display}
		statusKind={f.present ? 'on' : 'none'}
		title={f.raw}
		{icon}
	/>
{/snippet}

{#snippet nullableTag(state: NullableState)}
	<StatTile
		label="status"
		display={nullableLabel(state)}
		statusKind={nullableKind(state)}
		title={`field is ${nullableLabel(state)}`}
	/>
{/snippet}

{#snippet bonesTable(rows: BoneRow[])}
	<div class="-mx-3 overflow-x-auto sm:mx-0">
		<table class="w-full text-xs">
			<thead>
				<tr class="bg-surface-100-900">
					<th class="px-2 py-1 text-left font-medium">index</th>
					<th class="px-2 py-1 text-left font-medium">pos.x</th>
					<th class="px-2 py-1 text-left font-medium">pos.y</th>
					<th class="px-2 py-1 text-left font-medium">pos.z</th>
				</tr>
			</thead>
			<tbody>
				{#each rows as row (row.index)}
					<tr class="border-t border-surface-200-800">
						<td class="px-2 py-1 align-top font-mono tabular-nums">{row.index}</td>
						<td class="px-2 py-1 align-top font-mono tabular-nums">{row.pos_x.display}</td>
						<td class="px-2 py-1 align-top font-mono tabular-nums">{row.pos_y.display}</td>
						<td class="px-2 py-1 align-top font-mono tabular-nums">{row.pos_z.display}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/snippet}

{#snippet kvTiles(entries: Array<[string, unknown]>)}
	{#if entries.length === 0}
		<div class="text-surface-500-400 text-sm">no entries</div>
	{:else}
		<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
			{#each entries as [k, v] (k)}
				<StatTile label={k} display={fmtKvValue(v)} statusKind={kvKind(v)} title={fmtKvValue(v)} />
			{/each}
		</div>
	{/if}
{/snippet}

{#snippet playerCard(p: PlayerVm)}
	<div class="card preset-tonal p-3">
		<div class="mb-3 flex items-center gap-2">
			<HashIcon class="text-surface-700-200 size-3.5" />
			<span class="text-surface-700-200 font-mono text-xs font-semibold uppercase">
				player[{p.index}]
			</span>
		</div>

		<div class="flex flex-col gap-3">
			<!-- extended -->
			<div class="flex flex-col gap-2">
				<div class="flex items-center gap-2">
					<code class="text-surface-700-200 font-mono text-xs font-semibold uppercase">
						extended
					</code>
					{@render nullableTag(p.extended.state)}
				</div>
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
					{@render tile('legs.pitch', p.extended.legs_pitch, compassIcon)}
					{@render tile('legs.yaw', p.extended.legs_yaw, compassIcon)}
					{@render tile('legs.roll', p.extended.legs_roll, compassIcon)}
					{@render tile('airborne', p.extended.airborne, activityIcon)}
					{@render tile('airborne_ticks', p.extended.airborne_ticks, timerIcon)}
					{@render tile('idle_ticks', p.extended.idle_ticks, timerIcon)}
					{@render tile('state_flags', p.extended.state_flags, layersIcon)}
					{@render tile('stunned', p.extended.stunned, shieldIcon)}
					{@render tile('aim_assist_sphere', p.extended.aim_assist_sphere, targetIcon)}
				</div>
			</div>

			<!-- bones -->
			<div class="flex flex-col gap-2">
				<div class="flex items-center gap-2">
					<code class="text-surface-700-200 font-mono text-xs font-semibold uppercase">
						bones
					</code>
					{@render nullableTag(p.bones.state)}
					{#if p.bones.state === 'present'}
						<span class="text-surface-500-400 font-mono text-xs">({p.bones.count})</span>
					{/if}
				</div>
				{#if p.bones.state === 'present'}
					{#if p.bones.rows.length > 0}
						{@render bonesTable(p.bones.rows)}
					{:else}
						<div class="text-surface-500-400 text-sm">empty array</div>
					{/if}
				{/if}
			</div>

			<!-- update_queue -->
			<div class="flex flex-col gap-2">
				<div class="flex items-center gap-2">
					<code class="text-surface-700-200 font-mono text-xs font-semibold uppercase">
						update_queue
					</code>
					{@render nullableTag(p.update_queue.state)}
				</div>
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
					{@render tile('buttons.crouch', p.update_queue.crouch, activityIcon)}
					{@render tile('buttons.jump', p.update_queue.jump, activityIcon)}
					{@render tile('buttons.fire', p.update_queue.fire, targetIcon)}
					{@render tile('desired_yaw', p.update_queue.desired_yaw, compassIcon)}
					{@render tile('desired_pitch', p.update_queue.desired_pitch, compassIcon)}
				</div>
			</div>

			<!-- raw -->
			<div class="flex flex-col gap-2">
				<div class="flex items-center gap-2">
					<code class="text-surface-700-200 font-mono text-xs font-semibold uppercase"> raw </code>
					{@render nullableTag(p.raw.state)}
					{#if p.raw.state === 'present'}
						<span class="text-surface-500-400 font-mono text-xs">({p.raw.entries.length})</span>
					{/if}
				</div>
				{#if p.raw.state === 'present'}
					{@render kvTiles(p.raw.entries)}
				{/if}
			</div>
		</div>
	</div>
{/snippet}

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="players">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('players', vm.players.count)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{#if vm.players.rows.length === 0}
				<div class="text-surface-500-400 text-sm">no players in payload</div>
			{:else}
				<div class="flex flex-col gap-3">
					{#each vm.players.rows as p (p.index)}
						{@render playerCard(p)}
					{/each}
				</div>
			{/if}
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="state_inputs">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('state_inputs', vm.state_inputs.count)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{@render kvTiles(vm.state_inputs.entries)}
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="score_probe">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render sectionHeader('score_probe', vm.score_probe.count)}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{@render kvTiles(vm.score_probe.entries)}
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
