<script lang="ts">
	// One broadcast player card: a tinted Spartan bust (CharacterPreview) topped
	// with the gamertag, K/D/A, and a big score numeral. Halo 2 also carries the
	// player's emblem (chest decal + corner badge); Halo: CE has no emblem system
	// so that branch is simply absent, which is the visible CE-vs-H2 difference.
	//
	// Game-agnostic: it reads the broadcast theme through `var(--bc-*)` set by an
	// ancestor, and the per-player Spartan tint from the armor palette. Consumes a
	// PlayerRow (overlay-view) so the scoreboard + cards share one player shape.
	import CharacterPreview from '$lib/components/gamertag/CharacterPreview.svelte';
	import EmblemPreview from '$lib/components/gamertag/EmblemPreview.svelte';
	import type { PlayerRow } from '$lib/utils/overlay-view';
	import type { BroadcastGame } from './theme';
	import { CE_COLORS, H2_COLORS, colorHex, colorName, type Appearance } from '$lib/utils/emblem';

	let {
		player,
		game,
		teamColor,
		appearance,
		size = 150,
		hasTick = true,
		hasScores = true
	}: {
		player: PlayerRow;
		game: BroadcastGame;
		/** Team accent (from teamMeta) — ribbon + name rule. */
		teamColor: string;
		/** H2 emblem source; ignored for CE. */
		appearance?: Appearance;
		/** Spartan bust height in px. */
		size?: number;
		/** Whether live tick data is present (drives health/shield bars). */
		hasTick?: boolean;
		/** Whether cumulative stats are trustworthy (mutes K/D/A + score if not). */
		hasScores?: boolean;
	} = $props();

	const palette = $derived(game === 'ce' ? CE_COLORS : H2_COLORS);
	const armorHex = $derived(colorHex(palette, player.armorColor));
	const armorName = $derived(colorName(palette, player.armorColor));
	const dead = $derived(hasTick && !player.alive);
	const respawnSecs = $derived(player.respawnIn != null ? Math.ceil(player.respawnIn / 30) : null);
</script>

<div class="card" class:dead style="--armor: {armorHex}; --team: {teamColor};">
	<!-- team ribbon -->
	<div class="ribbon"></div>

	<!-- Spartan pod -->
	<div class="pod">
		<div class="glow"></div>
		<CharacterPreview {game} {appearance} colorIndex={player.armorColor} {size} showName={false} />
		{#if game === 'h2' && appearance}
			<div class="emblem-badge">
				<EmblemPreview {appearance} size={34} rounded ring title="Emblem" />
			</div>
		{/if}
		{#if player.hasOvershield || player.hasCamo}
			<div class="pips">
				{#if player.hasOvershield}<span class="pip os" title="Overshield">OS</span>{/if}
				{#if player.hasCamo}<span class="pip camo" title="Active camo">CAMO</span>{/if}
			</div>
		{/if}
		{#if dead}
			<div class="down">
				<span class="down-label">RESPAWN</span>
				{#if respawnSecs != null}<span class="down-secs">{respawnSecs}s</span>{/if}
			</div>
		{/if}
	</div>

	<!-- identity -->
	<div class="ident">
		<span class="name" title={player.name}>{player.name}</span>
		<span class="armor-chip"><i style="background: {armorHex}"></i>{armorName}</span>
	</div>

	<!-- stats -->
	<div class="stats" class:muted={!hasScores}>
		<div class="score">
			<span class="score-num">{hasScores ? player.score : '—'}</span>
			<span class="score-lbl">SCORE</span>
		</div>
		<div class="kda">
			<div class="kda-cell"><b>{hasScores ? player.kills : '—'}</b><span>K</span></div>
			<div class="kda-cell"><b>{hasScores ? player.deaths : '—'}</b><span>D</span></div>
			<div class="kda-cell"><b>{hasScores ? player.assists : '—'}</b><span>A</span></div>
		</div>
	</div>

	<!-- vitals -->
	{#if hasTick}
		<div class="bars">
			<div class="bar">
				<span class="fill shield" style="width: {player.shields * 100}%"></span>
			</div>
			<div class="bar"><span class="fill health" style="width: {player.health * 100}%"></span></div>
		</div>
	{/if}
</div>

<style>
	.card {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.45rem;
		width: 12rem;
		padding: 0.7rem 0.7rem 0.85rem;
		font-family: var(--bc-font);
		color: var(--bc-ink);
		background: var(--bc-panel);
		border: 1px solid var(--bc-edge);
		border-top: 3px solid var(--team);
		border-radius: var(--bc-radius);
		box-shadow:
			0 0 0 1px rgba(0, 0, 0, 0.5),
			0 8px 26px rgba(0, 0, 0, 0.55),
			inset 0 0 44px rgba(0, 0, 0, 0.35);
		overflow: hidden;
		transition: opacity 0.25s;
	}
	.card.dead {
		opacity: 0.72;
	}
	/* faint corner accent tick */
	.card::after {
		content: '';
		position: absolute;
		right: 0;
		bottom: 0;
		width: 26px;
		height: 26px;
		background: linear-gradient(315deg, var(--bc-glow), transparent 60%);
		opacity: 0.5;
		pointer-events: none;
	}

	.ribbon {
		position: absolute;
		inset: 0 0 auto 0;
		height: 3px;
		background: linear-gradient(90deg, transparent, var(--team), transparent);
		opacity: 0.8;
	}

	.pod {
		position: relative;
		display: flex;
		align-items: flex-end;
		justify-content: center;
		width: 100%;
	}
	.glow {
		position: absolute;
		inset: 0;
		background: radial-gradient(ellipse 70% 55% at 50% 78%, var(--armor), transparent 70%);
		opacity: 0.28;
		filter: blur(2px);
		pointer-events: none;
	}
	.emblem-badge {
		position: absolute;
		top: -2px;
		right: -2px;
		filter: drop-shadow(0 2px 5px rgba(0, 0, 0, 0.7));
	}
	.pips {
		position: absolute;
		left: 0;
		top: 0;
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}
	.pip {
		font-size: 0.56rem;
		font-weight: 800;
		letter-spacing: 0.06em;
		line-height: 1;
		padding: 0.16rem 0.28rem;
		border-radius: 0.2rem;
		text-shadow: none;
	}
	.pip.os {
		background: rgba(255, 214, 102, 0.92);
		color: #2a1c00;
	}
	.pip.camo {
		background: rgba(120, 200, 255, 0.82);
		color: #021018;
	}
	.down {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 0.1rem;
		background: rgba(6, 8, 12, 0.42);
		backdrop-filter: grayscale(0.6);
	}
	.down-label {
		font-size: 0.7rem;
		font-weight: 800;
		letter-spacing: 0.16em;
		color: #ff6b6b;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.9);
	}
	.down-secs {
		font-size: 1.4rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
		text-shadow: 0 1px 4px rgba(0, 0, 0, 0.9);
	}

	.ident {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.18rem;
		width: 100%;
	}
	.name {
		max-width: 100%;
		font-size: 1.05rem;
		font-weight: 700;
		letter-spacing: 0.02em;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.9);
	}
	.armor-chip {
		display: inline-flex;
		align-items: center;
		gap: 0.28rem;
		font-size: 0.62rem;
		font-weight: 600;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--bc-ink-muted);
	}
	.armor-chip i {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		box-shadow: 0 0 6px var(--armor);
	}

	.stats {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		width: 100%;
		padding: 0.32rem 0.2rem;
		background: var(--bc-panel-strong);
		border-radius: calc(var(--bc-radius) - 1px);
		border: 1px solid var(--bc-edge);
	}
	.stats.muted {
		opacity: 0.5;
	}
	.score {
		display: flex;
		flex-direction: column;
		align-items: center;
		line-height: 1;
	}
	.score-num {
		font-size: 1.7rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
		color: var(--bc-accent);
		text-shadow: 0 0 12px var(--bc-glow);
	}
	.score-lbl {
		font-size: 0.52rem;
		font-weight: 700;
		letter-spacing: 0.18em;
		color: var(--bc-ink-muted);
	}
	.kda {
		display: flex;
		gap: 0.5rem;
	}
	.kda-cell {
		display: flex;
		flex-direction: column;
		align-items: center;
		line-height: 1;
	}
	.kda-cell b {
		font-size: 0.95rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}
	.kda-cell span {
		font-size: 0.5rem;
		font-weight: 700;
		letter-spacing: 0.1em;
		color: var(--bc-ink-muted);
	}

	.bars {
		display: flex;
		flex-direction: column;
		gap: 3px;
		width: 100%;
	}
	.bar {
		height: 4px;
		background: rgba(255, 255, 255, 0.12);
		border-radius: 999px;
		overflow: hidden;
	}
	.fill {
		display: block;
		height: 100%;
		transition: width 0.14s linear;
	}
	.fill.shield {
		background: linear-gradient(90deg, var(--bc-accent2), var(--bc-accent));
	}
	.fill.health {
		background: linear-gradient(90deg, #ff5470, #ff8a5c);
	}
</style>
