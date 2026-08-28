<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// One leaderboard row — redesign (CL-01/08/10/12/18). Leads with the shared
	// NamePlate at 0.63× (240×40, motto shown where set), then unlabeled
	// K/D/A columns (Inter, no leading zeros, best-of-stat orange) and score.
	// PLAYER-FACING SURFACE: the board stays in view of players at the LAN,
	// so it carries NO live tactical state — shield/health bars, spree column,
	// camo overlay AND the plate's overshield ring are all off here (os is
	// never passed). Dead rows gray out.
	//
	//   ord      ordinal rank label — '' hides the column. The FFA board no
	//            longer passes it (ordinals live on the page's fixed slot
	//            rail so labels never travel with a gliding row).
	//   leader   lobby-best score → Selection Orange.
	//   best     {k,d,a} lobby bests for the column highlights.
	//   flat     team-container mode: no armor tint, row sits on the panel.
	import NamePlate from './NamePlate.svelte';

	let { player = {}, ord = '', leader = false, best = null, flat = false } = $props();

	const dead = $derived(player.alive === false);
	const armor = $derived(player.armor || '#9fb4d0');
	const cells = $derived([
		[player.kills ?? 0, best && (player.kills ?? 0) === best.k],
		[player.deaths ?? 0, best && (player.deaths ?? 0) === best.d],
		[player.assists ?? 0, best && (player.assists ?? 0) === best.a]
	]);
</script>

<div
	class="row"
	class:dead
	style={flat
		? 'background:transparent; border-color:transparent'
		: `background:${armor}26; border-color:${armor}4D`}
>
	<div class="inner">
		{#if ord}<span class="rank">{ord}</span>{/if}
		<NamePlate {player} h={40} bg={player.plateBg} />
		<div class="grow"></div>
		<div class="kda">
			{#each cells as [v, hot], i (i)}
				<i class:hot>{v}</i>
			{/each}
		</div>
		<span class="score" class:leader>{player.score ?? 0}</span>
	</div>
</div>

<style>
	.row {
		position: relative;
		border-radius: 6px;
		overflow: hidden;
		border: 1px solid;
		font-family: Inter, system-ui, sans-serif;
	}
	.row.dead {
		opacity: 0.45;
		filter: grayscale(1);
	}
	.inner {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 2px 10px 2px 8px;
	}
	.rank {
		width: 26px;
		flex: none;
		text-align: center;
		font-size: 9.5px;
		font-weight: 700;
		letter-spacing: 0.06em;
		color: var(--nh-steel);
		font-variant-numeric: tabular-nums;
	}
	.grow {
		flex: 1;
		min-width: 0;
	}
	/* Unlabeled K/D/A — Inter tabular, no leading zeros (CL-08 revised · 12);
	   best in category takes Selection Orange (CL-01). */
	.kda {
		flex: none;
		display: flex;
		gap: 7px;
	}
	.kda i {
		font-style: normal;
		font-size: 12px;
		font-weight: 700;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
		min-width: 16px;
		text-align: center;
	}
	.kda i.hot {
		color: var(--nh-orange);
	}
	.score {
		flex: none;
		width: 24px;
		text-align: right;
		font-size: 16px;
		font-weight: 700;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}
	.score.leader {
		color: var(--nh-orange);
	}
</style>
