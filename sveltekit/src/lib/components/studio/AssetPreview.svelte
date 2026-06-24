<script lang="ts">
	import type { AssetAspect, AssetKind } from '$lib/config/stream-assets';

	// A live, inline preview of a stream asset. Renders the REAL route in an
	// iframe pointed at its ?mock=1 URL (no game, no token needed), at a fixed
	// 1280×720 logical canvas scaled down to the card width — so the miniature is
	// true-to-OBS rather than the page rendering at card-pixel sizes. Overlay
	// assets sit on a checkerboard so the transparent background reads as
	// transparent; scenes sit on black (they carry their own background).

	let {
		src,
		aspect,
		kind,
		title
	}: { src: string; aspect: AssetAspect; kind: AssetKind; title: string } = $props();

	// Logical canvas the asset is designed against (16:9 1080p, scaled here).
	const BASE_W = 1280;
	const baseH = $derived(Math.round((BASE_W * aspect.h) / aspect.w));

	let stageW = $state(0);
	const scale = $derived(stageW > 0 ? stageW / BASE_W : 0);
</script>

<div
	class="stage"
	class:checker={kind === 'overlay'}
	style="aspect-ratio: {aspect.w} / {aspect.h}"
	bind:clientWidth={stageW}
>
	{#if scale > 0}
		<iframe
			{title}
			{src}
			loading="lazy"
			tabindex="-1"
			scrolling="no"
			style="width: {BASE_W}px; height: {baseH}px; transform: scale({scale});"
		></iframe>
	{/if}
</div>

<style>
	.stage {
		position: relative;
		width: 100%;
		overflow: hidden;
		border-radius: 0.4rem;
		background: #05070c;
	}
	/* Transparent-overlay backdrop: a subtle checkerboard so the (transparent)
	   overlay reads as composited, not as a solid panel. */
	.stage.checker {
		background-color: #0c1018;
		background-image:
			linear-gradient(45deg, rgba(255, 255, 255, 0.05) 25%, transparent 25%),
			linear-gradient(-45deg, rgba(255, 255, 255, 0.05) 25%, transparent 25%),
			linear-gradient(45deg, transparent 75%, rgba(255, 255, 255, 0.05) 75%),
			linear-gradient(-45deg, transparent 75%, rgba(255, 255, 255, 0.05) 75%);
		background-size: 18px 18px;
		background-position:
			0 0,
			0 9px,
			9px -9px,
			-9px 0;
	}
	iframe {
		position: absolute;
		top: 0;
		left: 0;
		border: 0;
		transform-origin: top left;
		/* Decorative miniature — block interaction so clicks reach the card. */
		pointer-events: none;
	}
</style>
