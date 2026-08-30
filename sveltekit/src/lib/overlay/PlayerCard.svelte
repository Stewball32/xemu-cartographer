<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// POV bar — redesign (CL-01/02/05/06/07/08/13/14/18). One per seat,
	// 820×72. Identity is the shared NamePlate (neutral — the bar's panel +
	// 1.5px accent frame carry team color). Score white, Selection Orange only
	// while lobby-best, kill pop on change. K/D/A unpadded. SPR column dropped —
	// the live spree is the crosshair tally above the frame. Camo ghosts the
	// bar surface and the plate's banner (never avatar/name/stats), the solid
	// front advancing left→right off the timer. Overshield = conic rings on the
	// plate's avatar well.
	import { untrack } from 'svelte';
	import { themes } from './themes.js';
	import NamePlate from './NamePlate.svelte';
	import SpreeTicks from './SpreeTicks.svelte';

	let {
		player = {},
		theme = 'ffa', // 'ffa' | 'red' | 'blue' — usually player.team
		scale = 1,
		origin = 'center',
		/** Lobby-best score — flips the score Selection Orange (CL-01). */
		leader = false,
		/** Sheen animation delay so stacked bars don't sweep in unison. */
		sheen = '0s'
	} = $props();

	const t = $derived(themes[player.team] ?? themes[theme] ?? themes.ffa);
	const dead = $derived(player.alive === false);

	const dmg = $derived(
		Number.isFinite(Number(player.damageRatio))
			? Number(player.damageRatio).toFixed(2)
			: (player.damageRatio ?? 0) > 0
				? '∞'
				: '0.00'
	);
	// ACC lights up once the shots_hit offsets land (CL-05); dash until then.
	const acc = $derived(player.acc != null ? Number(player.acc).toFixed(1) : null);

	// Camo (CL-07): player.camo — number > 1 reads as % of cloak remaining
	// (100 = fully cloaked); true/1 means the wire only has the has_camo bool,
	// so run CE's nominal 30s decay locally, matching the old row behavior.
	//
	// Split in two on purpose. `player` is a fresh object on every ~30Hz roster
	// update, so an effect that reads it re-runs constantly — running the decay
	// there restarted its timer each frame and pinned the wipe at 100%. The
	// first effect distils the prop down to two primitives; the second depends
	// only on those, so it re-runs on an actual cloak transition and the timer
	// survives the frames in between.
	let camoOn = $state(false);
	let camoWirePct = $state(0);
	$effect(() => {
		const c = player.camo;
		const pct = typeof c === 'number' && c > 1 ? Math.min(100, c) : 0;
		const on = c === true || c === 1;
		untrack(() => {
			camoWirePct = pct;
			camoOn = on;
		});
	});

	let camoPct = $state(0);
	$effect(() => {
		if (camoWirePct > 0) {
			untrack(() => (camoPct = camoWirePct));
			return;
		}
		if (!camoOn) {
			untrack(() => (camoPct = 0));
			return;
		}
		const t0 = performance.now();
		untrack(() => (camoPct = 100));
		const int = setInterval(() => {
			camoPct = Math.max(0, 100 - ((performance.now() - t0) / 30000) * 100);
			if (camoPct === 0) clearInterval(int);
		}, 100);
		return () => clearInterval(int);
	});
	const cloaked = $derived(camoPct > 0 && !dead);

	// Overshield rings live on the plate's avatar well (shield 1–3).
	const osVal = $derived((player.shield ?? 0) > 1 && !dead ? (player.shield ?? 0) : 0);

	// Kill pop (CL-06): scale 1.55 → 1 with the #FFB041 flash on score change.
	let pop = $state(false);
	let prevScore;
	let popTimer;
	$effect(() => {
		const s = player.score ?? 0;
		untrack(() => {
			if (prevScore !== undefined && s > prevScore) {
				pop = false;
				clearTimeout(popTimer);
				requestAnimationFrame(() => (pop = true));
				popTimer = setTimeout(() => (pop = false), 750);
			}
			prevScore = s;
		});
	});
</script>

<div
	class="pov"
	class:dead
	class:cloaked
	style="transform: scale({scale}); transform-origin: {origin}; --accent:{t.accent}; --border:{t.border}; --glow:{t.glow}; --panel:{t.panel}"
>
	<div class="breathe"></div>
	<div class="sheenclip"><div class="sheen" style="animation-delay:{sheen}"></div></div>

	{#if cloaked}
		<!-- Solid surface, wiped off the camo timer: the visible edge is the
		     re-solidifying front advancing left → right as camo drains. -->
		<div class="solid" style="clip-path: inset(0 {camoPct}% 0 0)"></div>
	{/if}

	<SpreeTicks spree={player.spree ?? 0} {dead} />
	<NamePlate {player} h={64} ghost={cloaked && camoPct > 50} os={osVal} bg={player.plateBg} />
	<div class="grow"></div>

	<span class="rule"></span>
	<span class="score" class:is-best={leader} class:pop>{player.score ?? 0}</span>
	<span class="rule"></span>

	<div class="stats">
		<div class="stat"><b>K</b><i>{player.kills ?? 0}</i></div>
		<div class="stat"><b>D</b><i>{player.deaths ?? 0}</i></div>
		<div class="stat"><b>A</b><i>{player.assists ?? 0}</i></div>
	</div>

	<span class="rule"></span>
	{#if acc}
		<div class="stat"><b>ACC</b><i>{acc}</i></div>
	{:else}
		<div class="stat"><b>ACC</b><i class="na">—</i></div>
	{/if}
	<div class="stat"><b>DMG</b><i>{dmg}</i></div>
</div>

<style>
	.pov {
		position: relative;
		display: flex;
		align-items: center;
		gap: 15px;
		width: 820px;
		box-sizing: border-box;
		height: 72px;
		padding: 0 22px 0 4px;
		background: var(--panel);
		border: 1.5px solid color-mix(in srgb, var(--accent) 70%, transparent);
		border-radius: 12px;
		box-shadow:
			0 0 30px var(--glow),
			0 0 10px var(--glow),
			inset 0 1px 0 rgba(255, 255, 255, 0.14);
		font-family: Inter, system-ui, sans-serif;
	}
	.pov.dead {
		opacity: 0.75;
		filter: grayscale(1) brightness(0.85);
	}
	/* Camo base state: the ghosted surface the solid front wipes over. */
	.pov.cloaked {
		background: rgba(11, 14, 26, 0.26);
		border-color: color-mix(in srgb, var(--accent) 25%, transparent);
	}
	.pov.cloaked > :global(*) {
		position: relative;
	}
	.pov.cloaked > .breathe,
	.pov.cloaked > .sheenclip,
	.pov.cloaked > .solid,
	.pov.cloaked > :global(.spree-clip) {
		position: absolute;
	}
	.solid {
		inset: 0;
		border-radius: 12px;
		background: var(--panel);
		border: 1.5px solid color-mix(in srgb, var(--accent) 70%, transparent);
		box-sizing: border-box;
		pointer-events: none;
	}

	.breathe {
		position: absolute;
		inset: -1px;
		border-radius: 12px;
		box-shadow: 0 0 30px var(--glow);
		animation: breathe 4.5s ease-in-out infinite;
		pointer-events: none;
	}
	.sheenclip {
		position: absolute;
		inset: 0;
		border-radius: 12px;
		overflow: hidden;
		pointer-events: none;
	}
	.sheen {
		position: absolute;
		top: 0;
		bottom: 0;
		left: 0;
		width: 55%;
		background: linear-gradient(105deg, transparent, rgba(232, 236, 245, 0.09), transparent);
		transform: translateX(-140%);
		animation: sheen 9s ease-in-out infinite;
	}

	.grow {
		flex: 1;
		min-width: 0;
	}
	.rule {
		width: 1px;
		height: 34px;
		background: rgba(255, 255, 255, 0.12);
		flex: none;
	}
	/* Inter numerals (CL-12); white unless lobby-best (CL-01). */
	.score {
		font-size: 30px;
		font-weight: 700;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
		min-width: 44px;
		text-align: center;
	}
	.score.is-best {
		color: var(--nh-orange);
	}
	.score.pop {
		display: inline-block;
		animation: killpop 0.7s ease-out both;
	}

	.stats {
		display: flex;
		gap: 13px;
	}
	.stat {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		min-width: 32px;
	}
	.stat b {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.22em;
		color: var(--nh-steel);
	}
	.stat i {
		font-style: normal;
		font-size: 18px;
		font-weight: 700;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}
	.stat i.na {
		color: var(--nh-mute);
	}

	@keyframes breathe {
		0%,
		100% {
			opacity: 0.35;
		}
		50% {
			opacity: 1;
		}
	}
	@keyframes sheen {
		0% {
			transform: translateX(-140%);
		}
		16%,
		100% {
			transform: translateX(300%);
		}
	}
	@keyframes killpop {
		0% {
			transform: scale(1.55);
			color: #ffb041;
			text-shadow: 0 0 14px rgba(255, 176, 65, 0.8);
		}
		38% {
			transform: scale(1);
			text-shadow: none;
		}
		100% {
			transform: scale(1);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.breathe,
		.sheen,
		.score.pop {
			animation: none;
		}
	}
</style>
