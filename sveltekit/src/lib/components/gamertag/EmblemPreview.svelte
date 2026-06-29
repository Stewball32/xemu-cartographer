<script lang="ts">
	// Composes a Halo 2 emblem the way the game does: the background plate is
	// drawn in the two ARMOR colors (primary/secondary); the foreground symbol is
	// drawn inset on top in the two EMBLEM colors (tertiary/quaternary). The art
	// is the real extracted H2 emblem set (static/emblems), tinted via masks.
	import {
		H2_COLORS,
		colorHex,
		readEmblem,
		DEFAULT_EMBLEM,
		type Appearance,
		type EmblemState
	} from '$lib/utils/emblem';
	import { backgroundSvg } from '$lib/utils/emblem-backgrounds';
	import { foregroundSvg, EMBLEM_FG_INSET } from '$lib/utils/emblem-foregrounds';
	import RawSvg from './RawSvg.svelte';

	let {
		appearance,
		emblem,
		size = 160,
		rounded = true,
		ring = false,
		title = 'Emblem preview'
	}: {
		appearance?: Appearance;
		emblem?: EmblemState;
		size?: number;
		rounded?: boolean;
		ring?: boolean;
		title?: string;
	} = $props();

	// unique, hydration-stable prefix for this instance's mask ids
	const uid = $props.id();

	const state = $derived(emblem ?? readEmblem(appearance ?? {}, DEFAULT_EMBLEM));

	const armorA = $derived(colorHex(H2_COLORS, state.armorPrimary));
	const armorB = $derived(colorHex(H2_COLORS, state.armorSecondary));
	const emblemA = $derived(colorHex(H2_COLORS, state.emblemPrimary));
	const emblemB = $derived(colorHex(H2_COLORS, state.emblemSecondary));

	// The foreground is registered + centred by the compositor (its sprite rect is
	// mapped onto the cell). This inset is the in-game emblem-widget scale — the
	// symbol drawn inset over the plate — applied symmetrically so it stays centred.
	const fgInset = EMBLEM_FG_INSET;
	const fgOffset = (100 - 100 * fgInset) / 2;
	const inner = $derived(
		backgroundSvg(state.background, armorA, armorB, `${uid}b`) +
			`<g transform="translate(${fgOffset},${fgOffset}) scale(${fgInset})">${foregroundSvg(state.foreground, emblemA, emblemB, `${uid}f`)}</g>`
	);
</script>

<div
	class="emblem-frame"
	class:rounded
	class:ring
	style="width:{size}px;height:{size}px;"
	role="img"
	aria-label={title}
>
	<RawSvg {inner} />
</div>

<style>
	.emblem-frame {
		display: inline-block;
		overflow: hidden;
		line-height: 0;
		background: #0b0f14;
	}
	.emblem-frame.rounded {
		border-radius: 12%;
	}
	.emblem-frame.ring {
		box-shadow:
			0 0 0 1px rgba(255, 255, 255, 0.18),
			0 4px 14px rgba(0, 0, 0, 0.45);
	}
</style>
