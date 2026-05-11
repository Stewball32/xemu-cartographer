<script lang="ts">
	// Detail-panel for one TickPlayer — surfaced inside a Dialog when the
	// operator selects a row in PlayersRuntimeSection. Renders position,
	// velocity, aim, life-status, combat state, weapon-loadout (via a
	// ColGroupedTable), and any extended/bones/queue/biped sub-structs.

	import type { GamePlayer, TickPlayer } from '$lib/types/scraper';
	import KvGrid, { type KvField } from '../shared/KvGrid.svelte';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { ColGroup } from '../shared/col-grouped-table';
	import { teamAccent, teamLabel } from '../shared/util';
	import { recordToFields } from './tick-vm';

	let {
		tickPlayer,
		gamePlayer,
		teamGame = false,
		annotationPrefix = 'tick.player'
	}: {
		tickPlayer: TickPlayer;
		gamePlayer: GamePlayer | null;
		teamGame?: boolean;
		annotationPrefix?: string;
	} = $props();

	const idx = $derived(tickPlayer.index);
	const indexedPrefix = $derived(`${annotationPrefix}.${idx}`);
	const accent = $derived(teamAccent(gamePlayer?.team ?? 0));
	const displayName = $derived(gamePlayer?.name ?? `Player ${idx}`);

	// Life / respawn / camo / overshield — the cluster the operator wants
	// first when triaging "is this player alive right now".
	const lifeFields = $derived<KvField[]>([
		{ key: 'alive', value: tickPlayer.alive, annotationKey: `${indexedPrefix}.alive` },
		{
			key: 'respawn_in_ticks',
			value: tickPlayer.respawn_in_ticks,
			annotationKey: `${indexedPrefix}.respawn_in_ticks`
		},
		{ key: 'health', value: tickPlayer.health, annotationKey: `${indexedPrefix}.health` },
		{ key: 'shields', value: tickPlayer.shields, annotationKey: `${indexedPrefix}.shields` },
		{ key: 'has_camo', value: tickPlayer.has_camo, annotationKey: `${indexedPrefix}.has_camo` },
		{
			key: 'camo_timer',
			value: tickPlayer.camo_timer,
			annotationKey: `${indexedPrefix}.camo_timer`
		},
		{
			key: 'has_overshield',
			value: tickPlayer.has_overshield,
			annotationKey: `${indexedPrefix}.has_overshield`
		}
	]);

	const positionFields = $derived<KvField[]>([
		{ key: 'x', value: tickPlayer.x, annotationKey: `${indexedPrefix}.x` },
		{ key: 'y', value: tickPlayer.y, annotationKey: `${indexedPrefix}.y` },
		{ key: 'z', value: tickPlayer.z, annotationKey: `${indexedPrefix}.z` }
	]);

	const velocityFields = $derived<KvField[]>([
		{ key: 'vx', value: tickPlayer.vx, annotationKey: `${indexedPrefix}.vx` },
		{ key: 'vy', value: tickPlayer.vy, annotationKey: `${indexedPrefix}.vy` },
		{ key: 'vz', value: tickPlayer.vz, annotationKey: `${indexedPrefix}.vz` }
	]);

	const aimFields = $derived<KvField[]>([
		{ key: 'aim_x', value: tickPlayer.aim_x, annotationKey: `${indexedPrefix}.aim_x` },
		{ key: 'aim_y', value: tickPlayer.aim_y, annotationKey: `${indexedPrefix}.aim_y` },
		{ key: 'aim_z', value: tickPlayer.aim_z, annotationKey: `${indexedPrefix}.aim_z` },
		{
			key: 'zoom_level',
			value: tickPlayer.zoom_level,
			annotationKey: `${indexedPrefix}.zoom_level`
		}
	]);

	const combatFields = $derived<KvField[]>([
		{ key: 'frags', value: tickPlayer.frags, annotationKey: `${indexedPrefix}.frags` },
		{ key: 'plasmas', value: tickPlayer.plasmas, annotationKey: `${indexedPrefix}.plasmas` },
		{
			key: 'selected_weapon_slot',
			value: tickPlayer.selected_weapon_slot,
			annotationKey: `${indexedPrefix}.selected_weapon_slot`
		},
		{
			key: 'is_crouching',
			value: tickPlayer.is_crouching,
			annotationKey: `${indexedPrefix}.is_crouching`
		},
		{
			key: 'is_jumping',
			value: tickPlayer.is_jumping,
			annotationKey: `${indexedPrefix}.is_jumping`
		},
		{
			key: 'is_firing',
			value: tickPlayer.is_firing,
			annotationKey: `${indexedPrefix}.is_firing`
		},
		{
			key: 'is_shooting',
			value: tickPlayer.is_shooting,
			annotationKey: `${indexedPrefix}.is_shooting`
		},
		{
			key: 'is_flashlight_on',
			value: tickPlayer.is_flashlight_on,
			annotationKey: `${indexedPrefix}.is_flashlight_on`
		},
		{
			key: 'is_throwing_grenade',
			value: tickPlayer.is_throwing_grenade,
			annotationKey: `${indexedPrefix}.is_throwing_grenade`
		},
		{
			key: 'is_meleeing',
			value: tickPlayer.is_meleeing,
			annotationKey: `${indexedPrefix}.is_meleeing`
		},
		{
			key: 'is_pressing_action',
			value: tickPlayer.is_pressing_action,
			annotationKey: `${indexedPrefix}.is_pressing_action`
		},
		{
			key: 'is_holding_action',
			value: tickPlayer.is_holding_action,
			annotationKey: `${indexedPrefix}.is_holding_action`
		},
		{
			key: 'crouchscale',
			value: tickPlayer.crouchscale,
			annotationKey: `${indexedPrefix}.crouchscale`
		}
	]);

	const weaponsGroups: ColGroup[] = [
		{
			label: 'slot',
			columns: [
				{ key: 'slot', label: 'slot' },
				{ key: 'tag', label: 'tag' }
			]
		},
		{
			label: 'ammo',
			columns: [
				{ key: 'ammo_pack', label: 'pack' },
				{ key: 'ammo_mag', label: 'mag' },
				{ key: 'charge', label: 'charge' },
				{ key: 'is_energy', label: 'energy' }
			]
		}
	];

	const extendedFields = $derived(recordToFields(tickPlayer.extended, `${indexedPrefix}.extended`));
	const queueFields = $derived(
		recordToFields(tickPlayer.update_queue, `${indexedPrefix}.update_queue`)
	);
	const bipedFields = $derived(recordToFields(tickPlayer.biped_tag, `${indexedPrefix}.biped_tag`));

	const bonesGroups: ColGroup[] = [
		{
			label: 'bone',
			columns: [
				{ key: 'index', label: 'idx' },
				{ key: 'x', label: 'x' },
				{ key: 'y', label: 'y' },
				{ key: 'z', label: 'z' }
			]
		}
	];

	// Cast through unknown so the typed scraper structs (WeaponInfo, TickBone)
	// satisfy ColGroupedTable's open-ended Record<string, unknown> rows prop.
	const weaponRows = $derived(
		(tickPlayer.weapons ?? []) as unknown as ReadonlyArray<Record<string, unknown>>
	);
	const boneRows = $derived(
		(tickPlayer.bones ?? []) as unknown as ReadonlyArray<Record<string, unknown>>
	);
</script>

<header class="mb-3 flex flex-wrap items-center gap-3">
	<span
		class="size-2.5 rounded-full {tickPlayer.alive ? 'bg-success-500' : 'bg-error-500'}"
		title={tickPlayer.alive ? 'alive' : 'dead'}
	></span>
	{#if teamGame}
		<span class="size-3 rounded-sm {accent.dot}" title={teamLabel(gamePlayer?.team ?? 0)}></span>
	{/if}
	<h3 class="min-w-0 flex-1 truncate h4">{displayName}</h3>
	<span class="text-surface-500-400 font-mono text-xs">#{idx}</span>
</header>

<div class="grid gap-3 sm:grid-cols-2">
	<div class="card preset-tonal p-3">
		<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			life / shields
		</header>
		<KvGrid fields={lifeFields} />
	</div>
	<div class="card preset-tonal p-3">
		<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			position
		</header>
		<KvGrid fields={positionFields} />
	</div>
	<div class="card preset-tonal p-3">
		<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			velocity
		</header>
		<KvGrid fields={velocityFields} />
	</div>
	<div class="card preset-tonal p-3">
		<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			aim / zoom
		</header>
		<KvGrid fields={aimFields} />
	</div>
	<div class="card preset-tonal p-3 sm:col-span-2">
		<header class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			combat / input state
		</header>
		<KvGrid fields={combatFields} />
	</div>
</div>

<section class="mt-3">
	<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
		weapons ({tickPlayer.weapons?.length ?? 0})
	</header>
	<ColGroupedTable
		rows={weaponRows}
		groups={weaponsGroups}
		emptyMessage="no weapons"
		annotationPrefix="{indexedPrefix}.weapons"
	/>
</section>

{#if extendedFields.length > 0}
	<section class="mt-3">
		<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
			extended
		</header>
		<div class="card preset-tonal p-3">
			<KvGrid fields={extendedFields} />
		</div>
	</section>
{/if}

{#if queueFields.length > 0}
	<section class="mt-3">
		<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
			update_queue
		</header>
		<div class="card preset-tonal p-3">
			<KvGrid fields={queueFields} />
		</div>
	</section>
{/if}

{#if bipedFields.length > 0}
	<section class="mt-3">
		<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
			biped_tag
		</header>
		<div class="card preset-tonal p-3">
			<KvGrid fields={bipedFields} />
		</div>
	</section>
{/if}

{#if (tickPlayer.bones ?? []).length > 0}
	<section class="mt-3">
		<header class="text-surface-700-200 mb-1 text-xs font-semibold tracking-wide uppercase">
			bones ({tickPlayer.bones?.length ?? 0})
		</header>
		<ColGroupedTable
			rows={boneRows}
			groups={bonesGroups}
			annotationPrefix="{indexedPrefix}.bones"
		/>
	</section>
{/if}
