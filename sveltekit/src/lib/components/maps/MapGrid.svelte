<script lang="ts">
	// Thumbnail grid of a build's maps. Multiplayer maps show their top-down
	// render (or a pending/failed placeholder); campaign/ui maps list without an
	// image. Shared by the organizer game-management page + the play catalog.
	import { ImageOffIcon, LoaderIcon, MapIcon } from '@lucide/svelte';
	import { mapThumbURL, type IsoMap } from '$lib/utils/maps';

	interface Props {
		maps: IsoMap[];
		/** compact = smaller tiles for the play catalog. */
		compact?: boolean;
	}
	let { maps, compact = false }: Props = $props();

	// Multiplayer (thumbnailed) maps first — that's the payoff surface.
	const ordered = $derived(
		[...maps].sort((a, b) => {
			const am = a.map_type === 'multiplayer' ? 0 : 1;
			const bm = b.map_type === 'multiplayer' ? 0 : 1;
			return am - bm || a.name.localeCompare(b.name);
		})
	);
</script>

{#if maps.length === 0}
	<p class="text-xs text-surface-500">No maps found on this build yet.</p>
{:else}
	<div
		class="grid gap-2 {compact
			? 'grid-cols-3 sm:grid-cols-4'
			: 'grid-cols-2 sm:grid-cols-3 md:grid-cols-4'}"
	>
		{#each ordered as m (m.id)}
			{@const url = mapThumbURL(m)}
			<div class="flex flex-col gap-1">
				<div
					class="relative flex aspect-square items-center justify-center overflow-hidden rounded bg-surface-200-800"
				>
					{#if url}
						<img src={url} alt={m.name} class="size-full object-contain" loading="lazy" />
					{:else if m.thumb_status === 'pending'}
						<LoaderIcon class="size-5 animate-spin text-surface-500" />
					{:else if m.thumb_status === 'failed'}
						<ImageOffIcon class="size-5 text-surface-500" />
					{:else}
						<MapIcon class="size-5 text-surface-500" />
					{/if}
				</div>
				<span class="truncate text-center text-[0.7rem] leading-tight" title={m.name}>{m.name}</span
				>
				{#if !compact && m.map_type !== 'multiplayer'}
					<span class="text-center text-[0.6rem] text-surface-500 uppercase">{m.map_type}</span>
				{/if}
			</div>
		{/each}
	</div>
{/if}
