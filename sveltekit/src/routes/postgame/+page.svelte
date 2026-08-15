<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Post-game carnage report. Ported from the obs-handoff pack's postgame.html,
	// wired to cartographer's native live feed via overlay-state.
	//
	// OBS browser source: 900 wide; height scales with the roster (≈620 for an
	// 8-player FFA, ≈700 for a 4v4).
	//
	// Leave "Shutdown source when not visible" OFF: the match duration is latched
	// from the last LIVE tick (engine_tick free-runs once the game returns to the
	// menu), so a source that only wakes up after the match ends has nothing to
	// latch and shows an em dash.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import {
		applyIdentities,
		createClockLatch,
		matchState,
		matchTotals,
		overlayPlayers,
		rankPlayers
	} from '$lib/utils/overlay-state';
	import { createProfileLookup } from '$lib/stores/overlay-profiles.svelte';
	import { damageRatioOf } from '$lib/utils/overlay-split';
	import starUrl from '$lib/assets/star.png';
	import wordmarkUrl from '$lib/assets/norcal-halo.png';
	import '$lib/styles/overlay-base.css';

	const DASH = '—';
	const INFINITY = '∞';

	// Column order is shared by the header, the player rows and the team
	// aggregate chips — change it in one place.
	//   best:  'high' | 'low' — which end of the column wins the orange highlight
	//   shame: nonzero is bad, always red, never "best"
	//   dp:    decimal places; mult: render as ×N
	//   agg:   how the team chip rolls the column up ('sum' | 'max' | 'mean')
	//   n/a:   not derivable from the scrape — renders as an em dash everywhere.
	//          ACC needs shots_HIT (reads 0 live; only shots_fired is counted) and
	//          headshots are not on the wire at all. Kept as columns so the layout
	//          matches the rest of the pack and they light up for free once the
	//          offsets are hunted — see the field-availability note in
	//          overlay-state.ts.
	const COLS = [
		{ key: 'score', label: 'SCORE', w: 40, best: 'high', agg: 'sum', get: (p) => p.score },
		{ key: 'k', label: 'K', w: 30, best: 'high', agg: 'sum', get: (p) => p.kills },
		{ key: 'd', label: 'D', w: 30, best: 'low', agg: 'sum', get: (p) => p.deaths },
		{ key: 'a', label: 'A', w: 30, best: 'high', agg: 'sum', get: (p) => p.assists },
		{ key: 'kd', label: 'K/D', w: 42, best: 'high', dp: 2, agg: 'kd', get: (p) => kd(p) },
		{ key: 'acc', label: 'ACC', w: 42, dp: 1, na: true },
		{
			key: 'dmg',
			label: 'DMG',
			w: 42,
			best: 'high',
			dp: 2,
			agg: 'ratio',
			get: (p) => p.damageRatio
		},
		{
			key: 'spree',
			label: 'SPREE',
			w: 42,
			best: 'high',
			mult: true,
			agg: 'max',
			get: (p) => p.bestSpree
		},
		{ key: 'hs', label: 'HS', w: 30, na: true },
		{ key: 'melee', label: 'MELEE', w: 36, best: 'high', agg: 'sum', get: (p) => p.meleeKills },
		{ key: 'nade', label: 'NADE', w: 36, best: 'high', agg: 'sum', get: (p) => p.grenadeThrows },
		{ key: 'camo', label: 'CAMO', w: 38, best: 'high', agg: 'sum', get: (p) => p.camoPickups },
		{ key: 'os', label: 'OS', w: 28, best: 'high', agg: 'sum', get: (p) => p.osPickups },
		{ key: 'btrl', label: 'BTRL', w: 34, shame: true, agg: 'sum', get: (p) => p.betrayals },
		{ key: 'suic', label: 'SUIC', w: 34, shame: true, agg: 'sum', get: (p) => p.suicides }
	];

	function kd(p) {
		return p.deaths ? p.kills / p.deaths : p.kills;
	}

	function fmt(col, v) {
		if (col.na || v == null) return DASH;
		if (col.mult) return v > 0 ? `×${v}` : DASH;
		if (col.dp) {
			const n = Number(v);
			// An unbounded ratio (damage dealt, none taken) — see damageRatioOf.
			return Number.isFinite(n) ? n.toFixed(col.dp) : INFINITY;
		}
		return String(v);
	}

	/** Best-of per column across the WHOLE lobby, so a standout still glows in
	 * team mode. Columns where everyone sits at zero highlight nobody. */
	function bestOf(rows) {
		const out = {};
		for (const col of COLS) {
			if (col.shame || col.na || !rows.length) continue;
			const vals = rows.map((p) => Number(col.get(p)) || 0);
			out[col.key] = col.best === 'low' ? Math.min(...vals) : Math.max(...vals);
		}
		return out;
	}

	function cellClass(col, v, best) {
		if (col.na) return 'na';
		if (col.shame) return Number(v) > 0 ? 'shame' : '';
		const b = best[col.key];
		return b != null && Number(v) === b && Number(v) !== 0 ? 'best' : '';
	}

	/** Team aggregate: sum the counting columns, recompute the ratios off the
	 * sums, take the peak for spree. Summed client-side per the pack contract —
	 * the scraper never sends team aggregates.
	 *
	 * SCORE is the exception: it comes from the engine's own team_scores, not
	 * from summing the player rows. The two can disagree (objective gametypes
	 * score the team, not the scorer), and the scorebug + leaderboard both show
	 * the engine value — a post-game chip that summed rows instead would put two
	 * different numbers for the same team on the same broadcast. */
	function aggregate(rows, teamScore) {
		const out = {};
		for (const col of COLS) {
			if (col.na || col.agg === 'ratio') continue;
			const vals = rows.map((p) => Number(col.get(p)) || 0);
			if (col.agg === 'max') out[col.key] = vals.length ? Math.max(...vals) : 0;
			else if (col.agg !== 'kd') out[col.key] = vals.reduce((a, b) => a + b, 0);
		}
		out.kd = out.d ? out.k / out.d : out.k;
		// Both ratios are recomputed off the SUMS, never averaged across players:
		// a mean would weight a 3-damage skirmish the same as a 3000-damage one,
		// and one player with an unbounded ratio would poison the whole row.
		const sum = (pick) => rows.reduce((n, p) => n + (Number(pick(p)) || 0), 0);
		out.dmg = damageRatioOf(
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

	// Latch the final live clock — see the header note.
	const latch = createClockLatch();
	let duration = $state(undefined);
	$effect(() => {
		latch.observe(feed.game);
		duration = latch.duration;
	});

	// Identity resolution: ask once per newly-seen scraped name (the store keeps a
	// negative cache, so the ~30Hz roster re-derive never re-hits the endpoint).
	const lookup = createProfileLookup();
	const scraped = $derived(overlayPlayers(feed.game, feed.tick));
	$effect(() => {
		lookup.ensure(
			scraped.map((p) => p.name),
			data.mock
		);
	});
	const players = $derived(applyIdentities(scraped, lookup.all, data.names));
	const match = $derived(matchState(feed.game, feed.scenario));
	const isTeam = $derived(match.mode === 'team');
	const best = $derived(bestOf(players));
	const totals = $derived(matchTotals(players));

	const red = $derived(match.teams?.find((t) => t.id === 'red'));
	const blue = $derived(match.teams?.find((t) => t.id === 'blue'));
	const redWins = $derived(isTeam && (red?.score ?? 0) >= (blue?.score ?? 0));

	const winner = $derived(
		isTeam
			? ((redWins ? red?.name : blue?.name) ?? DASH)
			: (rankPlayers(players)[0]?.display ?? DASH)
	);

	// Winning block first, so the eye lands on the victors.
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
</script>

<svelte:head>
	<title>NorCal Halo — post-game report</title>
</svelte:head>

<div class="stage" data-anchor={data.anchor}>
	<div class="report">
		<div class="head" style="--emblem:url({starUrl})">
			<div class="col">
				<span class="verdict" class:is-red={isTeam && redWins}>VICTORY</span>
				<span class="winner">
					{winner}
					<span class="mark" style="color:{isTeam && redWins ? 'var(--nh-red)' : 'var(--nh-blue)'}"
						>✦</span
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
			<span class="c-av"></span>
			<span class="c-player">PLAYER</span>
			{#each COLS as col (col.key)}
				<span style="width:{col.w}px">{col.label}</span>
			{/each}
		</div>

		{#each blocks as block, bi (block.id)}
			{#if block.team}
				{@const agg = aggregate(block.rows, block.team.score)}
				<div class="padded">
					<div class="aggrow is-{block.id}" class:spaced={bi > 0}>
						<span class="c-name">{block.team.name}</span>
						{#each COLS as col (col.key)}
							<span class={col.key === 'score' ? 'c-score' : ''} style="width:{col.w}px"
								>{fmt(col, col.na ? null : agg[col.key])}</span
							>
						{/each}
					</div>
				</div>
			{/if}

			{#each block.rows as p, i (p.name)}
				<div class="padded">
					<div
						class="pgrow"
						style="background:{p.armor}26; border-color:{p.armor}4D; animation-delay:{(
							bi * 0.1 +
							0.15 +
							i * 0.06
						).toFixed(2)}s"
					>
						<span class="c-rank">{i + 1}</span>
						<div class="avatar" class:is-placeholder={!p.avatar}>
							<img
								src={p.avatar || starUrl}
								alt=""
								onerror={(e) => (e.currentTarget.src = starUrl)}
							/>
						</div>
						<span class="c-player">{p.display || p.name}</span>
						{#each COLS as col (col.key)}
							{@const v = col.na ? null : col.get(p)}
							<span
								class="{col.key === 'score' ? 'c-score' : ''} {cellClass(col, v, best)}"
								style="width:{col.w}px">{fmt(col, v)}</span
							>
						{/each}
					</div>
				</div>
			{/each}
		{/each}

		<div class="foot">
			<img class="wordmark" src={wordmarkUrl} alt="NorCal Halo" />
			<span>{totals.kills} KILLS</span>
			<span class="rule"></span>
			<span>{totals.shots} SHOTS FIRED</span>
			<span class="rule"></span>
			<span>{totals.nades} GRENADES THROWN</span>
			<span class="rule"></span>
			<span>{totals.damage} DAMAGE DEALT</span>
			<img class="footstar" src={starUrl} alt="" />
		</div>
	</div>
</div>

<style>
	/* Transparent canvas — see the scorebug for why both html and body are reset
	   and why body::before/::after are killed. */
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
		width: 900px;
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

	.cols {
		display: flex;
		align-items: center;
		gap: 9px;
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
		width: 16px;
	}
	.c-av {
		width: 42px;
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

	.pgrow {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 2px 10px 2px 8px;
		border-radius: 6px;
		border: 1px solid;
		animation: row-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
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
		width: 16px;
		text-align: center;
		font-size: 13px;
		font-weight: 700;
		color: var(--nh-steel);
	}
	.pgrow .c-player {
		flex: 1;
		min-width: 0;
		text-align: left;
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 11px;
		color: var(--nh-text);
		letter-spacing: 0.04em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
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
	.pgrow .shame {
		color: #ff8f85;
	}
	/* Not derivable from the scrape — muted so it reads as "no data", not zero. */
	.pgrow .na {
		color: var(--nh-mute);
	}

	.avatar {
		width: 42px;
		height: 42px;
		flex: none;
		border-radius: 50%;
		background: repeating-linear-gradient(45deg, #10152a 0 5px, #141a30 5px 10px);
		border: 1px solid rgba(159, 180, 208, 0.4);
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
	}
	/* A real avatar fills the circle; the placeholder emblem overflows it and
	   sits back at 0.85 so it reads as a placeholder rather than a photo. */
	.avatar img {
		width: 100%;
		height: 100%;
		flex: none;
		display: block;
		object-fit: cover;
	}
	.avatar.is-placeholder img {
		width: 46px;
		height: auto;
		opacity: 0.85;
	}

	/* Aggregate chip rows reuse the post-game column grid. */
	.aggrow {
		padding: 8px 10px 8px 8px;
		border-radius: 6px;
		display: flex;
		align-items: center;
		gap: 9px;
	}
	.aggrow.spaced {
		margin-top: 4px;
	}
	.aggrow.is-red {
		background: rgba(224, 82, 82, 0.28);
		border: 1px solid rgba(224, 82, 82, 0.5);
	}
	.aggrow.is-blue {
		background: rgba(61, 98, 224, 0.28);
		border: 1px solid rgba(61, 98, 224, 0.5);
	}
	.aggrow > span {
		flex: none;
		text-align: right;
		font-family: 'Lucida Console', monospace;
		font-size: 10px;
		font-variant-numeric: tabular-nums;
	}
	.aggrow.is-red > span {
		color: #ffd9d4;
	}
	.aggrow.is-blue > span {
		color: #cfdcff;
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
	.aggrow .c-score {
		width: 40px;
		font-family: Inter, sans-serif;
		font-size: 15px;
		font-weight: 700;
		color: var(--nh-text);
	}

	/* Footer carries the wordmark left, totals centre, emblem right. */
	.foot {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 26px;
		padding: 12px 18px 8px;
		border-top: var(--nh-hairline);
		margin-top: 5px;
	}
	.foot span {
		font-family: 'Lucida Console', monospace;
		font-size: 11px;
		color: var(--nh-steel);
	}
	.foot .rule {
		width: 1px;
		height: 12px;
		background: rgba(255, 255, 255, 0.14);
	}
	.wordmark {
		height: 12px;
		width: auto;
		flex: none;
		opacity: 0.55;
		display: block;
	}
	.footstar {
		height: 16px;
		width: 16px;
		flex: none;
		opacity: 0.5;
		display: block;
		object-fit: contain;
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
		.pgrow {
			animation: none;
		}
	}
</style>
