<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
	import { themes } from './themes.js';

	let {
		player = {}, // {name, score, kills, deaths, assists, accuracy, damageRatio, place, team, alive, respawn, camo, camoMax}
		theme = 'ffa', // 'ffa' | 'red' | 'blue' — usually player.team
		scale = 1,
		origin = 'center',
		avatar = '' // url; falls back to the star mark
	} = $props();

	const t = $derived(themes[player.team] ?? themes[theme] ?? themes.ffa);
	const dead = $derived(player.alive === false);
	const camoActive = $derived((player.camo ?? 0) > 0);
	// Camo wipe: fraction of the card still cloaked (1 = just picked up, 0 = expired).
	const cloaked = $derived(camoActive ? player.camo / (player.camoMax || 30) : 0);
	const layers = $derived(camoActive ? ['ghost', 'full'] : ['full']);
	const acc = $derived(Number(player.accuracy ?? 0).toFixed(1));
	const dmg = $derived(Number(player.damageRatio ?? 0).toFixed(2));
</script>

<div
	class="card-anchor"
	class:dead
	style="transform: scale({scale}); transform-origin: {origin}; --panel:{t.panel}; --border:{t.border}; --glow:{t.glow}; --accent:{t.accent}; --clip:{cloaked *
		100}%"
>
	{#each layers as layer (layer)}
		<div class="card" class:ghost={layer === 'ghost'} class:wipe={layer === 'full' && camoActive}>
			{#if layer === 'full'}<div class="halo"></div>{/if}
			<div class="avatar">
				{#if avatar}
					<img src={avatar} alt={player.name} />
				{:else}
					<span class="star">✦</span>
				{/if}
			</div>
			<div class="ident">
				<span class="name">{player.name ?? '—'}</span>
				<span class="tag" style="color:{t.tagColor}">
					{t.tagText}
					{#if player.place}<span class="place"> · {player.place}</span>{/if}
				</span>
			</div>
			<span class="div"></span>
			{#key player.score}
				<span class="score pop" style="color:{t.scoreColor}">{player.score ?? 0}</span>
			{/key}
			<span class="div"></span>
			<div class="stats">
				<div class="stat">
					<span class="lbl">K</span>{#key player.kills}<span class="val pop"
							>{player.kills ?? 0}</span
						>{/key}
				</div>
				<div class="stat">
					<span class="lbl">D</span>{#key player.deaths}<span class="val pop"
							>{player.deaths ?? 0}</span
						>{/key}
				</div>
				<div class="stat">
					<span class="lbl">A</span>{#key player.assists}<span class="val pop"
							>{player.assists ?? 0}</span
						>{/key}
				</div>
			</div>
			<span class="div"></span>
			<div class="stat"><span class="lbl">ACC</span><span class="val">{acc}</span></div>
			<div class="stat"><span class="lbl">DMG</span><span class="val">{dmg}</span></div>
		</div>
	{/each}
</div>

<style>
	.card-anchor {
		position: relative;
		width: 700px;
		height: 64px;
		font-family: Inter, sans-serif;
	}
	.card-anchor.dead {
		opacity: 0.75;
		filter: grayscale(1) brightness(0.85);
	}
	.card {
		position: absolute;
		inset: 0;
		box-sizing: border-box;
		display: flex;
		align-items: center;
		gap: 15px;
		padding: 0 22px 0 9px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 32px;
		box-shadow:
			0 0 22px var(--glow),
			inset 0 1px 0 rgba(255, 255, 255, 0.14);
	}
	.card.ghost {
		opacity: 0.28;
	}
	/* Solidifies left-to-right as camo drains; the wipe edge is the progress bar. */
	.card.wipe {
		clip-path: inset(0 var(--clip) 0 0);
		transition: clip-path 0.25s linear;
	}
	.halo {
		position: absolute;
		inset: -1px;
		border-radius: 32px;
		pointer-events: none;
		box-shadow: 0 0 30px var(--glow);
		animation: breathe 4.5s ease-in-out infinite;
	}
	.avatar {
		width: 46px;
		height: 46px;
		flex: none;
		border-radius: 50%;
		overflow: hidden;
		background: repeating-linear-gradient(45deg, #10152a 0 7px, #141a30 7px 14px);
		border: 2px solid var(--accent);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.star {
		font-size: 16px;
		color: #3d62e0;
		filter: drop-shadow(0 0 5px rgba(61, 98, 224, 0.7));
		animation: twinkle 5s ease-in-out infinite;
	}
	.ident {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1;
		min-width: 0;
	}
	.name {
		font-family: Ultra, serif;
		font-size: 18px;
		line-height: 1;
		color: #e8ecf5;
		letter-spacing: 0.03em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.tag {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.3em;
	}
	.place {
		color: #9fb4d0;
		letter-spacing: 0.18em;
	}
	.div {
		width: 1px;
		height: 34px;
		background: rgba(255, 255, 255, 0.12);
		flex: none;
	}
	.score {
		font-size: 30px;
		font-weight: 700;
		line-height: 1;
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
	.lbl {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.22em;
		color: #9fb4d0;
	}
	.val {
		font-size: 18px;
		font-weight: 700;
		line-height: 1;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.pop {
		display: inline-block;
		animation: pop 0.7s ease-out;
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
	@keyframes twinkle {
		0%,
		100% {
			opacity: 0.75;
		}
		50% {
			opacity: 1;
		}
	}
	@keyframes pop {
		0% {
			transform: scale(1.55);
			color: #ffb041;
			text-shadow: 0 0 16px rgba(255, 176, 65, 0.7);
		}
		55% {
			transform: scale(1);
			color: #ffb041;
		}
		100% {
			transform: scale(1);
		}
	}
</style>
