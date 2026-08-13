<!--
	StarfieldBackground.svelte — animated Star Command backdrop.

	Svelte port of the NorCal Halo UnleashX BG.wmv loop: three parallax star
	layers, twinklers, shooting stars, breathing nebula glows, and (optionally)
	the floating star emblem with glow + type lockup.

	Mount once, full-viewport, behind the app (see README). Colors pull from
	the active Skeleton theme (secondary = blue glow, primary = warm glow), so
	it also looks right under other dark themes. Honors prefers-reduced-motion.

	Props:
	  logo    — URL of the text-free star emblem PNG (null = no logo block)
	  lockup  — show the NORCAL / HALO type under the logo (default true)
	  dim     — 0..1 extra dimmer over the whole field (default 0)
-->
<script lang="ts">
	let {
		logo = null,
		lockup = true,
		dim = 0
	}: { logo?: string | null; lockup?: boolean; dim?: number } = $props();

	const twinkles = [
		{ x: 15, y: 18, s: 2.5, d: 2.8, delay: 0 },
		{ x: 53, y: 11, s: 2, d: 3.7, delay: 1.1 },
		{ x: 81, y: 37, s: 2.5, d: 4.4, delay: 0.6, warm: true },
		{ x: 30, y: 69, s: 2, d: 3.2, delay: 2 },
		{ x: 70, y: 83, s: 2, d: 4, delay: 2.8 },
		{ x: 9, y: 90, s: 2, d: 3.5, delay: 1.6 }
	];
</script>

<div class="starfield" aria-hidden="true">
	<div class="layer far"></div>
	<div class="layer mid"></div>
	<div class="layer near"></div>
	<!-- Keyed on position: `twinkles` is a static const, so any stable unique
	     key works — this repo's eslint requires one (svelte/require-each-key). -->
	{#each twinkles as t (`${t.x}:${t.y}`)}
		<span
			class="twinkle"
			class:warm={t.warm}
			style="left:{t.x}%;top:{t.y}%;width:{t.s}px;height:{t.s}px;animation-duration:{t.d}s;animation-delay:{t.delay}s;"
		></span>
	{/each}
	<div class="blue glow"></div>
	<div class="warm glow"></div>
	<div class="shoots">
		<i class="shoot s1"></i>
		<i class="shoot s2"></i>
		<i class="shoot s3"></i>
	</div>
	{#if logo}
		<div class="emblem">
			<div class="emblem-glow"></div>
			<img src={logo} alt="" class="emblem-img" />
			{#if lockup}
				<div class="lockup">
					<div class="lockup-norcal">NORCAL</div>
					<div class="lockup-halo">HALO</div>
				</div>
			{/if}
		</div>
	{/if}
	{#if dim > 0}
		<div class="dimmer" style="opacity:{dim};"></div>
	{/if}
</div>

<style>
	.starfield {
		position: fixed;
		inset: 0;
		z-index: -10;
		overflow: hidden;
		pointer-events: none;
		background: linear-gradient(
			160deg,
			var(--color-surface-900) 0%,
			var(--color-surface-950) 60%,
			oklch(0.08 0.02 266) 100%
		);
	}
	.layer {
		position: absolute;
		inset: 0;
	}
	.far {
		background-image:
			radial-gradient(1.2px 1.2px at 22px 34px, rgba(205, 213, 234, 0.7) 50%, transparent 51%),
			radial-gradient(1px 1px at 98px 112px, rgba(159, 180, 208, 0.55) 50%, transparent 51%),
			radial-gradient(1px 1px at 150px 60px, rgba(159, 180, 208, 0.4) 50%, transparent 51%);
		background-size: 180px 150px;
		animation: drift-far 45s linear infinite;
	}
	.mid {
		background-image:
			radial-gradient(1.4px 1.4px at 40px 30px, rgba(205, 213, 234, 0.8) 50%, transparent 51%),
			radial-gradient(1.2px 1.2px at 130px 150px, rgba(159, 180, 208, 0.6) 50%, transparent 51%),
			radial-gradient(1px 1px at 210px 90px, rgba(159, 180, 208, 0.5) 50%, transparent 51%);
		background-size: 260px 220px;
		animation: drift-mid 28s linear infinite;
	}
	.near {
		background-image:
			radial-gradient(2px 2px at 60px 80px, rgba(232, 236, 245, 0.9) 50%, transparent 51%),
			radial-gradient(1.8px 1.8px at 200px 220px, rgba(205, 213, 234, 0.7) 50%, transparent 51%),
			radial-gradient(1.6px 1.6px at 300px 140px, rgba(255, 176, 65, 0.65) 50%, transparent 51%);
		background-size: 340px 300px;
		animation: drift-near 16s linear infinite;
	}
	@keyframes drift-far {
		to {
			background-position: -180px 0;
		}
	}
	@keyframes drift-mid {
		to {
			background-position: -260px 0;
		}
	}
	@keyframes drift-near {
		to {
			background-position: -340px 0;
		}
	}

	.twinkle {
		position: absolute;
		border-radius: 50%;
		background: #e8ecf5;
		box-shadow: 0 0 6px rgba(232, 236, 245, 0.9);
		animation: twinkle 3s ease-in-out infinite;
	}
	.twinkle.warm {
		background: oklch(0.83 0.13 65);
		box-shadow: 0 0 7px oklch(0.83 0.13 65 / 0.9);
	}
	@keyframes twinkle {
		0%,
		100% {
			opacity: 0.15;
		}
		50% {
			opacity: 1;
		}
	}

	.glow {
		position: absolute;
		animation: breathe 11s ease-in-out infinite;
	}
	.glow.blue {
		left: -10%;
		top: 8%;
		width: 60%;
		height: 70%;
		background: radial-gradient(
			closest-side,
			color-mix(in oklch, var(--color-secondary-500) 22%, transparent),
			transparent 70%
		);
	}
	.glow.warm {
		right: -8%;
		bottom: -10%;
		width: 46%;
		height: 60%;
		background: radial-gradient(
			closest-side,
			color-mix(in oklch, var(--color-primary-500) 13%, transparent),
			transparent 70%
		);
		animation-duration: 15s;
		animation-delay: 5s;
	}
	@keyframes breathe {
		0%,
		100% {
			opacity: 0.5;
			transform: translate(0, 0);
		}
		50% {
			opacity: 1;
			transform: translate(3%, -2%);
		}
	}

	.shoots {
		position: absolute;
		left: -6%;
		top: 10%;
		width: 120%;
		transform: rotate(16deg);
	}
	.shoot {
		position: absolute;
		left: 0;
		height: 2px;
		width: 130px;
		background: linear-gradient(90deg, transparent, rgba(205, 225, 255, 0.95));
		border-radius: 2px;
		opacity: 0;
		animation: shoot 9s linear infinite;
	}
	.shoot::after {
		content: '';
		position: absolute;
		right: -2px;
		top: -1.5px;
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: #fff;
		box-shadow: 0 0 10px #cfe4ff;
	}
	.shoot.s1 {
		top: 0;
	}
	.shoot.s2 {
		top: 160px;
		width: 100px;
		height: 1.5px;
		animation-duration: 13s;
		animation-delay: 3.5s;
	}
	.shoot.s3 {
		top: 320px;
		width: 110px;
		height: 1.5px;
		background: linear-gradient(90deg, transparent, rgba(255, 190, 90, 0.85));
		animation-duration: 17s;
		animation-delay: 7s;
	}
	.shoot.s3::after {
		background: #ffd9a0;
		box-shadow: 0 0 8px oklch(0.79 0.15 63);
	}
	@keyframes shoot {
		0% {
			transform: translateX(-20vw);
			opacity: 0;
		}
		4% {
			opacity: 1;
		}
		16% {
			transform: translateX(130vw);
			opacity: 0;
		}
		100% {
			transform: translateX(130vw);
			opacity: 0;
		}
	}

	.emblem {
		position: absolute;
		left: 50%;
		top: 50%;
		width: min(32vmin, 300px);
		transform: translate(-50%, -54%);
		animation: float 9s ease-in-out infinite;
	}
	.emblem-glow {
		position: absolute;
		inset: -28%;
		background: radial-gradient(
			closest-side,
			color-mix(in oklch, var(--color-secondary-500) 30%, transparent),
			transparent 70%
		);
		animation: pulse 7s ease-in-out infinite;
	}
	.emblem-img {
		position: relative;
		width: 100%;
		display: block;
		opacity: 0.82;
		filter: drop-shadow(0 0 14px color-mix(in oklch, var(--color-secondary-500) 55%, transparent));
		animation: glint 7s linear infinite;
	}
	.lockup {
		position: relative;
		text-align: center;
		line-height: 1;
		margin-top: 3%;
		animation: pulse 7s ease-in-out infinite;
	}
	.lockup-norcal {
		font-family: 'Orbitron', 'Oswald', sans-serif;
		font-weight: 800;
		font-size: clamp(18px, 3vmin, 28px);
		letter-spacing: 0.08em;
		color: var(--color-surface-50);
	}
	.lockup-halo {
		font-family: inherit;
		font-weight: 700;
		font-size: clamp(9px, 1.4vmin, 13px);
		letter-spacing: 1.2em;
		text-indent: 1.2em;
		margin-top: 0.5em;
		color: var(--color-secondary-300);
	}
	@keyframes float {
		0%,
		100% {
			transform: translate(-50%, -54%);
		}
		50% {
			transform: translate(-50%, calc(-54% - 10px));
		}
	}
	@keyframes pulse {
		0%,
		100% {
			opacity: 0.45;
		}
		50% {
			opacity: 1;
		}
	}
	@keyframes glint {
		0%,
		86%,
		100% {
			filter: drop-shadow(0 0 14px color-mix(in oklch, var(--color-secondary-500) 55%, transparent))
				brightness(1);
		}
		93% {
			filter: drop-shadow(0 0 18px color-mix(in oklch, var(--color-secondary-500) 75%, transparent))
				brightness(1.35);
		}
	}

	.dimmer {
		position: absolute;
		inset: 0;
		background: var(--color-surface-950);
	}

	@media (prefers-reduced-motion: reduce) {
		.far,
		.mid,
		.near,
		.twinkle,
		.glow,
		.shoot,
		.emblem,
		.emblem-glow,
		.emblem-img,
		.lockup {
			animation: none;
		}
		.shoot {
			display: none;
		}
	}
</style>
