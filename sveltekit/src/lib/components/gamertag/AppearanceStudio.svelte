<script lang="ts">
	// H2 appearance studio: an emblem creator (foreground / background / armor +
	// emblem colors / character) with a live emblem preview AND a live in-game
	// character preview. Writes the chosen values back into the H2 profile
	// `appearance` byte map (the route persists it; the server re-signs the save).
	//
	// Self-contained drop-in for H2AppearanceEditor — binds the same `appearance`
	// map, so the existing load()/save() path is unchanged.
	import { onMount } from 'svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import { ChevronDownIcon } from '@lucide/svelte';
	import EmblemPreview from './EmblemPreview.svelte';
	import CharacterPreview from './CharacterPreview.svelte';
	import RawSvg from './RawSvg.svelte';
	import {
		H2_COLORS,
		H2_BACKGROUNDS,
		H2_CHARACTERS,
		H2_KEYS,
		FOREGROUNDS,
		foregroundSvg,
		colorHex,
		colorName,
		readEmblem,
		seedEmblem,
		emblemUnset,
		DEFAULT_EMBLEM,
		type Appearance,
		type EmblemState
	} from '$lib/utils/emblem';
	import { backgroundSvg } from '$lib/utils/emblem-backgrounds';
	import type { H2AppearanceField } from '$lib/types/lansaves';

	let {
		fields = [],
		appearance = $bindable(),
		gamertag = ''
	}: {
		fields?: H2AppearanceField[];
		appearance: Appearance;
		gamertag?: string;
	} = $props();

	const FIELD_KEY: Record<keyof EmblemState, string> = {
		armorPrimary: H2_KEYS.armorPrimary,
		armorSecondary: H2_KEYS.armorSecondary,
		emblemPrimary: H2_KEYS.emblemPrimary,
		emblemSecondary: H2_KEYS.emblemSecondary,
		character: H2_KEYS.character,
		foreground: H2_KEYS.foreground,
		background: H2_KEYS.background
	};
	const MANAGED = new Set(Object.values(FIELD_KEY));

	// Seed a complete default emblem on first use so the preview is never blank
	// and a brand-new profile carries an intentional emblem.
	onMount(() => {
		if (!appearance) appearance = {};
		if (emblemUnset(appearance)) seedEmblem(appearance);
	});

	const emblem = $derived(readEmblem(appearance ?? {}, DEFAULT_EMBLEM));

	function select(field: keyof EmblemState, value: number) {
		if (!appearance) appearance = {};
		appearance[FIELD_KEY[field]] = value;
	}

	// armor colors drive the background thumbnails so they preview true-to-life.
	const armorA = $derived(colorHex(H2_COLORS, emblem.armorPrimary));
	const armorB = $derived(colorHex(H2_COLORS, emblem.armorSecondary));

	function fgThumb(i: number): string {
		return (
			`<rect x="0" y="0" width="100" height="100" rx="14" fill="#11161c"/>` +
			`<g transform="translate(13,13) scale(0.74)">${foregroundSvg(i, '#e9eef3', '#9fb4c6')}</g>`
		);
	}
	function bgThumb(i: number): string {
		return backgroundSvg(i, armorA, armorB);
	}

	// Advanced/raw bytes the pickers don't manage (flags, controller presets…).
	const advanced = $derived(fields.filter((f) => !MANAGED.has(f.key)));

	// Skeleton Accordion open-state for the advanced section (collapsed default).
	let advOpen = $state<string[]>([]);
</script>

<!-- Mobile-first + Skeleton: single column on phones; preview | controls split is
	a wide-screen (lg) enhancement. Chrome uses Skeleton theme tokens; the emblem /
	character ART surfaces stay deliberately dark to match the in-game aesthetic. -->
<div class="grid gap-5 lg:grid-cols-[minmax(240px,320px)_1fr] lg:items-start">
	<!-- live previews -->
	<div class="flex flex-col items-center gap-2 rounded-xl bg-surface-200-800/40 p-3">
		<div class="flex flex-wrap items-end justify-center gap-4">
			<div class="flex flex-col items-center gap-1.5">
				<EmblemPreview {appearance} size={144} ring title="Emblem preview" />
				<span class="text-surface-500-400 text-xs tracking-wide uppercase">Emblem</span>
			</div>
			<div class="flex flex-col items-center gap-1.5">
				<CharacterPreview game="h2" {appearance} {gamertag} size={184} />
			</div>
		</div>
		<p class="text-center text-xs leading-relaxed text-surface-600-400">
			<strong>{FOREGROUNDS[emblem.foreground]?.label}</strong> on
			<strong>{H2_BACKGROUNDS[emblem.background]?.label}</strong> ·
			{colorName(H2_COLORS, emblem.armorPrimary)}/{colorName(H2_COLORS, emblem.armorSecondary)} armor
			·
			{colorName(H2_COLORS, emblem.emblemPrimary)}/{colorName(H2_COLORS, emblem.emblemSecondary)} emblem
		</p>
	</div>

	<!-- controls -->
	<div class="flex flex-col gap-5">
		{#snippet colorRow(title: string, field: keyof EmblemState, selected: number)}
			<div class="flex flex-col gap-1.5">
				<div class="flex justify-between text-sm text-surface-700-300">
					<span>{title}</span><span class="text-surface-500-400 font-semibold"
						>{colorName(H2_COLORS, selected)}</span
					>
				</div>
				<div class="grid grid-cols-[repeat(auto-fill,minmax(2.25rem,1fr))] gap-1.5">
					{#each H2_COLORS as c, i (i)}
						<button
							type="button"
							class="aspect-square min-h-9 touch-manipulation rounded-md border border-surface-950/40 {i ===
							selected
								? 'outline-2 outline-offset-2 outline-primary-500'
								: ''}"
							style="background:{c.hex}"
							title={c.name}
							aria-label={c.name}
							aria-pressed={i === selected}
							onclick={() => select(field, i)}
						></button>
					{/each}
				</div>
			</div>
		{/snippet}

		{#snippet artCell(selected: boolean, label: string, inner: string, onpick: () => void)}
			<button
				type="button"
				class="aspect-square min-h-11 touch-manipulation overflow-hidden rounded-lg border bg-surface-900 {selected
					? 'border-transparent outline-2 outline-offset-1 outline-primary-500'
					: 'border-surface-700/40'}"
				title={label}
				aria-label={label}
				aria-pressed={selected}
				onclick={onpick}
			>
				<RawSvg {inner} />
			</button>
		{/snippet}

		<div class="flex flex-col gap-2.5">
			<h4 class="text-xs font-bold tracking-wide text-surface-600-400 uppercase">Armor colors</h4>
			{@render colorRow('Primary', 'armorPrimary', emblem.armorPrimary)}
			{@render colorRow('Secondary', 'armorSecondary', emblem.armorSecondary)}
		</div>

		<div class="flex flex-col gap-2.5">
			<h4 class="text-xs font-bold tracking-wide text-surface-600-400 uppercase">Emblem</h4>

			<div class="flex flex-col gap-1.5">
				<div class="flex justify-between text-sm text-surface-700-300">
					<span>Foreground symbol</span><span class="text-surface-500-400 font-semibold"
						>{FOREGROUNDS.length} symbols</span
					>
				</div>
				<!-- 64 symbols, contained + scrollable so it never dominates a phone -->
				<div
					class="grid max-h-[clamp(13rem,42vh,22rem)] grid-cols-[repeat(auto-fill,minmax(2.75rem,1fr))] gap-2 overflow-y-auto overscroll-contain rounded-lg bg-surface-200-800/40 p-1"
				>
					{#each FOREGROUNDS as f (f.index)}
						{@render artCell(emblem.foreground === f.index, f.label, fgThumb(f.index), () =>
							select('foreground', f.index)
						)}
					{/each}
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<div class="text-sm text-surface-700-300">Background</div>
				<div class="grid grid-cols-[repeat(auto-fill,minmax(2.75rem,1fr))] gap-2">
					{#each H2_BACKGROUNDS as bg (bg.index)}
						{@render artCell(emblem.background === bg.index, bg.label, bgThumb(bg.index), () =>
							select('background', bg.index)
						)}
					{/each}
				</div>
			</div>

			{@render colorRow('Emblem primary', 'emblemPrimary', emblem.emblemPrimary)}
			{@render colorRow('Emblem secondary', 'emblemSecondary', emblem.emblemSecondary)}
		</div>

		<div class="flex flex-col gap-2.5">
			<h4 class="text-xs font-bold tracking-wide text-surface-600-400 uppercase">Character</h4>
			<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
				{#each H2_CHARACTERS as ch (ch.index)}
					<button
						type="button"
						class="chip min-h-11 w-full justify-center {emblem.character === ch.index
							? 'preset-filled-primary-500'
							: 'preset-tonal'}"
						aria-pressed={emblem.character === ch.index}
						onclick={() => select('character', ch.index)}>{ch.label}</button
					>
				{/each}
			</div>
		</div>

		{#if advanced.length}
			<Accordion value={advOpen} onValueChange={(e) => (advOpen = e.value)} collapsible>
				<Accordion.Item value="advanced">
					<Accordion.ItemTrigger class="text-sm text-surface-700-300">
						<span>Advanced raw bytes ({advanced.length})</span>
						<Accordion.ItemIndicator>
							<ChevronDownIcon class="size-4" />
						</Accordion.ItemIndicator>
					</Accordion.ItemTrigger>
					<Accordion.ItemContent>
						<div class="grid grid-cols-[repeat(auto-fit,minmax(8rem,1fr))] gap-2.5 pt-1">
							{#each advanced as f (f.key)}
								<label class="label">
									<span class="text-xs"
										>{f.label}
										<span class="text-surface-500-400 font-mono">@0x{f.offset.toString(16)}</span
										></span
									>
									<input
										type="number"
										class="input"
										min="0"
										max="255"
										bind:value={appearance[f.key]}
										placeholder="—"
									/>
								</label>
							{/each}
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>
			</Accordion>
		{/if}

		<p class="text-surface-500-400 text-xs leading-relaxed">
			Emblem art is original, hand-drawn recreation (IP-safe). Field labels are confirmed against
			two real H2 profiles + the in-game enum order.
		</p>
	</div>
</div>
