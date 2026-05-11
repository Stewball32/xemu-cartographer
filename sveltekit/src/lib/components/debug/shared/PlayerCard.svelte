<script lang="ts">
	import { Progress } from '@skeletonlabs/skeleton-svelte';
	import { HatGlassesIcon, SkullIcon } from '@lucide/svelte';
	import type { GamePlayer, TickPlayer } from '$lib/types/scraper';
	import Card from '$lib/components/ui/Card.svelte';
	import {
		armorAccent,
		armorLabel,
		fmtPct,
		fmtPctRaw,
		meterTone,
		overshieldLayers,
		pctClamped,
		shieldMeterTone
	} from './util';

	let {
		p,
		t,
		machineLabel,
		isTeam
	}: {
		p: GamePlayer;
		t: TickPlayer | undefined;
		machineLabel: (p: GamePlayer) => string | null;
		isTeam: boolean;
	} = $props();

	const machineSlug = $derived.by(() => {
		const m = p.machine_index;
		const c = p.controller_index;
		if (m == null && c == null) return null;
		const mPart = m != null ? `M${m}` : 'M?';
		const cPart = c != null ? `P${c}` : 'P?';
		return `${mPart}·${cPart}`;
	});

	const hp = $derived(pctClamped(t?.health));
	const layers = $derived(overshieldLayers(t?.shields));
	const armor = $derived(armorAccent(p.armor_color));

	// Per-card max-seen trackers so the ring fills proportionally to whatever
	// initial value the engine sets for the current death/camo session. Reset
	// when the corresponding state ends so a later session starts fresh.
	let maxRespawnSeen = $state(0);
	let maxCamoSeen = $state(0);

	$effect(() => {
		const r = t?.respawn_in_ticks;
		if (r != null && r > maxRespawnSeen) maxRespawnSeen = r;
		else if (r == null || r === 0) maxRespawnSeen = 0;
	});

	$effect(() => {
		const c = t?.camo_timer;
		if (c != null && c > maxCamoSeen) maxCamoSeen = c;
		else if (c == null || c === 0) maxCamoSeen = 0;
	});

	const statusKind = $derived.by<'dead' | 'camo' | 'idle'>(() => {
		if (t?.alive === false && (t?.respawn_in_ticks ?? 0) > 0) return 'dead';
		if (t?.alive === true && t?.has_camo === true && (t?.camo_timer ?? 0) > 0) return 'camo';
		return 'idle';
	});

	const ringValue = $derived.by(() => {
		if (statusKind === 'dead' && maxRespawnSeen > 0)
			return ((t?.respawn_in_ticks ?? 0) / maxRespawnSeen) * 100;
		if (statusKind === 'camo' && maxCamoSeen > 0) return ((t?.camo_timer ?? 0) / maxCamoSeen) * 100;
		return 0;
	});

	const ringRangeClass = $derived(
		statusKind === 'dead'
			? 'stroke-error-500'
			: statusKind === 'camo'
				? 'stroke-tertiary-500'
				: 'stroke-transparent'
	);
</script>

<Card size="sm">
	<header class="flex items-center gap-2">
		{#if t?.alive === true}
			<span class="size-2 rounded-full bg-success-500" title="alive"></span>
		{:else if t?.alive === false}
			<span class="size-2 rounded-full bg-error-500" title="dead"></span>
		{:else}
			<span class="size-2 rounded-full bg-surface-500" title="unknown"></span>
		{/if}
		{#if !isTeam}
			<span class="size-3 rounded-sm {armor.dot}" title={armorLabel(p.armor_color)}></span>
		{/if}
		<span class="min-w-0 flex-1 truncate text-sm font-semibold">{p.name || '—'}</span>
		<Progress class="relative size-8 shrink-0" value={ringValue}>
			<Progress.Circle class="size-8 [--size:--spacing(8)]">
				<Progress.CircleTrack class="stroke-surface-300-700" />
				<Progress.CircleRange class={ringRangeClass} />
			</Progress.Circle>
			{#if statusKind === 'dead'}
				<div class="pointer-events-none absolute inset-0 flex items-center justify-center">
					<SkullIcon class="size-3.5 text-error-500" />
				</div>
			{:else if statusKind === 'camo'}
				<div class="pointer-events-none absolute inset-0 flex items-center justify-center">
					<HatGlassesIcon class="size-3.5 text-tertiary-500" />
				</div>
			{/if}
		</Progress>
	</header>
	<article class="my-2 space-y-2">
		<div class="text-surface-700-200 flex flex-wrap gap-x-3 text-[11px]">
			<span>S <span class="font-mono text-surface-950-50 tabular-nums">{p.score}</span></span>
			<span>K <span class="font-mono text-surface-950-50 tabular-nums">{p.kills}</span></span>
			<span>D <span class="font-mono text-surface-950-50 tabular-nums">{p.deaths}</span></span>
			<span>A <span class="font-mono text-surface-950-50 tabular-nums">{p.assists}</span></span>
			<span>KS <span class="font-mono text-surface-950-50 tabular-nums">{p.kill_streak}</span></span
			>
		</div>
		<div class="space-y-1">
			<div class="flex items-center gap-2 text-[10px]">
				<span class="text-surface-700-200 w-8 uppercase">hp</span>
				<div class="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-300-700">
					<div class="h-full {meterTone(hp)}" style="width: {hp}%"></div>
				</div>
				<span class="w-10 text-right font-mono tabular-nums">{fmtPct(t?.health)}</span>
			</div>
			<div class="flex items-center gap-2 text-[10px]">
				<span class="text-surface-700-200 w-8 uppercase">sh</span>
				<div class="relative h-2.5 flex-1 overflow-hidden rounded-full bg-surface-300-700">
					<div
						class="absolute inset-y-0 left-0 {shieldMeterTone(layers.a)}"
						style="width: {layers.a}%"
					></div>
					{#if layers.b > 0}
						<div class="absolute inset-y-0 left-0 bg-error-500" style="width: {layers.b}%"></div>
					{/if}
					{#if layers.c > 0}
						<div class="absolute inset-y-0 left-0 bg-success-500" style="width: {layers.c}%"></div>
					{/if}
				</div>
				<span class="w-10 text-right font-mono tabular-nums">{fmtPctRaw(t?.shields)}</span>
			</div>
		</div>
	</article>
	{#if machineSlug || machineLabel(p)}
		<footer class="text-surface-700-200 flex items-center justify-between gap-2 text-[10px]">
			<span class="tabular-nums">{machineSlug ?? ''}</span>
			<span class="min-w-0 truncate text-right">{machineLabel(p) ?? ''}</span>
		</footer>
	{/if}
</Card>
