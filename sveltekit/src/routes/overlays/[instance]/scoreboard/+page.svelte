<script lang="ts">
	// Broadcast SCOREBOARD browser source. The canonical overlay: a themed header
	// (gametype · map · match clock · score-to) over team columns (or an FFA
	// ranked list), each row an armor-chipped player with K/D/A + score + vitals.
	// Reuses the shared overlay feed (so ?mock=1 previews) + the pure view builders;
	// the CE-vs-H2 look is applied through the broadcast theme's `--bc-*` vars.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildScoreboard, matchClock, statusStrip, teamMeta } from '$lib/utils/overlay-view';
	import type { PlayerRow } from '$lib/utils/overlay-view';
	import { broadcastTheme, themeVars } from '$lib/components/broadcast/theme';
	import { CE_COLORS, H2_COLORS, colorHex } from '$lib/utils/emblem';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	const theme = $derived(broadcastTheme(data.game));
	const palette = $derived(data.game === 'ce' ? CE_COLORS : H2_COLORS);

	const vm = $derived(buildScoreboard(feed.game, feed.tick));
	const clock = $derived(matchClock(feed.game));
	const status = $derived(statusStrip(feed.game, feed.scenario));

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			// game = roster + stats; tick = alive/health/shields; scenario = map name.
			classes: ['game', 'tick', 'scenario']
		})
	);
	onDestroy(() => feed.stop());
</script>

<svelte:head>
	<!-- OBS browser source: fully transparent canvas, no scrollbars. Neutralise the
	     theme's body background-image + ::before/::after decorations. -->
	<style>
		html,
		body {
			background: transparent !important;
			background-image: none !important;
			overflow: hidden !important;
			margin: 0 !important;
		}
		body::before,
		body::after {
			display: none !important;
			content: none !important;
		}
	</style>
</svelte:head>

<div class="stage" style="--scale: {data.scale}; {themeVars(theme)}" data-game={data.game}>
	{#if feed.missingToken}
		<div class="status">
			No overlay token. Mint one at <code>/overlays/manage/</code> or append
			<code>?mock=1</code> to preview.
		</div>
	{:else if !feed.connected}
		<div class="status">Connecting…</div>
	{:else if vm.playerCount === 0}
		<div class="status">Waiting for match…</div>
	{:else}
		<div class="board">
			<header class="head">
				<div class="head-left">
					<span class="wordmark">{theme.label}</span>
					<span class="gametype">{vm.gametype || 'Match'}</span>
					{#if status.map}<span class="map">{status.map}</span>{/if}
				</div>
				<div class="head-clock" class:live={clock.live}>
					{#if clock.live}<span class="live-dot"></span>{/if}
					<span class="clock">{clock.label}</span>
					<span class="clock-lbl">{clock.direction === 'down' ? 'REMAINING' : 'ELAPSED'}</span>
				</div>
				<div class="head-right">
					{#if status.scoreLimit > 0}
						<span class="limit"><b>{status.scoreLimit}</b><span>TO WIN</span></span>
					{/if}
					{#if feed.mock}<span class="badge">MOCK</span>{/if}
				</div>
			</header>

			{#if vm.isTeamGame}
				<div class="teams" style="--cols: {Math.min(vm.teams.length, 2)}">
					{#each vm.teams as t (t.team)}
						<section class="team" style="--team: {t.color}">
							<header class="team-head">
								<span class="team-name">{t.name}</span>
								{#if vm.hasScores}<span class="team-score">{t.score}</span>{/if}
							</header>
							<div class="rows">
								{#each t.players as p (p.index)}
									{@render row(p, t.color)}
								{/each}
							</div>
						</section>
					{/each}
				</div>
			{:else}
				<section class="team ffa" style="--team: {theme.accent}">
					<div class="rows">
						{#each vm.players as p, i (p.index)}
							{@render row(p, teamMeta(p.team).color, i + 1)}
						{/each}
					</div>
				</section>
			{/if}
		</div>
	{/if}
</div>

{#snippet row(p: PlayerRow, color: string, rank?: number)}
	<div class="prow" class:dead={vm.hasTick && !p.alive} style="--team: {color}">
		{#if rank}<span class="rank">{rank}</span>{/if}
		<span class="chip" style="background: {colorHex(palette, p.armorColor)}"></span>
		<span class="pname" title={p.name}>{p.name}</span>
		<span class="pips">
			{#if p.hasOvershield}<span class="pip os" title="Overshield">OS</span>{/if}
			{#if p.hasCamo}<span class="pip camo" title="Camo">C</span>{/if}
		</span>
		{#if vm.hasTick && !p.alive}
			<span class="respawn">↻{p.respawnIn ? ` ${Math.ceil(p.respawnIn / 30)}s` : ''}</span>
		{/if}
		<span class="kda" class:muted={!vm.hasScores}>
			{vm.hasScores ? `${p.kills}/${p.deaths}/${p.assists}` : '—'}
		</span>
		<span class="pscore" class:muted={!vm.hasScores}>{vm.hasScores ? p.score : '—'}</span>
		{#if vm.hasTick}
			<span class="minibars">
				<span class="mb"><i class="s" style="width: {p.shields * 100}%"></i></span>
				<span class="mb"><i class="h" style="width: {p.health * 100}%"></i></span>
			</span>
		{/if}
	</div>
{/snippet}

<style>
	.stage {
		position: fixed;
		inset: 0;
		padding: 1.1rem;
		font-family: var(--bc-font);
		color: var(--bc-ink);
		pointer-events: none;
		transform: scale(var(--scale, 1));
		transform-origin: top center;
		display: flex;
		justify-content: center;
	}

	.status {
		align-self: flex-start;
		display: inline-block;
		background: rgba(8, 10, 16, 0.72);
		border: 1px solid rgba(255, 255, 255, 0.12);
		padding: 0.6rem 1rem;
		border-radius: 0.5rem;
		font-size: 0.95rem;
		color: #fff;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.status code {
		font-family: 'JetBrains Mono', monospace;
		font-size: 0.82em;
		background: rgba(255, 255, 255, 0.1);
		padding: 0.05em 0.35em;
		border-radius: 0.25rem;
	}

	.board {
		width: 44rem;
		max-width: 100%;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	/* ── header ─────────────────────────────────────────────── */
	.head {
		display: grid;
		grid-template-columns: 1fr auto 1fr;
		align-items: center;
		gap: 1rem;
		padding: 0.5rem 0.9rem;
		background: var(--bc-header-grad);
		border: 1px solid var(--bc-edge);
		border-radius: var(--bc-radius);
		box-shadow: 0 6px 22px rgba(0, 0, 0, 0.5);
	}
	.head-left {
		display: flex;
		align-items: baseline;
		gap: 0.6rem;
		min-width: 0;
	}
	.wordmark {
		font-size: 0.6rem;
		font-weight: 800;
		letter-spacing: var(--bc-tracking);
		color: var(--bc-accent);
		padding: 0.14rem 0.4rem;
		border: 1px solid var(--bc-edge);
		border-radius: 3px;
		white-space: nowrap;
	}
	.gametype {
		font-size: 1.05rem;
		font-weight: 800;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		white-space: nowrap;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.9);
	}
	.map {
		font-size: 0.78rem;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--bc-ink-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.head-clock {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		line-height: 1;
		padding: 0.1rem 0.9rem;
		border-left: 1px solid var(--bc-edge);
		border-right: 1px solid var(--bc-edge);
	}
	.live-dot {
		position: absolute;
		top: 2px;
		right: 6px;
		width: 7px;
		height: 7px;
		border-radius: 999px;
		background: #ff4d4d;
		box-shadow: 0 0 8px #ff4d4d;
		animation: pulse 1.4s ease-in-out infinite;
	}
	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.35;
		}
	}
	.clock {
		font-size: 1.7rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
		color: var(--bc-accent);
		text-shadow: 0 0 14px var(--bc-glow);
	}
	.clock-lbl {
		font-size: 0.5rem;
		font-weight: 700;
		letter-spacing: 0.2em;
		color: var(--bc-ink-muted);
	}
	.head-right {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.6rem;
	}
	.limit {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		line-height: 1;
	}
	.limit b {
		font-size: 1.25rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
	}
	.limit span {
		font-size: 0.5rem;
		font-weight: 700;
		letter-spacing: 0.18em;
		color: var(--bc-ink-muted);
	}
	.badge {
		font-size: 0.6rem;
		font-weight: 800;
		letter-spacing: 0.14em;
		background: var(--bc-accent);
		color: #12100a;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
	}

	/* ── team columns ───────────────────────────────────────── */
	.teams {
		display: grid;
		grid-template-columns: repeat(var(--cols, 2), 1fr);
		gap: 0.5rem;
	}
	.team {
		display: flex;
		flex-direction: column;
		background: var(--bc-panel);
		border: 1px solid var(--bc-edge);
		border-top: 3px solid var(--team);
		border-radius: var(--bc-radius);
		overflow: hidden;
		box-shadow: 0 6px 20px rgba(0, 0, 0, 0.45);
	}
	.team-head {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.3rem 0.7rem;
		background: linear-gradient(
			90deg,
			color-mix(in srgb, var(--team) 60%, rgba(6, 8, 12, 0.85)),
			rgba(6, 8, 12, 0.7)
		);
	}
	.team-name {
		font-weight: 800;
		font-size: 1rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.team-score {
		margin-left: auto;
		font-size: 1.6rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
		text-shadow: 0 1px 4px rgba(0, 0, 0, 0.95);
	}
	.rows {
		display: flex;
		flex-direction: column;
	}
	.team.ffa {
		border-top-color: var(--bc-accent);
	}

	/* ── player row ─────────────────────────────────────────── */
	.prow {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.45rem;
		padding: 0.24rem 0.6rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		transition: opacity 0.25s;
	}
	.prow:last-child {
		border-bottom: none;
	}
	.prow.dead {
		opacity: 0.42;
	}
	.rank {
		font-size: 0.72rem;
		font-weight: 700;
		color: var(--bc-ink-muted);
		width: 1rem;
		text-align: center;
		font-variant-numeric: tabular-nums;
	}
	.pname {
		flex: 1;
		min-width: 0;
	}
	.kda {
		margin-left: auto;
	}
	.chip {
		width: 10px;
		height: 10px;
		border-radius: 2px;
		box-shadow: 0 0 6px currentColor;
	}
	.pname {
		font-size: 0.9rem;
		font-weight: 600;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.pips {
		display: inline-flex;
		gap: 0.2rem;
	}
	.pip {
		font-size: 0.54rem;
		font-weight: 800;
		line-height: 1;
		padding: 0.1rem 0.2rem;
		border-radius: 2px;
	}
	.pip.os {
		background: rgba(255, 214, 102, 0.9);
		color: #2a1c00;
	}
	.pip.camo {
		background: rgba(120, 200, 255, 0.78);
		color: #021018;
	}
	.respawn {
		font-size: 0.66rem;
		color: var(--bc-ink-muted);
		font-variant-numeric: tabular-nums;
	}
	.kda {
		font-size: 0.8rem;
		font-variant-numeric: tabular-nums;
		color: var(--bc-ink-muted);
		white-space: nowrap;
	}
	.pscore {
		font-size: 1rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
		text-align: right;
		color: var(--bc-accent);
		text-shadow: 0 0 8px var(--bc-glow);
	}
	.kda.muted,
	.pscore.muted {
		opacity: 0.4;
		color: var(--bc-ink-muted);
		text-shadow: none;
	}
	.minibars {
		flex-basis: 100%;
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 2px;
		margin-top: 1px;
	}
	.mb {
		height: 3px;
		background: rgba(255, 255, 255, 0.1);
		border-radius: 999px;
		overflow: hidden;
	}
	.mb i {
		display: block;
		height: 100%;
		transition: width 0.14s linear;
	}
	.mb i.s {
		background: linear-gradient(90deg, var(--bc-accent2), var(--bc-accent));
	}
	.mb i.h {
		background: linear-gradient(90deg, #ff5470, #ff8a5c);
	}
</style>
