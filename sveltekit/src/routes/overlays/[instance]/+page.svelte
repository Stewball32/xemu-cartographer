<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildScoreboard } from '$lib/utils/overlay-view';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	const vm = $derived(buildScoreboard(feed.game, feed.tick));

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			// game = roster + cumulative stats; tick = alive / health / shields.
			classes: ['game', 'tick']
		})
	);
	onDestroy(() => feed.stop());

	function kda(p: { kills: number; deaths: number; assists: number }): string {
		return `${p.kills} / ${p.deaths} / ${p.assists}`;
	}
</script>

<svelte:head>
	<!-- OBS browser source: fully transparent canvas, no scrollbars. Neutralise
	     the theme's body background-image + ::before/::after decorations on top
	     of the plain background, or they composite into the capture. -->
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

<div class="overlay" style="--scale: {data.scale}">
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
			{#if vm.gametype || feed.scenario?.map}
				<div class="title">
					{#if vm.gametype}<span class="gametype">{vm.gametype}</span>{/if}
					{#if feed.mock}<span class="badge">MOCK</span>{/if}
				</div>
			{/if}

			{#if vm.isTeamGame}
				{#each vm.teams as t (t.team)}
					<section class="team">
						<header class="team-head" style="--team: {t.color}">
							<span class="team-tick"></span>
							<span class="team-name">{t.name}</span>
							{#if vm.hasScores}<span class="team-score">{t.score}</span>{/if}
						</header>
						{#each t.players as p (p.index)}
							{@render row(p, t.color)}
						{/each}
					</section>
				{/each}
			{:else}
				<section class="team">
					{#each vm.players as p, i (p.index)}
						{@render row(p, '#9aa4b2', i + 1)}
					{/each}
				</section>
			{/if}
		</div>
	{/if}
</div>

{#snippet row(
	p: {
		index: number;
		name: string;
		alive: boolean;
		respawnIn: number | null;
		health: number;
		shields: number;
		hasOvershield: boolean;
		hasCamo: boolean;
		kills: number;
		deaths: number;
		assists: number;
	},
	color: string,
	rank?: number
)}
	<div class="prow" class:dead={vm.hasTick && !p.alive} style="--team: {color}">
		<span class="accent"></span>
		<div class="pmain">
			<div class="pline">
				{#if rank}<span class="rank">{rank}</span>{/if}
				<span class="pname">{p.name}</span>
				<span class="ppow">
					{#if p.hasOvershield}<span class="pip os" title="Overshield">OS</span>{/if}
					{#if p.hasCamo}<span class="pip camo" title="Camo">C</span>{/if}
				</span>
				{#if vm.hasTick && !p.alive}
					<span class="respawn">↻{p.respawnIn ? ` ${Math.ceil(p.respawnIn / 30)}s` : ''}</span>
				{/if}
				<span class="kda" class:muted={!vm.hasScores}>
					{vm.hasScores ? kda(p) : '—'}
				</span>
			</div>
			{#if vm.hasTick}
				<div class="bars">
					<div class="bar"><span class="fill shield" style="width: {p.shields * 100}%"></span></div>
					<div class="bar"><span class="fill health" style="width: {p.health * 100}%"></span></div>
				</div>
			{/if}
		</div>
	</div>
{/snippet}

<style>
	.overlay {
		position: fixed;
		inset: 0;
		padding: 1.25rem;
		font-family: 'Rajdhani', 'Inter', system-ui, sans-serif;
		color: #fff;
		pointer-events: none;
		transform: scale(var(--scale, 1));
		transform-origin: top left;
	}

	.status {
		display: inline-block;
		background: rgba(8, 10, 16, 0.72);
		border: 1px solid rgba(255, 255, 255, 0.12);
		padding: 0.6rem 1rem;
		border-radius: 0.5rem;
		font-size: 0.95rem;
		letter-spacing: 0.01em;
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
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		width: 23rem;
	}

	.title {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.78rem;
		text-transform: uppercase;
		letter-spacing: 0.14em;
		opacity: 0.85;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.9);
	}
	.gametype {
		font-weight: 700;
	}
	.badge {
		font-size: 0.62rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		background: rgba(224, 163, 46, 0.9);
		color: #1a1300;
		padding: 0.05rem 0.4rem;
		border-radius: 0.25rem;
	}

	.team {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.team-head {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.3rem 0.6rem;
		background: linear-gradient(
			90deg,
			color-mix(in srgb, var(--team) 55%, rgba(8, 10, 16, 0.85)),
			rgba(8, 10, 16, 0.8)
		);
		border-left: 4px solid var(--team);
		border-radius: 0.35rem 0.35rem 0 0;
	}
	.team-tick {
		display: none;
	}
	.team-name {
		font-weight: 700;
		font-size: 0.95rem;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.team-score {
		margin-left: auto;
		font-size: 1.35rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95);
	}

	.prow {
		display: flex;
		background: rgba(8, 10, 16, 0.62);
		overflow: hidden;
		transition: opacity 0.25s;
	}
	.prow.dead {
		opacity: 0.45;
	}
	.accent {
		width: 4px;
		flex-shrink: 0;
		background: var(--team);
		opacity: 0.85;
	}
	.pmain {
		flex: 1;
		padding: 0.28rem 0.55rem 0.3rem;
		display: flex;
		flex-direction: column;
		gap: 0.22rem;
		min-width: 0;
	}
	.pline {
		display: flex;
		align-items: baseline;
		gap: 0.45rem;
	}
	.rank {
		font-size: 0.72rem;
		opacity: 0.6;
		width: 1rem;
		font-variant-numeric: tabular-nums;
	}
	.pname {
		font-weight: 600;
		font-size: 0.92rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.ppow {
		display: inline-flex;
		gap: 0.2rem;
	}
	.pip {
		font-size: 0.58rem;
		font-weight: 700;
		line-height: 1;
		padding: 0.12rem 0.22rem;
		border-radius: 0.2rem;
	}
	.pip.os {
		background: rgba(255, 214, 102, 0.85);
		color: #2a1c00;
	}
	.pip.camo {
		background: rgba(120, 200, 255, 0.7);
		color: #021018;
	}
	.respawn {
		font-size: 0.7rem;
		opacity: 0.8;
		font-variant-numeric: tabular-nums;
	}
	.kda {
		margin-left: auto;
		font-size: 0.82rem;
		font-variant-numeric: tabular-nums;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
		white-space: nowrap;
	}
	.kda.muted {
		opacity: 0.4;
	}

	.bars {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.bar {
		height: 3px;
		background: rgba(255, 255, 255, 0.12);
		border-radius: 999px;
		overflow: hidden;
	}
	.fill {
		display: block;
		height: 100%;
		transition: width 0.12s linear;
	}
	.fill.shield {
		background: linear-gradient(90deg, #5cc8ff, #3a7bff);
	}
	.fill.health {
		background: linear-gradient(90deg, #ff5470, #ff8a5c);
	}
</style>
