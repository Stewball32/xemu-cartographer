<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// POV bar — one per split-screen seat: emblem badge, name, placing, score,
	// K/D/A, spree/accuracy/damage. Ported from the obs-handoff pack's
	// pov-bar.html. Natural size 700×64; /overlay/ anchors 1–4 of these on a
	// 1440×1080 canvas and scales the quadrant seats to 68%.
	//
	// The pack's standalone ?player= / ?slot= targeting is deliberately NOT
	// carried over — /overlay/'s ?console=NAME resolves a seat by console name,
	// which survives lobby churn (machine indices shift live) where a fixed slot
	// index would silently follow the wrong player.
	import starUrl from '$lib/assets/star.png';
	import { themes } from './themes.js';

	let {
		player = {},
		theme = 'ffa', // 'ffa' | 'red' | 'blue' — usually player.team
		scale = 1,
		origin = 'center',
		/** Placing label, e.g. `3RD`. Empty hides the suffix. */
		place = '',
		/** Sheen animation delay so stacked bars don't sweep in unison. */
		sheen = '0s'
	} = $props();

	const t = $derived(themes[player.team] ?? themes[theme] ?? themes.ffa);
	const dead = $derived(player.alive === false);

	const spree = $derived(player.spree > 0 ? `×${player.spree}` : '—');
	// DMG = damage dealt ÷ damage taken. Unbounded (dealt some, taken none)
	// renders as ∞ — see damageRatioOf for why not the raw dealt total.
	// ACC needs shots_HIT, which reads 0 live (see overlay-state.ts), so it shows
	// an em dash rather than a fake 0.0 that would read as a real stat.
	const dmg = $derived(
		Number.isFinite(Number(player.damageRatio))
			? Number(player.damageRatio).toFixed(2)
			: (player.damageRatio ?? 0) > 0
				? '∞'
				: '0.00'
	);
</script>

<div
	class="pov"
	class:dead
	style="transform: scale({scale}); transform-origin: {origin}; --accent:{t.accent}; --border:{t.border}; --glow:{t.glow}"
>
	<div class="breathe"></div>
	<div class="sheenclip"><div class="sheen" style="animation-delay:{sheen}"></div></div>

	<div class="badge" style="border-color:{player.armor || t.accent}">
		<img src={starUrl} alt="" />
	</div>

	<div class="id">
		<span class="name">{player.name ?? '—'}</span>
		<span class="tag" style="color:{t.tagColor}">
			{t.tagText}
			{#if place}<em>· {place}</em>{/if}
		</span>
	</div>

	<span class="rule"></span>
	<span class="score">{player.score ?? 0}</span>
	<span class="rule"></span>

	<div class="stats">
		<div class="stat"><b>K</b><i>{player.kills ?? 0}</i></div>
		<div class="stat"><b>D</b><i>{player.deaths ?? 0}</i></div>
		<div class="stat"><b>A</b><i>{player.assists ?? 0}</i></div>
	</div>

	<span class="rule"></span>
	<div class="stat"><b>SPR</b><i>{spree}</i></div>
	<div class="stat"><b>ACC</b><i class="na">—</i></div>
	<div class="stat"><b>DMG</b><i>{dmg}</i></div>
</div>

<style>
	.pov {
		position: relative;
		display: flex;
		align-items: center;
		gap: 15px;
		width: 700px;
		box-sizing: border-box;
		height: 64px;
		padding: 0 22px 0 5px;
		background: rgba(11, 14, 26, 0.93);
		border: 1px solid var(--border);
		border-radius: 12px;
		box-shadow:
			0 0 22px var(--glow),
			inset 0 1px 0 rgba(255, 255, 255, 0.14);
		font-family: Inter, system-ui, sans-serif;
	}
	.pov.dead {
		opacity: 0.75;
		filter: grayscale(1) brightness(0.85);
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

	.badge {
		width: 54px;
		height: 54px;
		flex: none;
		border-radius: 50%;
		background: repeating-linear-gradient(45deg, #10152a 0 7px, #141a30 7px 14px);
		border: 2px solid var(--accent);
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
	}
	.badge img {
		width: 58px;
		flex: none;
		display: block;
		filter: drop-shadow(0 0 6px rgba(61, 98, 224, 0.6));
		animation: twinkle 5s ease-in-out infinite;
	}

	.id {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1;
		min-width: 0;
	}
	.name {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 15px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.04em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.tag {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.3em;
	}
	.tag em {
		font-style: normal;
		color: var(--nh-steel);
		letter-spacing: 0.18em;
	}

	.rule {
		width: 1px;
		height: 34px;
		background: rgba(255, 255, 255, 0.12);
		flex: none;
	}
	.score {
		font-size: 30px;
		font-weight: 700;
		line-height: 1;
		color: var(--nh-orange);
		font-variant-numeric: tabular-nums;
		min-width: 44px;
		text-align: center;
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
	@keyframes twinkle {
		0%,
		100% {
			opacity: 0.75;
		}
		50% {
			opacity: 1;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.breathe,
		.sheen,
		.badge img {
			animation: none;
		}
	}
</style>
