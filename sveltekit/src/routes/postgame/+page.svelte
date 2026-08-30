<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Post-game carnage report — redesign (CL-03/05/16/18 + stat overhaul).
	// 868 wide. 14-column ledger on nameplate rows; deep-panel team containers
	// with ordinal chips (both 1ST on a tie → FINAL · TIE GAME banner); footer
	// is the NORCAL/HALO lockup + a seamless superlatives marquee.
	//
	// Columns: SCORE K D A K/D SHOTS ACC DMG DMG+ DMG- SPR AC OS KP
	//   DMG   damage dealt ÷ taken, 1dp, capped 99.9
	//   DMG+  raw damage dealt (5 digits, no decimal) · DMG- taken, best = low
	//   SPR   highest spree of the game, capped 99, no ×
	//   AC/OS camo / overshield pickups
	//   KP    kill participation = (K+A) ÷ team kills (lobby kills in FFA),
	//         1dp capped 100 — trailing .0 trimmed only at exactly 100
	// HS/MELEE/NADE/BTRL/SUIC are retired from the ledger — the marquee
	// surfaces them as awards instead.
	//
	// Leave "Shutdown source when not visible" OFF — the duration latch needs
	// the last LIVE tick.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import {
		applyIdentities,
		createClockLatch,
		matchState,
		overlayPlayers,
		rankPlayers
	} from '$lib/utils/overlay-state';
	import { createProfileLookup } from '$lib/stores/overlay-profiles.svelte';
	import { damageRatioOf } from '$lib/utils/overlay-split';
	import { ordinal, themes } from '$lib/overlay/themes.js';
	import NamePlate from '$lib/overlay/NamePlate.svelte';
	import starUrl from '$lib/assets/star.png';
	import '$lib/styles/overlay-base.css';

	const DASH = '—';

	const COLS = [
		{ key: 'score', label: 'SCORE', w: 40, best: 'high', agg: 'sum', get: (p) => p.score },
		{ key: 'k', label: 'K', w: 26, best: 'high', agg: 'sum', get: (p) => p.kills },
		{ key: 'd', label: 'D', w: 26, best: 'low', agg: 'sum', get: (p) => p.deaths },
		{ key: 'a', label: 'A', w: 26, best: 'high', agg: 'sum', get: (p) => p.assists },
		{ key: 'kd', label: 'K/D', w: 38, best: 'high', dp: 2, agg: 'kd', get: (p) => kd(p) },
		{ key: 'shots', label: 'SHOTS', w: 38, best: 'high', agg: 'sum', get: (p) => p.shotsFired },
		// ACC lights up once the shots_hit offsets land (CL-05); dash until then.
		{ key: 'acc', label: 'ACC', w: 38, best: 'high', dp: 1, agg: 'none', get: (p) => p.acc },
		{
			key: 'dmgr',
			label: 'DMG',
			w: 34,
			best: 'high',
			dp: 1,
			cap: 99.9,
			agg: 'dmgr',
			get: (p) => p.damageRatio
		},
		{ key: 'dmg', label: 'DMG+', w: 44, best: 'high', agg: 'sum', get: (p) => p.damageDealt },
		{ key: 'dmgt', label: 'DMG-', w: 44, best: 'low', agg: 'sum', get: (p) => p.damageTaken },
		{
			key: 'spree',
			label: 'SPR',
			w: 26,
			best: 'high',
			cap99: true,
			agg: 'max',
			get: (p) => p.bestSpree
		},
		{ key: 'ac', label: 'AC', w: 26, best: 'high', agg: 'sum', get: (p) => p.camoPickups },
		{ key: 'os', label: 'OS', w: 26, best: 'high', agg: 'sum', get: (p) => p.osPickups },
		{
			key: 'kp',
			label: 'KP',
			w: 38,
			best: 'high',
			dp: 1,
			cap: 100,
			trim: true,
			agg: 'kp',
			get: (p) => p.kp
		}
	];

	function kd(p) {
		return p.deaths ? p.kills / p.deaths : p.kills;
	}

	function fmt(col, v) {
		if (v == null) return DASH;
		if (col.dp) {
			let n = Number(v);
			if (!Number.isFinite(n)) n = col.cap ?? Infinity;
			if (col.cap != null) n = Math.min(n, col.cap);
			if (!Number.isFinite(n)) return '∞';
			return col.trim && n === col.cap ? String(col.cap) : n.toFixed(col.dp);
		}
		if (col.cap99) return String(Math.min(99, Number(v) || 0));
		return String(v);
	}

	/** Best-of per column across the WHOLE lobby, so a standout still glows in
	 * team mode. Columns where everyone sits at zero highlight nobody. */
	function bestOf(rows) {
		const out = {};
		for (const col of COLS) {
			if (!rows.length) continue;
			const vals = rows.map((p) => Number(col.get(p)) || 0);
			out[col.key] = col.best === 'low' ? Math.min(...vals) : Math.max(...vals);
		}
		return out;
	}

	function cellClass(col, v, best) {
		if (v == null) return 'na';
		const b = best[col.key];
		return b != null && Number(v) === b && Number(v) !== 0 ? 'best' : '';
	}

	/** Team aggregate: counting columns summed, K/D · DMG · KP recomputed from
	 * the sums (never averaged), SPR takes the peak. SCORE stays the engine's
	 * own team_scores — the scorebug must agree with this row. */
	function aggregate(rows, teamScore) {
		const out = {};
		for (const col of COLS) {
			if (col.agg === 'none' || col.agg === 'dmgr' || col.agg === 'kd' || col.agg === 'kp')
				continue;
			const vals = rows.map((p) => Number(col.get(p)) || 0);
			out[col.key] =
				col.agg === 'max' ? (vals.length ? Math.max(...vals) : 0) : vals.reduce((a, b) => a + b, 0);
		}
		out.kd = out.d ? out.k / out.d : out.k;
		out.kp = out.k ? Math.min(100, ((out.k + (out.a || 0)) / out.k) * 100) : 0;
		const sum = (pick) => rows.reduce((n, p) => n + (Number(pick(p)) || 0), 0);
		out.dmgr = damageRatioOf(
			sum((p) => p.damageDealt),
			sum((p) => p.damageTaken)
		);
		if (teamScore != null) out.score = teamScore;
		return out;
	}

	let { data } = $props();
	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			console: data.console,
			mock: data.mock,
			classes: ['game', 'tick', 'scenario']
		})
	);
	onDestroy(() => feed.stop());

	const latch = createClockLatch();
	let duration = $state(undefined);
	$effect(() => {
		latch.observe(feed.game);
		duration = latch.duration;
	});

	const lookup = createProfileLookup();
	const scraped = $derived(overlayPlayers(feed.game, feed.tick));
	$effect(() => {
		lookup.ensure(
			scraped.map((p) => p.name),
			data.mock
		);
	});
	const identified = $derived(applyIdentities(scraped, lookup.all, data.names));
	const match = $derived(matchState(feed.game, feed.scenario));
	const isTeam = $derived(match.mode === 'team');

	// KP — kill participation, computed client-side (CL: post-game stats).
	const players = $derived.by(() => {
		const kTot = {};
		for (const p of identified) {
			const t = isTeam ? p.team : 'all';
			kTot[t] = (kTot[t] || 0) + (p.kills || 0);
		}
		return identified.map((p) => ({
			...p,
			kp: Math.min(
				100,
				(((p.kills || 0) + (p.assists || 0)) / ((isTeam ? kTot[p.team] : kTot.all) || 1)) * 100
			)
		}));
	});
	const best = $derived(bestOf(players));

	const red = $derived(match.teams?.find((t) => t.id === 'red'));
	const blue = $derived(match.teams?.find((t) => t.id === 'blue'));
	const tiedT = $derived(isTeam && (red?.score ?? 0) === (blue?.score ?? 0));
	const redWins = $derived(isTeam && (red?.score ?? 0) >= (blue?.score ?? 0));

	const winner = $derived(
		isTeam
			? ((redWins ? red?.name : blue?.name) ?? DASH)
			: (rankPlayers(players)[0]?.display ?? DASH)
	);

	// Winning container first; a tie keeps red first but chips both 1ST (CL-03).
	const blocks = $derived(
		!isTeam
			? [{ id: 'ffa', rows: rankPlayers(players) }]
			: (redWins
					? [
							{ id: 'red', team: red, rows: players.filter((p) => p.team === 'red') },
							{ id: 'blue', team: blue, rows: players.filter((p) => p.team === 'blue') }
						]
					: [
							{ id: 'blue', team: blue, rows: players.filter((p) => p.team === 'blue') },
							{ id: 'red', team: red, rows: players.filter((p) => p.team === 'red') }
						]
				).map((b) => ({ ...b, rows: rankPlayers(b.rows) }))
	);

	// Footer superlatives (marquee) — computed awards for stats no longer in
	// the ledger. DOUBLE AGENT only appears when someone actually earned it.
	const awards = $derived.by(() => {
		const ps = players;
		if (!ps.length) return [];
		const top = (pick, dir) =>
			[...ps].sort((a, b) => (dir === 'low' ? pick(a) - pick(b) : pick(b) - pick(a)))[0];
		const fin = (v) => Number.isFinite(v);
		const dpk = top((p) => (p.kills > 0 ? p.damageDealt / p.kills : Infinity), 'low');
		const dpd = top((p) => (p.deaths > 0 ? p.damageTaken / p.deaths : 0));
		const spk = top((p) => (p.kills > 0 ? p.shotsFired / p.kills : Infinity), 'low');
		const melee = top((p) => p.meleeKills || 0);
		const nades = top((p) => p.grenadeThrows || 0);
		const betray = top((p) => p.betrayals || 0);
		return [
			dpk.kills > 0 && fin(dpk.damageDealt / dpk.kills)
				? ['EFFICIENT ASSASSIN', dpk, `${Math.round(dpk.damageDealt / dpk.kills)} DAMAGE PER KILL`]
				: null,
			dpd.deaths > 0
				? ['LUCKIEST BASTARD', dpd, `${Math.round(dpd.damageTaken / dpd.deaths)} DAMAGE PER DEATH`]
				: null,
			spk.kills > 0 && fin(spk.shotsFired / spk.kills)
				? ['BUDGET KILLER', spk, `${(spk.shotsFired / spk.kills).toFixed(1)} SHOTS PER KILL`]
				: null,
			(melee.meleeKills || 0) > 0
				? ['PUNCH DRUNK', melee, `${melee.meleeKills} MELEE KILLS`]
				: null,
			(nades.grenadeThrows || 0) > 0
				? ['DEMO EXPERT', nades, `${nades.grenadeThrows} GRENADES THROWN`]
				: null,
			(betray.betrayals || 0) > 0
				? [
						'DOUBLE AGENT',
						betray,
						`${betray.betrayals} ${betray.betrayals > 1 ? 'BETRAYALS' : 'BETRAYAL'}`
					]
				: null
		].filter(Boolean);
	});
</script>

<svelte:head>
	<title>NorCal Halo — post-game report</title>
</svelte:head>

<div class="stage" data-anchor={data.anchor}>
	<div class="report">
		<div class="head" style="--emblem:url({starUrl})">
			<div class="col">
				<span class="verdict" class:is-red={isTeam && redWins && !tiedT}
					>{tiedT ? 'FINAL' : 'VICTORY'}</span
				>
				<span class="winner">
					{tiedT ? 'TIE GAME' : winner}
					<span
						class="mark"
						style="color:{tiedT
							? 'var(--nh-blue)'
							: isTeam && redWins
								? 'var(--nh-red)'
								: 'var(--nh-blue)'}">✦</span
					>
				</span>
			</div>
			<div class="col right">
				<span class="rules">{match.rules}</span>
				<span class="rmap">{match.map}</span>
				<span class="rtime">MATCH TIME {duration ?? DASH}</span>
			</div>
		</div>

		<div class="cols">
			<span class="c-pad"></span>
			<span class="c-player">PLAYER</span>
			{#each COLS as col (col.key)}
				<span style="width:{col.w}px">{col.label}</span>
			{/each}
		</div>

		{#each blocks as block, bi (block.id)}
			{#if block.team}
				{@const agg = aggregate(block.rows, block.team.score)}
				<div
					class="teambox"
					class:spaced={bi > 0}
					style="border-color:{themes[block.id].border}; background:{themes[block.id].panel}"
				>
					<div class="padded tight">
						<div class="aggrow">
							<span class="agg-ord">{tiedT ? '1ST' : ordinal(bi)}</span>
							<span class="c-name">{block.team.name}</span>
							{#each COLS as col (col.key)}
								<span class={col.key === 'score' ? 'c-score' : 'c-agg'} style="width:{col.w}px"
									>{fmt(col, col.agg === 'none' ? null : agg[col.key])}</span
								>
							{/each}
						</div>
					</div>
					<div class="agg-rule"></div>
					{#each block.rows as p, i (p.name)}
						<div class="padded tight">
							<div
								class="pgrow flat"
								style="animation-delay:{(bi * 0.1 + 0.15 + i * 0.06).toFixed(2)}s"
							>
								<NamePlate player={p} h={28} showMotto={false} />
								<span class="grow"></span>
								{#each COLS as col (col.key)}
									{@const v = col.get(p)}
									<span
										class="{col.key === 'score' ? 'c-score' : ''} {cellClass(col, v, best)}"
										style="width:{col.w}px">{fmt(col, v)}</span
									>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			{:else}
				{#each block.rows as p, i (p.name)}
					<div class="padded">
						<div
							class="pgrow"
							style="background:{p.armor}26; border-color:{p.armor}4D; animation-delay:{(
								0.15 +
								i * 0.06
							).toFixed(2)}s"
						>
							<span class="c-rank">{ordinal(i)}</span>
							<NamePlate player={p} h={28} showMotto={false} />
							<span class="grow"></span>
							{#each COLS as col (col.key)}
								{@const v = col.get(p)}
								<span
									class="{col.key === 'score' ? 'c-score' : ''} {cellClass(col, v, best)}"
									style="width:{col.w}px">{fmt(col, v)}</span
								>
							{/each}
						</div>
					</div>
				{/each}
			{/if}
		{/each}

		<div class="foot">
			<div class="lockup">
				<span class="lk-norcal">NORCAL</span>
				<span class="lk-halo">HALO</span>
			</div>
			<div class="marq-clip">
				<div class="marq">
					{#each [0, 1] as r (r)}
						<div class="half" aria-hidden={r === 1}>
							{#each awards as it, i (i)}
								<div class="award">
									<span class="aw-title">{it[0]}</span>
									<NamePlate player={it[1]} h={30} showMotto={false} />
									<span class="aw-stat">{it[2]}</span>
								</div>
							{/each}
						</div>
					{/each}
				</div>
			</div>
			<img class="footstar" src={starUrl} alt="" />
		</div>
	</div>
</div>

<style>
	/* Transparent canvas — see the scorebug. */
	:global(html, body) {
		margin: 0;
		padding: 0;
		background: transparent !important;
		background-image: none !important;
		overflow: hidden;
	}
	:global(body::before, body::after) {
		display: none !important;
		content: none !important;
	}

	.stage[data-anchor='center'] {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100vw;
		height: 100vh;
	}

	.report {
		width: 868px;
		border-radius: 12px;
		overflow: hidden;
		border: var(--nh-edge);
		box-shadow: var(--nh-lift);
		background: var(--nh-panel);
		padding-bottom: 6px;
		font-family: Inter, system-ui, sans-serif;
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 18px 22px 15px;
		border-bottom: var(--nh-hairline);
		animation: row-in 0.45s cubic-bezier(0.22, 1, 0.36, 1) both;
		background:
			linear-gradient(rgba(11, 14, 26, 0.68), rgba(11, 14, 26, 0.68)),
			var(--emblem) center / 230px no-repeat;
	}
	.col {
		display: flex;
		flex-direction: column;
		gap: 5px;
	}
	.col.right {
		align-items: flex-end;
		gap: 4px;
	}
	.verdict {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.34em;
		color: var(--nh-orange);
	}
	.verdict.is-red {
		color: #ff8f85;
	}
	.winner {
		font-family: Orbitron, sans-serif;
		font-weight: 800;
		font-size: 24px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.05em;
	}
	.rules {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: var(--nh-steel);
	}
	.rmap {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 14px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.06em;
	}
	.rtime {
		font-family: 'Lucida Console', monospace;
		font-size: 12px;
		color: var(--nh-steel);
	}

	/* Shared 10px gutter across header, rows and team rows. */
	.cols {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 9px 18px 5px 16px;
	}
	.cols span {
		flex: none;
		text-align: right;
		font-size: 8px;
		font-weight: 700;
		letter-spacing: 0.18em;
		color: var(--nh-mute);
	}
	.c-pad {
		width: 34px;
	}
	.cols .c-player {
		flex: 1;
		min-width: 0;
		text-align: left;
		letter-spacing: 0.22em;
	}

	.padded {
		padding: 3px 8px;
	}
	.padded.tight {
		padding: 1px 3px;
	}
	/* The `.pgrow > span` blanket below is more specific than a bare `.grow`
	   and later in source, so it collapsed the player rows' spacer spans and
	   packed the 14 stat columns left, out from under their headers. The
	   header row escapes it only because `.cols .c-player` happens to
	   out-specify the same blanket. */
	.grow,
	.pgrow > span.grow {
		flex: 1;
		min-width: 0;
	}

	.pgrow {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 2px 10px 2px 8px;
		border-radius: 6px;
		border: 1px solid;
		animation: row-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
	}
	.pgrow.flat {
		background: transparent;
		border-color: transparent;
	}
	.pgrow > span {
		flex: none;
		text-align: right;
		font-size: 13px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: var(--nh-dim);
	}
	.pgrow .c-rank {
		width: 34px;
		text-align: center;
		font-size: 9.5px;
		font-weight: 700;
		letter-spacing: 0.04em;
		color: var(--nh-steel);
	}
	.pgrow .c-score {
		width: 40px;
		font-size: 15px;
		font-weight: 700;
	}
	/* Selection Orange means "best" and nothing else. */
	.pgrow .best {
		color: var(--nh-orange);
	}
	.pgrow .na {
		color: var(--nh-mute);
	}

	/* Team containers — the whole div takes the team color (CL-16). */
	.teambox {
		margin: 3px 4px;
		border: 1px solid;
		border-radius: 8px;
		padding-bottom: 1px;
	}
	.teambox.spaced {
		margin-top: 7px;
	}
	.aggrow {
		padding: 8px 10px 8px 8px;
		border-radius: 6px;
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.agg-ord {
		flex: none;
		font-weight: 700;
		font-size: 9px;
		letter-spacing: 0.18em;
		color: #cdd9ea;
	}
	.aggrow .c-name {
		flex: 1;
		min-width: 0;
		text-align: left;
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 12px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.04em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	/* Team numerals in Inter tabular (CL-12), smaller than score. */
	.aggrow .c-agg {
		flex: none;
		text-align: right;
		font-family: Inter, system-ui, sans-serif;
		font-size: 10px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: var(--nh-dim);
	}
	.aggrow .c-score {
		flex: none;
		text-align: right;
		width: 40px;
		font-family: Inter, sans-serif;
		font-size: 15px;
		font-weight: 700;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}
	.agg-rule {
		height: 1px;
		background: rgba(255, 255, 255, 0.12);
		margin: 0 8px 2px;
	}

	/* Footer — lockup left, superlatives marquee centre, ✦ right. */
	.foot {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 6px 12px 5px;
		border-top: var(--nh-hairline);
		margin-top: 5px;
	}
	.lockup {
		display: flex;
		flex-direction: column;
		flex: none;
		line-height: 1;
	}
	.lk-norcal {
		font-family: Orbitron, sans-serif;
		font-weight: 800;
		font-size: 13px;
		letter-spacing: 0.08em;
		color: #e8ecf5;
	}
	.lk-halo {
		font-family: 'Highway Gothic', Inter, sans-serif;
		font-size: 10px;
		letter-spacing: 0.42em;
		color: #9fb4d0;
		margin-top: 3px;
	}
	.marq-clip {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		mask-image: linear-gradient(90deg, transparent, #000 5%, #000 95%, transparent);
		-webkit-mask-image: linear-gradient(90deg, transparent, #000 5%, #000 95%, transparent);
	}
	.marq {
		display: inline-flex;
		white-space: nowrap;
		animation: marquee 34s linear infinite;
	}
	.marq .half {
		display: inline-flex;
		gap: 34px;
		padding-right: 34px;
	}
	.award {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 3px;
	}
	.aw-title {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 11px;
		letter-spacing: 0.14em;
		color: #e8ecf5;
	}
	.aw-stat {
		font-family: Inter, system-ui, sans-serif;
		font-size: 10px;
		font-weight: 500;
		color: #9fb4d0;
		font-variant-numeric: tabular-nums;
		letter-spacing: 0.06em;
	}
	.footstar {
		height: 16px;
		width: 16px;
		flex: none;
		opacity: 0.5;
		display: block;
		object-fit: contain;
	}

	@keyframes marquee {
		from {
			transform: translateX(0);
		}
		to {
			transform: translateX(-50%);
		}
	}
	@keyframes row-in {
		from {
			opacity: 0;
			transform: translateY(16px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.head,
		.pgrow,
		.marq {
			animation: none;
		}
	}
</style>
