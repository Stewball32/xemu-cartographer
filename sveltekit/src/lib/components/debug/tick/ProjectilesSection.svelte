<script lang="ts">
	// Renders TickPayload.projectiles — one row per live projectile (bullet,
	// grenade, plasma bolt, etc.) the scraper enumerated this tick. The wide
	// per-projectile field set (timers, deceleration, rotation) makes this the
	// busiest table in the Tick tab; ColGroupedTable's horizontal scroll
	// handles it without truncation.
	import type { ColGroup } from '../shared/col-grouped-table';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { TickProjectile } from '$lib/types/scraper';

	let {
		projectiles,
		showHeader = true
	}: {
		projectiles: TickProjectile[] | null | undefined;
		showHeader?: boolean;
	} = $props();

	function hex(n: number | undefined): string {
		if (n === undefined) return '—';
		const u = n >>> 0;
		return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
	}

	const groups: ColGroup[] = [
		{
			label: 'Projectile',
			columns: [
				{ key: 'object_id_hex', label: 'id' },
				{ key: 'tag', label: 'tag' },
				{ key: 'flags_hex', label: 'flags' },
				{ key: 'action', label: 'action' }
			]
		},
		{
			label: 'Position',
			columns: [
				{ key: 'x', label: 'x' },
				{ key: 'y', label: 'y' },
				{ key: 'z', label: 'z' }
			]
		},
		{
			label: 'Hit',
			columns: [
				{ key: 'hit_material_type', label: 'material' },
				{ key: 'ignore_object_index', label: 'ignore' },
				{ key: 'target_object_index', label: 'target' }
			]
		},
		{
			label: 'Detonation',
			columns: [
				{ key: 'detonation_timer', label: 'timer' },
				{ key: 'detonation_timer_delta', label: 'Δ' },
				{ key: 'arming_time_delta', label: 'arm Δ' }
			]
		},
		{
			label: 'Travel',
			columns: [
				{ key: 'distance_traveled', label: 'distance' },
				{ key: 'deceleration_timer', label: 'decel t' },
				{ key: 'deceleration_timer_delta', label: 'decel Δ' },
				{ key: 'deceleration', label: 'decel' },
				{ key: 'maximum_damage_distance', label: 'max dmg dist' }
			]
		},
		{
			label: 'Rotation',
			columns: [
				{ key: 'rotation_axis_x', label: 'ax x' },
				{ key: 'rotation_axis_y', label: 'ax y' },
				{ key: 'rotation_axis_z', label: 'ax z' },
				{ key: 'rotation_sine', label: 'sin' },
				{ key: 'rotation_cosine', label: 'cos' }
			]
		}
	];

	const rows = $derived(
		(projectiles ?? []).map((p) => ({
			object_id_hex: hex(p.object_id),
			tag: p.tag || '—',
			flags_hex: hex(p.flags),
			action: p.action,
			x: p.x,
			y: p.y,
			z: p.z,
			hit_material_type: p.hit_material_type,
			ignore_object_index: p.ignore_object_index,
			target_object_index: p.target_object_index,
			detonation_timer: p.detonation_timer,
			detonation_timer_delta: p.detonation_timer_delta,
			arming_time_delta: p.arming_time_delta,
			distance_traveled: p.distance_traveled,
			deceleration_timer: p.deceleration_timer,
			deceleration_timer_delta: p.deceleration_timer_delta,
			deceleration: p.deceleration,
			maximum_damage_distance: p.maximum_damage_distance,
			rotation_axis_x: p.rotation_axis_x,
			rotation_axis_y: p.rotation_axis_y,
			rotation_axis_z: p.rotation_axis_z,
			rotation_sine: p.rotation_sine,
			rotation_cosine: p.rotation_cosine
		}))
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Projectiles ({rows.length})
		</div>
	{/if}
	<ColGroupedTable {rows} {groups} emptyMessage="no projectiles in this tick" />
</section>
