<script lang="ts">
	// Composes a Halo 2 emblem the way the game does: the background plate is
	// drawn in the two ARMOR colors (primary/secondary); the foreground symbol is
	// drawn inset on top in the two EMBLEM colors (tertiary/quaternary).
	import {
		H2_COLORS,
		colorHex,
		readEmblem,
		DEFAULT_EMBLEM,
		type Appearance,
		type EmblemState
	} from '$lib/utils/emblem';
	import { backgroundSvg } from '$lib/utils/emblem-backgrounds';
	import { foregroundSvg } from '$lib/utils/emblem-foregrounds';
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

	const state = $derived(emblem ?? readEmblem(appearance ?? {}, DEFAULT_EMBLEM));

	const armorA = $derived(colorHex(H2_COLORS, state.armorPrimary));
	const armorB = $derived(colorHex(H2_COLORS, state.armorSecondary));
	const emblemA = $derived(colorHex(H2_COLORS, state.emblemPrimary));
	const emblemB = $derived(colorHex(H2_COLORS, state.emblemSecondary));

	const inner = $derived(
		backgroundSvg(state.background, armorA, armorB) +
			`<g transform="translate(11,11) scale(0.78)">${foregroundSvg(state.foreground, emblemA, emblemB)}</g>`
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
