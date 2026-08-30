<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// NamePlate — the shared identity unit (CL-18). ONE 384×64 master (6:1),
	// scaled uniformly via `h` (all geometry ∝ h/64), so custom plate art maps
	// 1:1 wherever the plate appears. Neutral navy pill — NEVER team/armor
	// tinted; the host surface carries team color.
	//
	//   h         plate height (width locks to 6×h). 64 POV · 40 leaderboard ·
	//             28 post-game rows · 56 respawn ring · 30 marquee.
	//   showMotto false on small hosts (post-game rows, podium, ring).
	//   ghost     camo — the pill surface drops to a 22% ghost (border + bevel
	//             fade with it, 350ms); avatar/name/motto never cloak.
	//   os        shield value 0–3 — conic rings on the avatar well: red = OS
	//             layer (1–2), green = layer above (2–3), draining live off the
	//             ~30Hz shield feed. Nothing renders at shield ≤ 1.
	//   bg        optional 600×100 plate banner (profile field), edge to edge
	//             under a light legibility scrim. Custom-bg rendering carries
	//             the organizer-pack text treatment (white motto + ~2px
	//             near-black letter outline) so the organizer's banner-crop
	//             preview and the OBS output stay identical — that deviation is
	//             deliberate (chosen over a heavier directional scrim, which
	//             darkened the art too much).
	//   chromeOnly organizer crop-stage mode: the pill surface goes transparent
	//             (the live crop shows through from the stage beneath) while the
	//             scrim + avatar well + name/motto still draw at exact geometry.
	import starUrl from '$lib/assets/star.png';

	let {
		player = {},
		h = 64,
		showMotto = true,
		ghost = false,
		os = 0,
		bg = '',
		chromeOnly = false
	} = $props();

	const k = $derived(h / 64);
	const nameText = $derived(player.display || player.name || '—');
	const motto = $derived(showMotto ? player.motto || '' : '');

	// fitText — canvas measure in master-size units (name 22→14, motto 11.5→9,
	// then ellipsis). Re-measured once webfonts finish loading.
	const AVAIL = 384 - 4 - 56 - 14 - 24; // width − padL − avatar − gap − padR
	let fontsTick = $state(0);
	if (typeof document !== 'undefined') document.fonts?.ready?.then(() => fontsTick++);
	let ctx;
	function fit(text, max, min, font) {
		if (!text || typeof document === 'undefined') return max;
		ctx ??= document.createElement('canvas').getContext('2d');
		for (let s = max; s > min; s -= 0.5) {
			ctx.font = font(s);
			if (ctx.measureText(text).width <= AVAIL) return s;
		}
		return min;
	}
	const nameSize = $derived(
		(fontsTick, fit(nameText, 22, 14, (s) => `900 ${s}px Orbitron, sans-serif`))
	);
	const mottoSize = $derived(
		(fontsTick, fit(motto, 11.5, 9, (s) => `500 ${s}px Inter, sans-serif`))
	);

	const sh = $derived(+os || 0);
	const shR = $derived(Math.max(0, Math.min(1, sh - 1)));
	const shG = $derived(Math.max(0, Math.min(1, sh - 2)));
	const ringMask = $derived(
		`radial-gradient(farthest-side, transparent calc(100% - ${3.5 * k}px), #000 calc(100% - ${3 * k}px))`
	);
</script>

<div
	class="plate"
	class:ghost
	class:hasart={!!bg || chromeOnly}
	class:chrome={chromeOnly}
	style="height:{h}px; width:{h * 6}px; border-radius:{h / 2}px; padding:0 {24 * k}px 0 {4 *
		k}px; gap:{14 * k}px"
>
	{#if bg}
		<div class="bgart" style="background-image:url({bg}); border-radius:{h / 2}px"></div>
	{:else if chromeOnly}
		<div class="bgart" style="border-radius:{h / 2}px"></div>
	{/if}

	<div class="well" style="width:{56 * k}px; height:{56 * k}px">
		<div class="wellbg" style="--hp:{7 * k}px">
			{#if player.avatar}
				<img
					class="photo"
					src={player.avatar}
					alt=""
					onerror={(e) => (e.currentTarget.src = starUrl)}
				/>
			{:else}
				<img class="mark" src={starUrl} alt="" />
			{/if}
		</div>
		{#if shR > 0}
			<div
				class="ring"
				style="background:conic-gradient(from -90deg, #E05252 0deg {shR *
					360}deg, transparent {shR *
					360}deg 360deg); -webkit-mask:{ringMask}; mask:{ringMask}; filter:drop-shadow(0 0 {4 *
					k}px #E05252)"
			></div>
		{/if}
		{#if shG > 0}
			<div
				class="ring"
				style="background:conic-gradient(from -90deg, #4FD66A 0deg {shG *
					360}deg, transparent {shG *
					360}deg 360deg); -webkit-mask:{ringMask}; mask:{ringMask}; filter:drop-shadow(0 0 {4 *
					k}px #4FD66A)"
			></div>
		{/if}
	</div>

	<div class="id" style="gap:{3 * k}px">
		<span class="pname" style="font-size:{nameSize * k}px">{nameText}</span>
		{#if motto}<span class="motto" style="font-size:{mottoSize * k}px">{motto}</span>{/if}
	</div>
</div>

<style>
	.plate {
		position: relative;
		display: inline-flex;
		align-items: center;
		flex: none;
		box-sizing: border-box;
		background: linear-gradient(180deg, rgba(30, 38, 64, 0.9) 0%, rgba(13, 17, 33, 0.92) 82%);
		border: 1px solid rgba(255, 255, 255, 0.12);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.13);
		transition:
			background 350ms ease,
			border-color 350ms ease,
			box-shadow 350ms ease;
		overflow: hidden;
	}
	/* Camo: the banner/pill ghosts; content stays opaque. */
	.plate.ghost {
		background: rgba(13, 17, 33, 0.22);
		border-color: rgba(255, 255, 255, 0.06);
		box-shadow: none;
	}
	.plate.ghost .bgart {
		opacity: 0.16;
	}
	/* Crop-stage chrome: the live crop shows through from beneath; only the
	   scrim (.bgart with no image) + content draw. */
	.plate.chrome {
		background: none;
	}

	/* Custom-art text treatment (organizer-pack deviation, kept identical
	   between the organizer preview and OBS): white motto + layered ~2px
	   near-black letter outline on name AND motto, over the flat 0.22 scrim —
	   no extra darkening. Art-backed plates only; the plain navy pill keeps
	   the original CL-18 rendering. */
	.plate.hasart .pname,
	.plate.hasart .motto {
		text-shadow:
			2px 0 0 rgba(4, 6, 12, 0.85),
			-2px 0 0 rgba(4, 6, 12, 0.85),
			0 2px 0 rgba(4, 6, 12, 0.85),
			0 -2px 0 rgba(4, 6, 12, 0.85),
			1.5px 1.5px 0 rgba(4, 6, 12, 0.85),
			-1.5px 1.5px 0 rgba(4, 6, 12, 0.85),
			1.5px -1.5px 0 rgba(4, 6, 12, 0.85),
			-1.5px -1.5px 0 rgba(4, 6, 12, 0.85);
	}
	.plate.hasart .motto {
		color: #ffffff;
	}

	.bgart {
		position: absolute;
		inset: 0;
		background-size: cover;
		background-position: center;
		transition: opacity 350ms ease;
	}
	/* Light uniform legibility scrim over custom art. */
	.bgart::after {
		content: '';
		position: absolute;
		inset: 0;
		background: rgba(13, 17, 33, 0.22);
	}

	.well {
		position: relative;
		flex: none;
		z-index: 1;
	}
	.wellbg {
		position: absolute;
		inset: 0;
		border-radius: 50%;
		background: repeating-linear-gradient(
			45deg,
			#10152a 0 var(--hp),
			#141a30 var(--hp) calc(var(--hp) * 2)
		);
		border: 1px solid rgba(255, 255, 255, 0.12);
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		box-sizing: border-box;
	}
	.wellbg .photo {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.wellbg .mark {
		width: 100%;
		height: 100%;
		object-fit: contain;
		display: block;
		filter: drop-shadow(0 0 6px rgba(61, 98, 224, 0.6));
		animation: plate-twinkle 5s ease-in-out infinite;
	}
	.ring {
		position: absolute;
		inset: 0;
		border-radius: 50%;
	}

	.id {
		position: relative;
		z-index: 1;
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
	}
	.pname {
		font-family: Orbitron, sans-serif;
		font-weight: 900;
		line-height: 1.1;
		letter-spacing: 0.04em;
		color: #ffffff;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.motto {
		font-family: Inter, system-ui, sans-serif;
		font-weight: 500;
		line-height: 1.2;
		letter-spacing: 0.01em;
		color: #cdd9ea;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	@keyframes plate-twinkle {
		0%,
		100% {
			opacity: 0.75;
		}
		50% {
			opacity: 1;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.wellbg .mark {
			animation: none;
		}
	}
</style>
