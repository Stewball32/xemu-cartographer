<script lang="ts">
	// H2 appearance studio: an emblem creator (foreground / background / armor +
	// emblem colors / character) with a live emblem preview AND a live in-game
	// character preview. Writes the chosen values back into the H2 profile
	// `appearance` byte map (the route persists it; the server re-signs the save).
	//
	// Self-contained drop-in for H2AppearanceEditor — binds the same `appearance`
	// map, so the existing load()/save() path is unchanged.
	import { onMount } from 'svelte';
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
</script>

<div class="studio">
	<!-- live previews -->
	<div class="stage">
		<div class="stage-row">
			<div class="stage-item">
				<EmblemPreview {appearance} size={168} ring title="Emblem preview" />
				<span class="stage-cap">Emblem</span>
			</div>
			<div class="stage-item">
				<CharacterPreview game="h2" {appearance} {gamertag} size={208} />
			</div>
		</div>
		<p class="summary">
			<strong>{FOREGROUNDS[emblem.foreground]?.label}</strong> on
			<strong>{H2_BACKGROUNDS[emblem.background]?.label}</strong> ·
			{colorName(H2_COLORS, emblem.armorPrimary)}/{colorName(H2_COLORS, emblem.armorSecondary)} armor
			·
			{colorName(H2_COLORS, emblem.emblemPrimary)}/{colorName(H2_COLORS, emblem.emblemSecondary)} emblem
		</p>
	</div>

	<!-- controls -->
	<div class="controls">
		{#snippet colorRow(title: string, field: keyof EmblemState, selected: number)}
			<div class="ctl">
				<div class="ctl-head">
					<span>{title}</span><span class="ctl-val">{colorName(H2_COLORS, selected)}</span>
				</div>
				<div class="swatches">
					{#each H2_COLORS as c, i (i)}
						<button
							type="button"
							class="swatch"
							class:sel={i === selected}
							style="background:{c.hex}"
							title={c.name}
							aria-label={c.name}
							onclick={() => select(field, i)}
						></button>
					{/each}
				</div>
			</div>
		{/snippet}

		<div class="group">
			<h4 class="group-title">Armor colors</h4>
			{@render colorRow('Primary', 'armorPrimary', emblem.armorPrimary)}
			{@render colorRow('Secondary', 'armorSecondary', emblem.armorSecondary)}
		</div>

		<div class="group">
			<h4 class="group-title">Emblem</h4>

			<div class="ctl">
				<div class="ctl-head"><span>Foreground symbol</span></div>
				<div class="grid-pick">
					{#each FOREGROUNDS as f (f.index)}
						<button
							type="button"
							class="cell"
							class:sel={emblem.foreground === f.index}
							title={f.label}
							aria-label={f.label}
							onclick={() => select('foreground', f.index)}
						>
							<RawSvg inner={fgThumb(f.index)} />
						</button>
					{/each}
				</div>
			</div>

			<div class="ctl">
				<div class="ctl-head"><span>Background</span></div>
				<div class="grid-pick">
					{#each H2_BACKGROUNDS as bg (bg.index)}
						<button
							type="button"
							class="cell"
							class:sel={emblem.background === bg.index}
							title={bg.label}
							aria-label={bg.label}
							onclick={() => select('background', bg.index)}
						>
							<RawSvg inner={bgThumb(bg.index)} />
						</button>
					{/each}
				</div>
			</div>

			{@render colorRow('Emblem primary', 'emblemPrimary', emblem.emblemPrimary)}
			{@render colorRow('Emblem secondary', 'emblemSecondary', emblem.emblemSecondary)}
		</div>

		<div class="group">
			<h4 class="group-title">Character</h4>
			<div class="char-pick">
				{#each H2_CHARACTERS as ch (ch.index)}
					<button
						type="button"
						class="chip"
						class:sel={emblem.character === ch.index}
						onclick={() => select('character', ch.index)}>{ch.label}</button
					>
				{/each}
			</div>
		</div>

		{#if advanced.length}
			<details class="advanced">
				<summary>Advanced raw bytes ({advanced.length})</summary>
				<div class="adv-grid">
					{#each advanced as f (f.key)}
						<label class="label">
							<span class="text-xs"
								>{f.label}
								<span class="font-mono text-surface-500">@0x{f.offset.toString(16)}</span></span
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
			</details>
		{/if}

		<p class="note">
			Emblem art is original, hand-drawn recreation (IP-safe). Field labels are confirmed against
			two real H2 profiles + the in-game enum order.
		</p>
	</div>
</div>

<style>
	.studio {
		display: grid;
		gap: 1.25rem;
	}
	@media (min-width: 900px) {
		.studio {
			grid-template-columns: minmax(240px, 320px) 1fr;
		}
	}
	.stage {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		align-items: center;
		padding: 0.75rem;
		border-radius: 0.75rem;
		background: color-mix(in oklab, var(--color-surface-500) 8%, transparent);
	}
	.stage-row {
		display: flex;
		gap: 1.25rem;
		align-items: flex-end;
		justify-content: center;
		flex-wrap: wrap;
	}
	.stage-item {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.35rem;
	}
	.stage-cap {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		opacity: 0.6;
	}
	.summary {
		font-size: 0.78rem;
		text-align: center;
		opacity: 0.85;
		line-height: 1.4;
	}
	.controls {
		display: flex;
		flex-direction: column;
		gap: 1.1rem;
	}
	.group {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}
	.group-title {
		font-size: 0.75rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		opacity: 0.7;
		margin: 0;
	}
	.ctl {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.ctl-head {
		display: flex;
		justify-content: space-between;
		font-size: 0.78rem;
		opacity: 0.85;
	}
	.ctl-val {
		font-weight: 600;
		opacity: 0.7;
	}
	.swatches {
		display: grid;
		grid-template-columns: repeat(18, 1fr);
		gap: 3px;
	}
	@media (max-width: 560px) {
		.swatches {
			grid-template-columns: repeat(9, 1fr);
		}
	}
	.swatch {
		aspect-ratio: 1;
		border-radius: 4px;
		border: 1px solid rgba(0, 0, 0, 0.25);
		cursor: pointer;
		transition: transform 0.08s;
	}
	.swatch:hover {
		transform: scale(1.12);
	}
	.swatch.sel {
		outline: 2px solid var(--color-primary-500, #69f);
		outline-offset: 1px;
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.5);
	}
	.grid-pick {
		display: grid;
		grid-template-columns: repeat(8, 1fr);
		gap: 5px;
	}
	@media (max-width: 560px) {
		.grid-pick {
			grid-template-columns: repeat(6, 1fr);
		}
	}
	.cell {
		aspect-ratio: 1;
		padding: 0;
		border-radius: 8px;
		overflow: hidden;
		border: 1px solid rgba(255, 255, 255, 0.08);
		background: #11161c;
		cursor: pointer;
		transition: transform 0.08s;
	}
	.cell:hover {
		transform: scale(1.08);
	}
	.cell.sel {
		outline: 2px solid var(--color-primary-500, #69f);
		outline-offset: 1px;
		border-color: transparent;
	}
	.char-pick {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.chip {
		padding: 0.3rem 0.7rem;
		border-radius: 999px;
		font-size: 0.8rem;
		border: 1px solid rgba(255, 255, 255, 0.15);
		background: color-mix(in oklab, var(--color-surface-500) 12%, transparent);
		cursor: pointer;
	}
	.chip.sel {
		background: var(--color-primary-500, #4663ff);
		color: #fff;
		border-color: transparent;
	}
	.advanced {
		font-size: 0.85rem;
	}
	.advanced summary {
		cursor: pointer;
		opacity: 0.7;
	}
	.adv-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 0.6rem;
		margin-top: 0.6rem;
	}
	.note {
		font-size: 0.7rem;
		opacity: 0.55;
		line-height: 1.4;
	}
</style>
