<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// One leaderboard row: rank, emblem avatar, name, K/D/A, spree, score, and
	// the shield/health bar pair. Ported from the obs-handoff pack's
	// leaderboard.html rowsBlock(). Row pitch is 52px (46px card + 3px padding
	// top and bottom) — the board positions slots on that pitch, so any change
	// here must move ROW_PITCH in the board too.
	import starUrl from '$lib/assets/star.png';
	import { pad2 } from './themes.js';

	let {
		player = {},
		/** Rank number to show, or 0/undefined to hide it (team mode ranks within
		 * the team chip instead, so the column would be noise). */
		rank = 0,
		/** Leader of this block — score takes Selection Orange. */
		leader = false
	} = $props();

	const dead = $derived(player.alive === false);
	const camo = $derived((player.camo ?? 0) > 0);

	// Armor colour → row tint + edge, matching the in-game armour colour.
	const armor = $derived(player.armor || '#9fb4d0');
	const kda = $derived(`${pad2(player.kills)}/${pad2(player.deaths)}/${pad2(player.assists)}`);
	const spree = $derived(player.spree > 0 ? `×${player.spree}` : '—');

	const clamp = (v) => `${Math.max(0, Math.min(1, v ?? 0)) * 100}%`;
	// Shield reads above 1.0 with an overshield — the base bar saturates and the
	// white pip overlays the excess.
	const shieldPct = $derived(clamp(Math.min(1, player.shield ?? 0)));
	const overPct = $derived(clamp(Math.max(0, (player.shield ?? 0) - 1)));
	const healthPct = $derived(clamp(player.health));
</script>

<div class="row" class:dead style="background:{armor}26; border-color:{armor}4D">
	<div class="inner">
		{#if rank}<span class="rank">{rank}</span>{/if}

		<div class="avatar">
			<img src={starUrl} alt="" />
		</div>

		<div class="body">
			<div class="top">
				<span class="name">{player.name ?? '—'}</span>
				<span class="kda">{kda}</span>
				<span class="spree" class:hot={player.spree >= 3}>{spree}</span>
				<span class="score" class:leader>{player.score ?? 0}</span>
			</div>

			<div class="bars">
				<div class="bar is-shield">
					<i class="fill-shield" style="width:{shieldPct}"></i>
					<i class="fill-os" style="width:{overPct}"></i>
				</div>
				<div class="bar is-health">
					<i class="fill-health" style="width:{healthPct}"></i>
				</div>
			</div>
		</div>
	</div>

	{#if camo}<div class="camo"></div>{/if}
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
		padding: 2px 10px;
	}

	.rank {
		width: 16px;
		flex: none;
		text-align: center;
		font-size: 13px;
		font-weight: 700;
		color: var(--nh-steel);
		font-variant-numeric: tabular-nums;
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
	.avatar img {
		width: 46px;
		flex: none;
		display: block;
		opacity: 0.85;
	}

	.body {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.top {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.name {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 12px;
		color: var(--nh-text);
		letter-spacing: 0.04em;
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	/* Zero-padded so the column never shifts width as scores climb. */
	.kda {
		flex: none;
		width: 58px;
		text-align: right;
		font-family: 'Lucida Console', monospace;
		font-size: 10px;
		color: var(--nh-steel);
		font-variant-numeric: tabular-nums;
	}
	.spree {
		flex: none;
		width: 26px;
		text-align: right;
		font-size: 11px;
		font-weight: 700;
		color: var(--nh-steel);
		font-variant-numeric: tabular-nums;
	}
	.spree.hot {
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
	.bar.is-shield {
		height: 4px;
	}
	.bar.is-health {
		height: 3px;
	}
	.bar > i {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		display: block;
	}
	.fill-shield {
		background: #6ec8e8;
	}
	.fill-os {
		background: var(--nh-text);
		box-shadow: 0 0 6px rgba(232, 236, 245, 0.9);
	}
	.fill-health {
		background: var(--nh-red);
	}

	/* Camo cloaks the row and solidifies back left-to-right as it drains. The
	   scraper only exposes a has_camo bool (no timer), so this runs on CE's
	   nominal 30s cloak rather than the real remaining time. */
	.camo {
		position: absolute;
		inset: 0;
		background: rgba(11, 14, 26, 0.78);
		animation: camo-uncloak 30s linear infinite;
		pointer-events: none;
	}
	@keyframes camo-uncloak {
		0% {
			clip-path: inset(0 0 0 0 round 6px);
		}
		96%,
		100% {
			clip-path: inset(0 0 0 100% round 6px);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.camo {
			animation: none;
		}
	}
</style>
