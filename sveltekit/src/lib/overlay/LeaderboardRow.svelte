<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
	import { ORANGE, TEAM_HEX, tint, pad2 } from './themes.js';

	// player: {name, armor, team, score, kills, deaths, assists, spree, shield (0-2), health (0-1), alive, camo}
	let {
		player = {},
		topScore = 0,
		topSpree = 0,
		place = 0,
		avatar = '',
		forceTint = null
	} = $props();

	const armor = $derived(forceTint ?? TEAM_HEX[player.team] ?? player.armor ?? '#8a93a8');
	const t = $derived(tint(armor));
	const dead = $derived(player.alive === false);
	const camo = $derived((player.camo ?? 0) > 0);
	const kda = $derived(pad2(player.kills) + '/' + pad2(player.deaths) + '/' + pad2(player.assists));
	const spree = $derived(player.spree ?? 0);
	const spreeColor = $derived(spree > 0 && spree === topSpree ? ORANGE : '#9fb4d0');
	const scoreColor = $derived(player.score === topScore ? ORANGE : '#e8ecf5');
	const shieldPct = $derived(Math.min(player.shield ?? 0, 1) * 100 + '%');
	const osPct = $derived(Math.max((player.shield ?? 0) - 1, 0) * 100 + '%');
	const healthPct = $derived((player.health ?? 0) * 100 + '%');
</script>

<div class="rowpad" class:dead style="opacity: {camo ? 0.32 : dead ? 0.75 : 1}">
	<div class="chip" style="background:{t.bg}; border-color:{t.edge}">
		<div class="line">
			{#if place}<span class="place">{place}</span>{/if}
			<div class="avatar">
				{#if avatar}<img src={avatar} alt={player.name} />{:else}<span class="star">✦</span>{/if}
			</div>
			<div class="body">
				<div class="stats">
					<span class="name">{player.name ?? '—'}</span>
					<span class="kda">{kda}</span>
					<span class="spree" style="color:{spreeColor}">×{spree}</span>
					<span class="score" style="color:{scoreColor}">{player.score ?? 0}</span>
				</div>
				<div class="bars">
					<div class="bar shield">
						<div class="fill" style="width:{shieldPct}; background:#6ec8e8"></div>
						<div class="fill os" style="width:{osPct}"></div>
					</div>
					<div class="bar health">
						<div class="fill" style="width:{healthPct}; background:#e05252"></div>
					</div>
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	.rowpad {
		padding: 3px 8px;
		font-family: Inter, sans-serif;
	}
	.rowpad.dead {
		filter: grayscale(1) brightness(0.85);
	}
	.chip {
		border-radius: 9px;
		border: 1px solid;
		overflow: hidden;
	}
	.line {
		display: flex;
		align-items: center;
		gap: 9px;
		padding: 6px 10px 6px 8px;
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
	}
	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.star {
		font-size: 12px;
		color: #9fb4d0;
	}
	.body {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.stats {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.name {
		font-family: Ultra, serif;
		font-size: 14px;
		color: #e8ecf5;
		letter-spacing: 0.03em;
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.kda {
		flex: none;
		width: 58px;
		text-align: right;
		font-family: 'Lucida Console', monospace;
		font-size: 10px;
		color: #9fb4d0;
		font-variant-numeric: tabular-nums;
	}
	.spree {
		flex: none;
		width: 26px;
		text-align: right;
		font-size: 11px;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}
	.score {
		flex: none;
		width: 24px;
		text-align: right;
		font-size: 16px;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}
	.bars {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.bar {
		position: relative;
		border-radius: 2px;
		background: rgba(11, 14, 26, 0.6);
		overflow: hidden;
	}
	.bar.shield {
		height: 4px;
	}
	.bar.health {
		height: 3px;
	}
	.fill {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
	}
	.fill.os {
		background: #e8ecf5;
		box-shadow: 0 0 6px rgba(232, 236, 245, 0.9);
	}
</style>
