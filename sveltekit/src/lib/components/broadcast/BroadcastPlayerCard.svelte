<script lang="ts">
	// One broadcast player card: a tinted Spartan bust (CharacterPreview) topped
	// with the gamertag, K/D/A, and a big score numeral. Halo 2 also carries the
	// player's emblem (chest decal + corner badge); Halo: CE has no emblem system
	// so that branch is simply absent, which is the visible CE-vs-H2 difference.
	//
	// Identity rules (see components/broadcast/player.ts):
	//   - Armor colour: TEAM games share ONE team colour (colour doesn't tell
	//     teammates apart — the emblem + gamertag do); FFA is per-player + distinct.
	//   - Avatar: the player's gamertag PROFILE Spartan + H2 emblem when they have
	//     one; a plain tinted Spartan (no placeholder emblem) when they don't.
	import CharacterPreview from '$lib/components/gamertag/CharacterPreview.svelte';
	import EmblemPreview from '$lib/components/gamertag/EmblemPreview.svelte';
	import type { PlayerRow } from '$lib/utils/overlay-view';
	import type { BroadcastGame } from './theme';
	import { CE_COLORS, H2_COLORS, colorHex, colorName } from '$lib/utils/emblem';
	import { resolveArmorIndex, cardAppearance, type ResolvedProfile } from './player';

	let {
		player,
		game,
		teamColor,
		isTeamGame,
		profile,
		teamLabel,
		size = 150,
		hasTick = true,
		hasScores = true
	}: {
		player: PlayerRow;
		game: BroadcastGame;
		/** Team accent (from teamMeta) — ribbon + name rule. */
		teamColor: string;
		/** Whether this is a team game (drives the shared-team-colour rule). */
		isTeamGame: boolean;
		/** The player's resolved profile avatar (emblem/appearance), or null. */
		profile?: ResolvedProfile | null;
		/** Team name shown under the gamertag in team games (colour identifies the
		 * team, not the player). Ignored in FFA (the armor-colour chip shows there). */
		teamLabel?: string;
		/** Spartan bust height in px. */
		size?: number;
		/** Whether live tick data is present (drives health/shield bars). */
		hasTick?: boolean;
		/** Whether cumulative stats are trustworthy (mutes K/D/A + score if not). */
		hasScores?: boolean;
	} = $props();

	const palette = $derived(game === 'ce' ? CE_COLORS : H2_COLORS);
	// Game-accurate armor index: shared team colour (team) or per-player (FFA).
	const colorIndex = $derived(resolveArmorIndex(game, isTeamGame, player.team, player.armorColor));
	const armorHex = $derived(colorHex(palette, colorIndex));
	const armorName = $derived(colorName(palette, colorIndex));
	// H2 emblem from the profile, re-coloured to the game-accurate armor; undefined
	// for CE or a profile-less player → CharacterPreview draws no emblem.
	const appearance = $derived(cardAppearance(game, colorIndex, profile ?? null));
	const showEmblem = $derived(game === 'h2' && !!appearance);
	const dead = $derived(hasTick && !player.alive);
	const respawnSecs = $derived(player.respawnIn != null ? Math.ceil(player.respawnIn / 30) : null);
</script>

<div class="card" class:dead style="--armor: {armorHex}; --team: {teamColor};">
	<!-- team ribbon -->
	<div class="ribbon"></div>

	<!-- Spartan pod -->
	<div class="pod">
		<div class="glow"></div>
		<CharacterPreview
			{game}
			{appearance}
			{colorIndex}
			armorOverride={colorIndex}
			{showEmblem}
			{size}
			showName={false}
		/>
		{#if showEmblem}
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

	<!-- identity: PB profile avatar (pulled straight from PocketBase, cached
	     file — not client-rendered) + gamertag + team/armor chip -->
	<div class="ident">
		<span class="pb-avatar">
			{#if profile?.avatar}
				<img src={profile.avatar} alt="{player.name} avatar" loading="lazy" />
			{:else}
				<!-- generic fallback silhouette when the gamertag has no profile/avatar -->
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<circle cx="12" cy="8.4" r="4.2" fill="currentColor" />
					<path d="M3.5 21q1.4-6 8.5-6t8.5 6Z" fill="currentColor" />
				</svg>
			{/if}
		</span>
		<span class="who">
			<span class="name" title={player.name}>{player.name}</span>
			{#if isTeamGame}
				{#if teamLabel}
					<span class="armor-chip"><i style="background: {teamColor}"></i>{teamLabel}</span>
				{/if}
			{:else}
				<span class="armor-chip"><i style="background: {armorHex}"></i>{armorName}</span>
			{/if}
		</span>
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
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		width: 100%;
		min-width: 0;
	}
	/* Dedicated PB-avatar spot: fixed plate, image served by PocketBase. */
	.pb-avatar {
		flex-shrink: 0;
		width: 38px;
		height: 38px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--bc-panel-strong);
		border: 1px solid var(--bc-edge);
		border-radius: calc(var(--bc-radius) - 1px);
		overflow: hidden;
	}
	.pb-avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	.pb-avatar svg {
		width: 70%;
		height: 70%;
		color: var(--bc-ink-muted);
		opacity: 0.7;
	}
	.who {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.18rem;
		min-width: 0;
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
