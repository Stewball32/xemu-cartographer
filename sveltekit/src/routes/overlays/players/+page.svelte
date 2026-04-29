<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import type { SnapshotPlayer, TickPlayer, WeaponInfo } from '$lib/types/scraper';

	type Joined = {
		snap: SnapshotPlayer;
		tick: TickPlayer | null;
	};

	const HEALTH_MAX = 75;
	const SHIELD_MAX = 75;

	const TEAM_COLORS = ['bg-red-500', 'bg-blue-500', 'bg-green-500', 'bg-yellow-500'];

	function teamClass(team: number): string {
		return TEAM_COLORS[team % TEAM_COLORS.length];
	}

	function shortTag(tag: string): string {
		if (!tag) return '';
		const trimmed = tag.endsWith('.weapon') ? tag.slice(0, -'.weapon'.length) : tag;
		const segs = trimmed.split('\\');
		return segs[segs.length - 1] || trimmed;
	}

	function ammoLabel(w: WeaponInfo): string {
		if (w.is_energy && typeof w.charge === 'number') {
			return `${Math.round(w.charge * 100)}%`;
		}
		const mag = w.ammo_mag ?? 0;
		const pack = w.ammo_pack ?? 0;
		return `${mag} / ${pack}`;
	}

	function selectedWeapon(t: TickPlayer | null): WeaponInfo | null {
		if (!t || !t.weapons || t.weapons.length === 0) return null;
		const slot = t.selected_weapon_slot;
		const found = t.weapons.find((w) => w.slot === slot);
		return found ?? t.weapons[0];
	}

	let joined = $derived.by<Joined[]>(() => {
		const snap = scraperWS.snapshot;
		const tick = scraperWS.tick;
		if (!snap || !snap.players) return [];
		const tickByIdx = new SvelteMap<number, TickPlayer>();
		if (tick && tick.players) {
			for (const tp of tick.players) tickByIdx.set(tp.index, tp);
		}
		return snap.players
			.filter((p) => p.is_local === true)
			.sort((a, b) => (a.local_index ?? 0) - (b.local_index ?? 0))
			.map((p) => ({ snap: p, tick: tickByIdx.get(p.index) ?? null }));
	});

	onMount(() => {
		if (auth.token) {
			scraperWS.connect(auth.token);
		}
	});

	onDestroy(() => {
		scraperWS.disconnect();
	});
</script>

<svelte:head>
	<style>
		body {
			background: transparent !important;
		}
	</style>
</svelte:head>

<div class="overlay-root">
	{#if !scraperWS.connected}
		<div class="status-card">Connecting…</div>
	{:else if joined.length === 0}
		<div class="status-card">Waiting for match…</div>
	{:else}
		<div class="scoreboard">
			{#each joined as p (p.snap.index)}
				{@const w = selectedWeapon(p.tick)}
				{@const health = p.tick?.health ?? 0}
				{@const shields = p.tick?.shields ?? 0}
				{@const alive = p.tick?.alive ?? true}
				<div class="player-card" class:dead={!alive}>
					<div class="team-stripe {teamClass(p.snap.team)}"></div>
					<div class="player-body">
						<div class="player-header">
							<span class="player-name">{p.snap.name || `P${p.snap.index}`}</span>
							<span class="kda">
								{p.snap.kills} / {p.snap.deaths} / {p.snap.assists}
							</span>
						</div>
						<div class="bars">
							<div class="bar">
								<div class="bar-label">Shields</div>
								<div class="bar-track">
									<div
										class="bar-fill bar-shield"
										style="width: {Math.max(0, Math.min(1, shields / SHIELD_MAX)) * 100}%"
									></div>
								</div>
							</div>
							<div class="bar">
								<div class="bar-label">Health</div>
								<div class="bar-track">
									<div
										class="bar-fill bar-health"
										style="width: {Math.max(0, Math.min(1, health / HEALTH_MAX)) * 100}%"
									></div>
								</div>
							</div>
						</div>
						<div class="player-footer">
							<div class="weapon">
								{#if w}
									<span class="weapon-tag">{shortTag(w.tag)}</span>
									<span class="weapon-ammo">{ammoLabel(w)}</span>
								{:else}
									<span class="weapon-tag empty">unarmed</span>
								{/if}
							</div>
							<div class="powerups">
								{#if p.tick?.has_camo}
									<span class="powerup camo" title="Active Camouflage">C</span>
								{/if}
								{#if p.tick?.has_overshield}
									<span class="powerup overshield" title="Overshield">O</span>
								{/if}
								{#if p.tick}
									<span class="grenades" title="Grenades">
										{p.tick.frags}f / {p.tick.plasmas}p
									</span>
								{/if}
							</div>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.overlay-root {
		position: fixed;
		inset: 0;
		display: flex;
		align-items: flex-start;
		justify-content: flex-start;
		padding: 1.5rem;
		font-family:
			system-ui,
			-apple-system,
			sans-serif;
		color: #fff;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		pointer-events: none;
	}

	.status-card {
		background: rgba(0, 0, 0, 0.55);
		padding: 0.75rem 1.25rem;
		border-radius: 0.5rem;
		font-size: 1rem;
	}

	.scoreboard {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		min-width: 22rem;
	}

	.player-card {
		display: flex;
		background: rgba(0, 0, 0, 0.55);
		border-radius: 0.5rem;
		overflow: hidden;
		transition: opacity 0.2s;
	}

	.player-card.dead {
		opacity: 0.55;
	}

	.team-stripe {
		width: 6px;
		flex-shrink: 0;
	}

	.player-body {
		flex: 1;
		padding: 0.5rem 0.75rem;
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}

	.player-header {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
	}

	.player-name {
		font-weight: 600;
		font-size: 1rem;
	}

	.kda {
		font-variant-numeric: tabular-nums;
		font-size: 0.875rem;
	}

	.bars {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	.bar {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.bar-label {
		font-size: 0.7rem;
		width: 3.5rem;
		opacity: 0.7;
	}

	.bar-track {
		flex: 1;
		height: 0.5rem;
		background: rgba(255, 255, 255, 0.15);
		border-radius: 999px;
		overflow: hidden;
	}

	.bar-fill {
		height: 100%;
		transition: width 0.1s linear;
	}

	.bar-shield {
		background: linear-gradient(90deg, #4cc9f0, #4361ee);
	}

	.bar-health {
		background: linear-gradient(90deg, #f72585, #b5179e);
	}

	.player-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 0.85rem;
	}

	.weapon {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.weapon-tag {
		text-transform: capitalize;
	}

	.weapon-tag.empty {
		opacity: 0.5;
	}

	.weapon-ammo {
		font-variant-numeric: tabular-nums;
		opacity: 0.85;
	}

	.powerups {
		display: flex;
		gap: 0.4rem;
		align-items: center;
		font-size: 0.75rem;
	}

	.powerup {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.25rem;
		height: 1.25rem;
		border-radius: 999px;
		font-weight: 700;
		font-size: 0.7rem;
	}

	.powerup.camo {
		background: rgba(120, 200, 255, 0.4);
	}

	.powerup.overshield {
		background: rgba(255, 220, 100, 0.4);
	}

	.grenades {
		opacity: 0.85;
	}
</style>
