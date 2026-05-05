<script lang="ts">
	import { ChevronDownIcon } from '@lucide/svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import type { TickPlayer, GamePlayer } from '$lib/types/scraper';
	import KvCard from './KvCard.svelte';
	import JsonTree from '../JsonTree.svelte';

	let {
		tickPlayer,
		gamePlayer
	}: {
		tickPlayer: TickPlayer;
		gamePlayer: GamePlayer | null;
	} = $props();

	const identity = $derived({
		index: tickPlayer.index,
		name: gamePlayer?.name ?? '—',
		team: gamePlayer?.team ?? null,
		is_local: gamePlayer?.is_local ?? null,
		local_index: gamePlayer?.local_index ?? null,
		alive: tickPlayer.alive,
		respawn_in_ticks: tickPlayer.respawn_in_ticks,
		kills: gamePlayer?.kills ?? 0,
		deaths: gamePlayer?.deaths ?? 0,
		assists: gamePlayer?.assists ?? 0,
		ctf_score: gamePlayer?.ctf_score ?? 0,
		team_kills: gamePlayer?.team_kills ?? 0,
		suicides: gamePlayer?.suicides ?? 0,
		kill_streak: gamePlayer?.kill_streak ?? 0,
		multikill: gamePlayer?.multikill ?? 0,
		shots_fired: gamePlayer?.shots_fired ?? 0,
		shots_hit: gamePlayer?.shots_hit ?? 0
	});

	const positionVelocity = $derived({
		x: tickPlayer.x,
		y: tickPlayer.y,
		z: tickPlayer.z,
		vx: tickPlayer.vx,
		vy: tickPlayer.vy,
		vz: tickPlayer.vz,
		aim_x: tickPlayer.aim_x,
		aim_y: tickPlayer.aim_y,
		aim_z: tickPlayer.aim_z,
		zoom_level: tickPlayer.zoom_level,
		crouchscale: tickPlayer.crouchscale,
		is_crouching: tickPlayer.is_crouching,
		is_jumping: tickPlayer.is_jumping
	});

	const combat = $derived({
		health: tickPlayer.health,
		shields: tickPlayer.shields,
		has_camo: tickPlayer.has_camo,
		has_overshield: tickPlayer.has_overshield,
		frags: tickPlayer.frags,
		plasmas: tickPlayer.plasmas,
		selected_weapon_slot: tickPlayer.selected_weapon_slot,
		is_firing: tickPlayer.is_firing,
		is_shooting: tickPlayer.is_shooting,
		is_flashlight_on: tickPlayer.is_flashlight_on,
		is_throwing_grenade: tickPlayer.is_throwing_grenade,
		is_meleeing: tickPlayer.is_meleeing,
		is_pressing_action: tickPlayer.is_pressing_action,
		is_holding_action: tickPlayer.is_holding_action
	});

	let openSections = $state(['identity', 'combat']);
</script>

<div class="card preset-tonal p-3">
	<div class="mb-3 flex items-center justify-between">
		<h3 class="font-semibold">{gamePlayer?.name ?? `Player ${tickPlayer.index}`}</h3>
		<span class="text-surface-700-200 font-mono text-xs">slot #{tickPlayer.index}</span>
	</div>

	<Accordion value={openSections} onValueChange={(d) => (openSections = d.value)} multiple>
		<Accordion.Item value="identity">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold">Identity & Score</span>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={identity} />
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="position">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold">Position & Velocity</span>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={positionVelocity} />
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="combat">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold">Combat</span>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<KvCard value={combat} />
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="weapons">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold"
					>Weapons <span class="text-surface-700-200 font-normal">({tickPlayer.weapons?.length ?? 0})</span></span
				>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				{#if tickPlayer.weapons && tickPlayer.weapons.length > 0}
					<JsonTree value={tickPlayer.weapons} depth={1} />
				{:else}
					<div class="text-surface-500-400 text-sm">no weapons</div>
				{/if}
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="bones">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold"
					>Bones <span class="text-surface-700-200 font-normal">({tickPlayer.bones?.length ?? 0})</span></span
				>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				{#if tickPlayer.bones && tickPlayer.bones.length > 0}
					<JsonTree value={tickPlayer.bones} depth={1} />
				{:else}
					<div class="text-surface-500-400 text-sm">no bone data (player likely dead)</div>
				{/if}
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="extended">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold">Extended (Dynamic Biped)</span>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				{#if tickPlayer.extended}
					<KvCard value={tickPlayer.extended as unknown as Record<string, unknown>} />
				{:else}
					<div class="text-surface-500-400 text-sm">no extended data (player likely dead)</div>
				{/if}
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="update_queue">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold">Update Queue</span>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				{#if tickPlayer.update_queue}
					<KvCard value={tickPlayer.update_queue as unknown as Record<string, unknown>} />
				{:else}
					<div class="text-surface-500-400 text-sm">no update queue data</div>
				{/if}
			</Accordion.ItemContent>
		</Accordion.Item>

		<hr class="hr" />

		<Accordion.Item value="biped_tag">
			<Accordion.ItemTrigger class="flex w-full items-center justify-between gap-2 py-2 text-left">
				<span class="text-sm font-semibold">Biped Tag (Static)</span>
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				{#if tickPlayer.biped_tag}
					<KvCard value={tickPlayer.biped_tag as unknown as Record<string, unknown>} />
				{:else}
					<div class="text-surface-500-400 text-sm">no biped tag (player likely dead)</div>
				{/if}
			</Accordion.ItemContent>
		</Accordion.Item>
	</Accordion>
</div>
