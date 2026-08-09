<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked.
	// Rewired to cartographer's native live feed (overlay token + instance),
	// mapped to the pack's match/players shape by overlay-state. Postgame reads
	// the final live roster (previous_game replays it); stats the wire doesn't
	// carry yet (damage, headshots, melee/grenade, pickups) render as 0/—.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { matchState, overlayPlayers } from '$lib/utils/overlay-state';
	import { ORANGE, TEAM_HEX, tint } from '$lib/overlay/themes.js';

	let { data } = $props();
	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			classes: ['game', 'tick', 'scenario']
		})
	);
	onDestroy(() => feed.stop());

	const players = $derived(
		overlayPlayers(feed.game, feed.tick).map((p) => ({ ...p, name: data.names[p.name] ?? p.name }))
	);
	const match = $derived(matchState(feed.game, feed.scenario));

	// Column spec: [key, width(px), defaultColor]. Score/K/D/A brighter; ratios dimmer.
	const cols = [
		['score', 40, '#e8ecf5'],
		['k', 30, '#e8ecf5'],
		['d', 30, '#e8ecf5'],
		['a', 30, '#e8ecf5'],
		['kd', 42, '#cdd9ea'],
		['acc', 42, '#cdd9ea'],
		['dmg', 42, '#cdd9ea'],
		['spree', 42, '#cdd9ea'],
		['hs', 30, '#cdd9ea'],
		['melee', 36, '#cdd9ea'],
		['nade', 36, '#cdd9ea'],
		['camoP', 38, '#cdd9ea'],
		['os', 28, '#cdd9ea'],
		['btrl', 34, '#cdd9ea'],
		['suic', 34, '#cdd9ea']
	];
	const headers = {
		score: 'SCORE',
		k: 'K',
		d: 'D',
		a: 'A',
		kd: 'K/D',
		acc: 'ACC',
		dmg: 'DMG',
		spree: 'SPREE',
		hs: 'HS',
		melee: 'MELEE',
		nade: 'NADE',
		camoP: 'CAMO',
		os: 'OS',
		btrl: 'BTRL',
		suic: 'SUIC'
	};
	// Best-of-stat direction: 'hi' = highest wins orange, 'lo' = lowest; shame cols get red-when-nonzero instead.
	const bestDir = {
		score: 'hi',
		k: 'hi',
		d: 'lo',
		a: 'hi',
		kd: 'hi',
		acc: 'hi',
		dmg: 'hi',
		spree: 'hi',
		hs: 'hi',
		melee: 'hi',
		nade: 'hi',
		camoP: 'hi',
		os: 'hi'
	};

	const toRow = (p) => ({
		name: p.name,
		team: p.team,
		armor: p.armor ?? '#8a93a8',
		avatar: p.avatar,
		score: p.score ?? 0,
		k: p.kills ?? 0,
		d: p.deaths ?? 0,
		a: p.assists ?? 0,
		kdN: (p.kills ?? 0) / Math.max(p.deaths ?? 0, 1),
		accN: p.shots ? ((p.hits ?? 0) / p.shots) * 100 : (p.accuracy ?? 0),
		dmgN: p.damageTaken ? (p.damageDealt ?? 0) / p.damageTaken : (p.damageRatio ?? 0),
		spreeN: p.bestSpree ?? p.spree ?? 0,
		hs: p.headshots ?? 0,
		melee: p.meleeKills ?? 0,
		nade: p.grenadeKills ?? 0,
		camoP: p.camoPickups ?? 0,
		os: p.osPickups ?? 0,
		btrl: p.betrayals ?? 0,
		suic: p.suicides ?? 0
	});
	const finishRows = (rows) => {
		rows.forEach((r) => {
			r.kd = r.kdN.toFixed(2);
			r.acc = r.accN.toFixed(1);
			r.dmg = r.dmgN.toFixed(2);
			r.spree = '×' + r.spreeN;
			r.colors = {};
		});
		const numKey = { kd: 'kdN', acc: 'accN', dmg: 'dmgN', spree: 'spreeN' };
		for (const [key] of cols) {
			if (key === 'btrl' || key === 'suic') {
				rows.forEach((r) => {
					r.colors[key] = r[key] > 0 ? '#ff8f85' : '#cdd9ea';
				});
				continue;
			}
			const nk = numKey[key] ?? key;
			const vals = rows.map((r) => r[nk]);
			const best = bestDir[key] === 'lo' ? Math.min(...vals) : Math.max(...vals);
			rows.forEach((r, i) => {
				r.colors[key] = r[nk] === best ? ORANGE : cols.find((c) => c[0] === key)[2];
			});
		}
		return rows;
	};

	const teams = $derived(match.teams ?? null);
	const allRows = $derived(finishRows([...players].sort((a, b) => b.score - a.score).map(toRow)));
	const winnerTeam = $derived(teams ? [...teams].sort((a, b) => b.score - a.score) : null);
	const winnerName = $derived(
		teams
			? (winnerTeam[0].name ?? winnerTeam[0].id.toUpperCase() + ' TEAM')
			: (allRows[0]?.name ?? '')
	);
	const winnerColor = $derived(teams ? TEAM_HEX[winnerTeam[0].id] : '#3d62e0');
	const teamRows = (id) => allRows.filter((r) => r.team === id);
	const agg = (rows) => {
		const s = (k) => rows.reduce((t, r) => t + r[k], 0);
		return {
			score: s('score'),
			k: s('k'),
			d: s('d'),
			a: s('a'),
			kd: (s('k') / Math.max(s('d'), 1)).toFixed(2),
			acc: (rows.reduce((t, r) => t + r.accN, 0) / Math.max(rows.length, 1)).toFixed(1),
			dmg: (rows.reduce((t, r) => t + r.dmgN, 0) / Math.max(rows.length, 1)).toFixed(2),
			spree: '×' + Math.max(0, ...rows.map((r) => r.spreeN)),
			hs: s('hs'),
			melee: s('melee'),
			nade: s('nade'),
			camoP: s('camoP'),
			os: s('os'),
			btrl: s('btrl'),
			suic: s('suic')
		};
	};
	const totals = $derived({
		kills: players.reduce((t, p) => t + (p.kills ?? 0), 0),
		shots: players.reduce((t, p) => t + (p.shots ?? 0), 0).toLocaleString('en-US'),
		nades: players.reduce((t, p) => t + (p.grenadesThrown ?? 0), 0),
		dmg: players.reduce((t, p) => t + (p.damageDealt ?? 0), 0).toLocaleString('en-US')
	});
</script>

<svelte:head>
	<title>Norcal Halo — post-game</title>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Ultra&family=Inter:wght@400;500;600;700&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

{#snippet statCells(r, mono)}
	{#each cols as [key, w]}
		<span
			class="cell"
			class:mono
			style="width:{w}px; color:{r.colors?.[key] ?? 'inherit'}"
			class:big={key === 'score' && !mono}>{r[key]}</span
		>
	{/each}
{/snippet}

<div class="report">
	<div class="banner">
		<div class="win">
			<span
				class="vic"
				style="color:{teams ? (winnerTeam[0].id === 'red' ? '#ff8f85' : '#7d9cff') : ORANGE}"
				>VICTORY</span
			>
			<span class="wname">{winnerName} <span style="color:{winnerColor}">✦</span></span>
		</div>
		<div class="meta">
			<span class="mode"
				>{match.gametype ?? ''}{match.killLimit ? ' — KILL LIMIT ' + match.killLimit : ''}</span
			>
			<span class="map">{match.map ?? ''}</span>
			<span class="mtime">MATCH TIME {match.matchTime ?? match.clock ?? ''}</span>
		</div>
	</div>
	<div class="colhead">
		<span style="width:16px"></span><span style="width:34px"></span>
		<span class="ch player">PLAYER</span>
		{#each cols as [key, w]}<span class="ch" style="width:{w}px">{headers[key]}</span>{/each}
	</div>
	{#if teams}
		{#each winnerTeam as team, ti (team.id)}
			{@const rows = teamRows(team.id)}
			<div class="pad">
				<div
					class="row teamhead"
					style="background:{TEAM_HEX[team.id]}47; border-color:{TEAM_HEX[
						team.id
					]}80; animation-delay:{ti * (rows.length + 1) * 0.055 + 0.1}s"
				>
					<span class="tname">{team.name ?? team.id.toUpperCase() + ' TEAM'}</span>
					{@render statCells({ ...agg(rows), colors: {} }, true)}
				</div>
			</div>
			{#each rows as r, i (r.name)}
				<div class="pad">
					<div
						class="row"
						style="background:{tint(TEAM_HEX[team.id]).bg}; border-color:{tint(TEAM_HEX[team.id])
							.edge}; animation-delay:{(ti * (rows.length + 1) + i + 1) * 0.055 + 0.1}s"
					>
						<span class="place">{i + 1}</span>
						<span class="avatar"
							>{#if r.avatar}<img src={r.avatar} alt={r.name} />{:else}✦{/if}</span
						>
						<span class="pname">{r.name}</span>
						{@render statCells(r, false)}
					</div>
				</div>
			{/each}
		{/each}
	{:else}
		{#each allRows as r, i (r.name)}
			<div class="pad">
				<div
					class="row"
					style="background:{tint(r.armor).bg}; border-color:{tint(r.armor)
						.edge}; animation-delay:{i * 0.055 + 0.15}s"
				>
					<span class="place">{i + 1}</span>
					<span class="avatar"
						>{#if r.avatar}<img src={r.avatar} alt={r.name} />{:else}✦{/if}</span
					>
					<span class="pname">{r.name}</span>
					{@render statCells(r, false)}
				</div>
			</div>
		{/each}
	{/if}
	<div class="foot">
		<span>{totals.kills} KILLS</span><i></i>
		<span>{totals.shots} SHOTS FIRED</span><i></i>
		<span>{totals.nades} GRENADES THROWN</span><i></i>
		<span>{totals.dmg} DAMAGE DEALT</span>
	</div>
</div>

<style>
	/* OBS browser source, transparent; size ~900 × (150 + 46·rows). */
	:global(html, body) {
		margin: 0;
		background: transparent;
		overflow: hidden;
	}
	/* Kill the app's xbox-theme hex-mesh (body::before) so only the overlay
	   composites over the OBS feed. Unlayered + !important beats the themed
	   @layer base rule in routes/layout.css. */
	:global(body::before) {
		display: none !important;
	}
	.report {
		width: 900px;
		border-radius: 20px;
		overflow: hidden;
		padding-bottom: 6px;
		border: 1px solid rgba(159, 180, 208, 0.25);
		box-shadow:
			0 0 26px rgba(61, 98, 224, 0.28),
			inset 0 1px 0 rgba(255, 255, 255, 0.14);
		background: rgba(11, 14, 26, 0.95);
		font-family: Inter, sans-serif;
	}
	.banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 18px 22px 15px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		animation: row-in 0.45s cubic-bezier(0.22, 1, 0.36, 1) both;
	}
	.win {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.vic {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.34em;
	}
	.wname {
		font-family: Ultra, serif;
		font-size: 30px;
		line-height: 1;
		color: #e8ecf5;
		letter-spacing: 0.03em;
	}
	.meta {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 3px;
	}
	.mode {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: #9fb4d0;
	}
	.map {
		font-family: Ultra, serif;
		font-size: 17px;
		line-height: 1;
		color: #e8ecf5;
		letter-spacing: 0.04em;
	}
	.mtime {
		font-family: 'Lucida Console', monospace;
		font-size: 12px;
		color: #9fb4d0;
	}
	.colhead {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 9px 18px 5px 16px;
	}
	.ch {
		text-align: right;
		font-size: 8px;
		font-weight: 700;
		letter-spacing: 0.18em;
		color: #7c8496;
		flex: none;
	}
	.ch.player {
		flex: 1;
		min-width: 0;
		text-align: left;
		letter-spacing: 0.22em;
	}
	.pad {
		padding: 3px 8px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 6px 10px 6px 8px;
		border-radius: 9px;
		border: 1px solid;
		animation: row-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
	}
	.place {
		width: 16px;
		flex: none;
		text-align: center;
		font-size: 13px;
		font-weight: 700;
		color: #9fb4d0;
		font-variant-numeric: tabular-nums;
	}
	.avatar {
		width: 34px;
		height: 34px;
		flex: none;
		border-radius: 50%;
		overflow: hidden;
		background: repeating-linear-gradient(45deg, #10152a 0 5px, #141a30 5px 10px);
		border: 1px solid rgba(159, 180, 208, 0.4);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 12px;
		color: #9fb4d0;
	}
	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.pname,
	.tname {
		flex: 1;
		min-width: 0;
		font-family: Ultra, serif;
		font-size: 14px;
		color: #e8ecf5;
		letter-spacing: 0.03em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.tname {
		font-size: 15px;
	}
	.cell {
		flex: none;
		text-align: right;
		font-size: 13px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.cell.big {
		font-size: 15px;
		font-weight: 700;
	}
	.row.teamhead .cell {
		font-family: 'Lucida Console', monospace;
		font-size: 10px;
		font-weight: 400;
		color: #e8ecf5;
	}
	.row.teamhead .cell:first-of-type {
		font-family: Inter, sans-serif;
		font-size: 15px;
		font-weight: 700;
	}
	.foot {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 26px;
		padding: 12px 18px 8px;
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		margin-top: 5px;
		font-family: 'Lucida Console', monospace;
		font-size: 11px;
		color: #9fb4d0;
	}
	.foot i {
		width: 1px;
		height: 12px;
		background: rgba(255, 255, 255, 0.14);
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
</style>
