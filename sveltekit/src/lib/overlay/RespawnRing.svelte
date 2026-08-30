<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Respawn ring — redesign (CL-11/17/18). Filled conic disc reading
	// respawn_in_ticks ÷ 30 UNROUNDED, so it drains continuously instead of
	// jumping once a second; the seconds text keeps the ceiling. Sweep is
	// reversed vs the as-built: the accent edge advances CLOCKWISE as time
	// passes, glow tint filling behind it. `max` is the gametype's respawn
	// time, not a hardcoded 5. Below the disc: KILLED BY + the killer's
	// mottoless plate when the killer is known, else the RESPAWNING pill.
	// Motion 2a (lock-on): the disc swings in oversized with a −20°
	// counter-rotation and settles — disc only; the pill + plate rise in
	// quietly 240ms behind it. Plays on every mount, i.e. every death.
	// Exit is the out: transition in overlay/+page.svelte.
	import { themes } from './themes.js';
	import NamePlate from './NamePlate.svelte';

	let { seconds = 3, max = 8, theme = 'ffa', killer = null } = $props();

	const t = $derived(themes[theme] ?? themes.ffa);
	const deg = $derived(Math.max(0, Math.min(360, (seconds / max) * 360)));
</script>

<div class="ring-wrap">
	<div
		class="ring"
		style="background: conic-gradient({t.glow} 0deg {360 - deg}deg, {t.accent} {360 -
			deg}deg 360deg); --glow:{t.glow}"
	>
		<div class="ring-inner"><span class="secs">{Math.ceil(seconds)}</span></div>
	</div>
	{#if killer}
		<div class="killed">
			<span class="label" style="color:{t.tagColor}; border-color:{t.border}">KILLED BY</span>
			<NamePlate player={killer} h={56} showMotto={false} />
		</div>
	{:else}
		<span class="label" style="color:{t.tagColor}; border-color:{t.border}">RESPAWNING</span>
	{/if}
</div>

<style>
	.ring-wrap {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		font-family: Inter, sans-serif;
	}
	.ring {
		width: 118px;
		height: 118px;
		border-radius: 50%;
		box-shadow: 0 0 26px var(--glow);
		display: flex;
		align-items: center;
		justify-content: center;
		transform-origin: 50% 50%;
		animation: lock-in 0.42s cubic-bezier(0.22, 1, 0.36, 1) both;
	}
	.ring-inner {
		width: 98px;
		height: 98px;
		border-radius: 50%;
		background: rgba(11, 14, 26, 0.92);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.12);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.secs {
		font-size: 46px;
		font-weight: 700;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.killed {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
	}
	/* Pill + plate sit out the disc's swing — quiet rise once it locks. */
	.killed,
	.ring-wrap > .label {
		animation: plate-in 0.32s cubic-bezier(0.22, 1, 0.36, 1) 0.24s both;
	}
	.label {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.32em;
		background: rgba(11, 14, 26, 0.88);
		border: 1px solid;
		border-radius: 999px;
		padding: 5px 12px 5px 16px;
	}
	@keyframes lock-in {
		0% {
			opacity: 0;
			transform: scale(1.5) rotate(-20deg);
			filter: brightness(2.6);
		}
		55% {
			opacity: 1;
			transform: scale(0.96) rotate(1.5deg);
			filter: brightness(1.5);
		}
		100% {
			opacity: 1;
			transform: none;
			filter: brightness(1);
		}
	}
	@keyframes plate-in {
		0% {
			opacity: 0;
			transform: translateY(10px);
		}
		100% {
			opacity: 1;
			transform: none;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.ring,
		.killed,
		.ring-wrap > .label {
			animation: none;
		}
	}
</style>
