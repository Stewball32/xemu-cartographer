<script lang="ts">
	/**
	 * SkeletonBlock — shimmer placeholder that swaps to real content.
	 * Save to src/lib/components/fx/SkeletonBlock.svelte
	 *
	 *   <SkeletonBlock loaded={!!records} lines={3} avatar>
	 *     {#each records as r}…{/each}
	 *   </SkeletonBlock>
	 *
	 * Placeholder shapes use Skeleton's `placeholder` classes plus the kit's
	 * `.shimmer` sweep (motion.css) — static under prefers-reduced-motion.
	 */
	import type { Snippet } from 'svelte';

	let {
		loaded = false,
		lines = 3,
		avatar = false,
		media = false, // tall media block above the lines
		class: className = '',
		children
	}: {
		loaded?: boolean;
		lines?: number;
		avatar?: boolean;
		media?: boolean;
		class?: string;
		children?: Snippet;
	} = $props();

	const widths = ['w-3/4', 'w-full', 'w-1/2', 'w-5/6', 'w-2/3'];
</script>

{#if loaded && children}
	{@render children()}
{:else}
	<div class="shimmer space-y-3 {className}" role="status" aria-label="Loading">
		{#if avatar}
			<div class="flex items-center gap-3">
				<div class="placeholder-circle w-10"></div>
				<div class="placeholder w-1/3"></div>
			</div>
		{/if}
		{#if media}
			<div class="h-32 placeholder w-full"></div>
		{/if}
		{#each Array.from({ length: lines }, (_, i) => i) as line (line)}
			<div class="placeholder {widths[line % widths.length]}"></div>
		{/each}
	</div>
{/if}
