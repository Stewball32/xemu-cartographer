<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Leaderboard — redesign (CL-01/08/10/12/15/18). Every row leads with the
	// shared NamePlate at 0.63×. FFA: armor-tinted rows on a 52px pitch;
	// ordinals sit on a FIXED slot rail (they belong to the position, not the
	// row — no label flips mid-glide). Team: ONE deep-panel container per team (the scorebug/POV
	// panels), ordered by team score, 1ST/2ND on the chip (both 1ST on a tie),
	// hairline under it, flat rows on a tighter 47px pitch. One lobby-best
	// score takes orange (FFA ties highlight all leaders). Shield/health bars,
	// the spree column and the camo overlay are retired.
	// Slot-change motion: rows glide via the rowslot top transition (0.7s on
	// the DS spring curve); team containers FLIP-swap when the score order
	// flips them (animate:flip, same curve); a row whose slot changed pulses
	// brightness once (ramp to 1.3 and back, 0.9s) so the move reads.
	//
	// OBS browser source: 430 wide; height scales with the roster.
	import { onMount, onDestroy } from 'svelte';
	import { flip } from 'svelte/animate';
	import { quintOut } from 'svelte/easing';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import {
		applyIdentities,
		matchState,
		overlayPlayers,
		rankPlayers
	} from '$lib/utils/overlay-state';
	import { createProfileLookup } from '$lib/stores/overlay-profiles.svelte';
	import { ordinal, themes } from '$lib/overlay/themes.js';
	import LeaderboardRow from '$lib/overlay/LeaderboardRow.svelte';
	import starUrl from '$lib/assets/star.png';
	import '$lib/styles/overlay-base.css';

	const ROW_PITCH = 52; // FFA: 46px card + 3px padding top and bottom
	const TEAM_PITCH = 47; // team containers: tighter intra-team spacing

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

	const red = $derived(match.teams?.find((t) => t.id === 'red'));
	const blue = $derived(match.teams?.find((t) => t.id === 'blue'));

	const ffaRanked = $derived(isTeam ? [] : rankPlayers(players));

	// Team containers ordered by team score; a tie chips both 1ST.
	const tblocks = $derived(
		isTeam && red && blue
			? [
					{ id: 'red', team: red, rows: rankPlayers(players.filter((p) => p.team === 'red')) },
					{ id: 'blue', team: blue, rows: rankPlayers(players.filter((p) => p.team === 'blue')) }
				].sort((a, b) => b.team.score - a.team.score)
			: []
	);
	const teamsTied = $derived(
		tblocks.length === 2 && tblocks[0].team.score === tblocks[1].team.score
	);

	// One lobby-best score highlights (CL-10) — ties highlight all leaders.
	const topScore = $derived(Math.max(0, ...players.map((p) => p.score ?? 0)));
	const isLeader = (p) => topScore > 0 && (p.score ?? 0) === topScore;

	// Best-of-stat orange per K/D/A column (CL-01), across the whole lobby.
	const best = $derived(
		players.length
			? {
					k: Math.max(...players.map((p) => p.kills ?? 0)),
					d: Math.min(...players.map((p) => p.deaths ?? 0)),
					a: Math.max(...players.map((p) => p.assists ?? 0))
				}
			: null
	);

	// Mover choreography — the riser travels in front and pulses; displaced
	// rows go behind and start 120ms later (kicked out of the slot). Slot ids
	// are per-list (team-scoped), so a container swap alone doesn't trigger.
	// Directions derive DURING render so class + top land in the same flush
	// (transition-delay set after a transition starts wouldn't apply).
	const rm =
		typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;
	const slots = $derived(
		isTeam
			? tblocks.flatMap((b) => b.rows.map((p, i) => [p.name, b.id, i]))
			: ffaRanked.map((p, i) => [p.name, 'f', i])
	);
	let prevSlot = {};
	const moved = $derived.by(() => {
		const cur = Object.fromEntries(slots.map(([n, g, i]) => [n, { g, i }]));
		const m = new Map();
		if (!rm)
			for (const n of Object.keys(cur)) {
				const p = prevSlot[n];
				if (p && p.g === cur[n].g && p.i !== cur[n].i) m.set(n, cur[n].i < p.i ? 'up' : 'down');
			}
		prevSlot = cur;
		return m;
	});
	// STABLE DOM order (name-sorted) — only `top` changes per rank. A keyed
	// each that re-sorts would reinsert the node and reset its running top
	// transition — the row would teleport instead of glide.
	const stableSlots = (rows) =>
		rows.map((p, i) => ({ p, i })).sort((a, b) => a.p.name.localeCompare(b.p.name));
</script>

<svelte:head>
	<title>NorCal Halo — leaderboard</title>
</svelte:head>

<div class="stage" data-anchor={data.anchor}>
	<div class="board">
		<div class="head" class:has-teams={isTeam} style="--emblem:url({starUrl})">
			<div class="head-id">
				<span class="gametype">{match.gametype}</span>
				<span class="map">{match.map}</span>
			</div>
			<span class="clock">{match.clock ?? '0:00'}</span>
		</div>

		{#if isTeam}
			{#each tblocks as b, bi (b.id)}
				<!-- Coincidence caveat: animate:flip reorders the two teambox nodes,
				     which resets any intra-team row glide RUNNING in that same tick
				     (the row teleports). Rare with real data — a team-lead flip and a
				     same-team rank swap landing on one tick — so accepted; if it ever
				     bothers, defer the row's top write until the flip settles. -->
				<div
					class="teambox"
					class:spaced={bi > 0}
					animate:flip={{
						duration: rm ? 0 : 700,
						delay: rm || b.id === tblocks[0].id ? 0 : 120,
						easing: quintOut
					}}
					class:rising={b.id === tblocks[0].id}
					style="border-color:{themes[b.id].border}; background:{themes[b.id].panel}"
				>
					<div class="teamchip">
						<span class="chip-ord">{teamsTied ? '1ST' : ordinal(bi)}</span>
						<span class="teamchip-name">{b.team.name}</span>
						<span class="teamchip-score">{b.team.score}</span>
					</div>
					<div class="chip-rule"></div>
					<div class="rows" style="height:{b.rows.length * TEAM_PITCH}px">
						{#each stableSlots(b.rows) as s (s.p.name)}
							<div
								class="rowslot"
								class:up={moved.get(s.p.name) === 'up'}
								class:down={moved.get(s.p.name) === 'down'}
								style="top:{s.i * TEAM_PITCH}px"
							>
								<LeaderboardRow player={s.p} flat leader={isLeader(s.p)} {best} />
							</div>
						{/each}
					</div>
				</div>
			{/each}
		{:else}
			<div class="rows" style="height:{ffaRanked.length * ROW_PITCH}px">
				{#each ffaRanked as _, i}
					<span class="rail-ord" style="top:{i * ROW_PITCH}px; height:{ROW_PITCH}px"
						>{ordinal(i)}</span
					>
				{/each}
				{#each stableSlots(ffaRanked) as s (s.p.name)}
					<div
						class="rowslot railed"
						class:up={moved.get(s.p.name) === 'up'}
						class:down={moved.get(s.p.name) === 'down'}
						style="top:{s.i * ROW_PITCH}px"
					>
						<LeaderboardRow player={s.p} leader={isLeader(s.p)} {best} />
					</div>
				{/each}
			</div>
		{/if}
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

	.board {
		width: 430px;
		border-radius: 12px;
		overflow: hidden;
		border: var(--nh-edge);
		box-shadow: var(--nh-lift);
		background: var(--nh-panel);
		padding-bottom: 5px;
		font-family: Inter, system-ui, sans-serif;
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 13px 16px 11px;
		border-bottom: var(--nh-hairline);
		background:
			linear-gradient(rgba(11, 14, 26, 0.66), rgba(11, 14, 26, 0.66)),
			var(--emblem) center / 165px no-repeat;
	}
	.head.has-teams {
		margin-bottom: 5px;
	}
	.head-id {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.gametype {
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: var(--nh-steel);
	}
	.map {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 14px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.06em;
	}
	.clock {
		font-family: 'Lucida Console', monospace;
		font-size: 24px;
		font-weight: 700;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}

	.rows {
		position: relative;
	}
	.rowslot {
		position: absolute;
		left: 0;
		right: 0;
		padding: 3px 8px;
		transition: top 0.7s cubic-bezier(0.22, 1, 0.36, 1);
	}
	/* Directional layering: the riser rides in front and pulses; displaced
	   rows go behind and start 120ms later — kicked out, not teleported. */
	.rowslot.up {
		animation: row-pulse 0.9s ease-in-out;
		z-index: 2;
	}
	.rowslot.down {
		z-index: 0;
		transition-delay: 0.12s;
	}
	/* FFA: ordinals live on this fixed rail — the slot owns its rank label,
	   so nothing flips mid-glide; rows inset to clear it (43 + 10 = the old
	   8 + 10 + 26 + 9 rank column). */
	.rowslot.railed {
		padding-left: 43px;
	}
	.rail-ord {
		position: absolute;
		left: 18px;
		width: 26px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 9.5px;
		font-weight: 700;
		letter-spacing: 0.06em;
		color: var(--nh-steel);
		font-variant-numeric: tabular-nums;
	}
	@keyframes row-pulse {
		0% {
			filter: brightness(1);
		}
		30% {
			filter: brightness(1.32);
		}
		100% {
			filter: brightness(1);
		}
	}

	/* One deep panel per team — the whole div takes the team color (CL-16). */
	.teambox {
		position: relative;
		margin: 3px 8px;
		border: 1px solid;
		border-radius: 8px;
		padding-bottom: 2px;
	}
	/* The leading (rising) box layers above during a swap. */
	.teambox.rising {
		z-index: 2;
	}
	.teambox.spaced {
		margin-top: 7px;
	}
	.teamchip {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 11px 20px 8px;
	}
	.chip-ord {
		flex: none;
		font-weight: 700;
		font-size: 9px;
		letter-spacing: 0.18em;
		color: #cdd9ea;
	}
	.teamchip-name {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 12px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.04em;
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	/* Inter numerals (CL-12). */
	.teamchip-score {
		font-family: Inter, system-ui, sans-serif;
		font-weight: 800;
		font-size: 20px;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}
	.chip-rule {
		height: 1px;
		background: rgba(255, 255, 255, 0.12);
		margin: 0 8px 3px;
	}

	@media (prefers-reduced-motion: reduce) {
		.rowslot {
			transition: none;
		}
		.rowslot.up {
			animation: none;
		}
		.rowslot.down {
			transition-delay: 0s;
		}
	}
</style>
