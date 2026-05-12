<script lang="ts">
	// Renders TickPayload.objects — every dynamic engine object the scraper saw
	// this tick (excludes players, projectiles, power items; those have their
	// own sections). Tag string + hex IDs let the user verify the object-table
	// walk landed on real entries.
	import type { ColGroup } from '../shared/col-grouped-table';
	import ColGroupedTable from '../shared/ColGroupedTable.svelte';
	import type { TickObject } from '$lib/types/scraper';

	let {
		objects,
		showHeader = true
	}: {
		objects: TickObject[] | null | undefined;
		showHeader?: boolean;
	} = $props();

	const groups: ColGroup[] = [
		{
			label: 'Object',
			columns: [
				{ key: 'object_id_hex', label: 'id' },
				{ key: 'tag', label: 'tag' },
				{ key: 'type', label: 'type' },
				{ key: 'flags_hex', label: 'flags' }
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
			label: 'Angular velocity',
			columns: [
				{ key: 'ang_vel_x', label: 'avx' },
				{ key: 'ang_vel_y', label: 'avy' },
				{ key: 'ang_vel_z', label: 'avz' }
			]
		},
		{
			label: 'Lifetime',
			columns: [
				{ key: 'unk_damage_1', label: 'dmg1' },
				{ key: 'time_existing', label: 'age' }
			]
		},
		{
			label: 'Ownership',
			columns: [
				{ key: 'owner_unit_hex', label: 'owner unit' },
				{ key: 'owner_object_hex', label: 'owner obj' },
				{ key: 'ultimate_parent_hex', label: 'parent' }
			]
		}
	];

	function hex(n: number | undefined): string {
		if (n === undefined) return '—';
		const u = n >>> 0;
		return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
	}

	const rows = $derived(
		(objects ?? []).map((o) => ({
			object_id_hex: hex(o.object_id),
			tag: o.tag || '—',
			type: o.type,
			flags_hex: hex(o.flags),
			x: o.x,
			y: o.y,
			z: o.z,
			ang_vel_x: o.ang_vel_x,
			ang_vel_y: o.ang_vel_y,
			ang_vel_z: o.ang_vel_z,
			unk_damage_1: o.unk_damage_1,
			time_existing: o.time_existing,
			owner_unit_hex: hex(o.owner_unit_ref),
			owner_object_hex: hex(o.owner_object_ref),
			ultimate_parent_hex: hex(o.ultimate_parent)
		}))
	);
</script>

<section>
	{#if showHeader}
		<div class="text-surface-700-200 mb-2 text-xs font-semibold tracking-wide uppercase">
			Objects ({rows.length})
		</div>
	{/if}
	<ColGroupedTable {rows} {groups} emptyMessage="no objects in this tick" />
</section>
