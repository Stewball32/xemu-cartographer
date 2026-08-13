<script lang="ts">
	/**
	 * PageTransition — cross-fade + rise between route navigations.
	 * Save to src/lib/components/fx/PageTransition.svelte
	 *
	 * Wrap the page slot once in +layout.svelte:
	 *   <PageTransition>{@render children()}</PageTransition>
	 *
	 * Uses the View Transitions API where available; silently does nothing
	 * elsewhere and under prefers-reduced-motion, so it is safe everywhere.
	 */
	import { onNavigate } from '$app/navigation';
	import type { Snippet } from 'svelte';

	let {
		duration = 220, // ms
		y = 8, // px rise of the incoming page
		children
	}: { duration?: number; y?: number; children: Snippet } = $props();

	onNavigate((navigation) => {
		if (!('startViewTransition' in document)) return;
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		document.documentElement.style.setProperty('--pt-duration', `${duration}ms`);
		document.documentElement.style.setProperty('--pt-y', `${y}px`);
		return new Promise((resolve) => {
			(
				document as Document & { startViewTransition: (cb: () => Promise<void>) => void }
			).startViewTransition(async () => {
				resolve();
				await navigation.complete;
			});
		});
	});
</script>

{@render children()}

<style>
	:global(::view-transition-old(root)) {
		animation: pt-out var(--pt-duration, 220ms) ease both;
	}
	:global(::view-transition-new(root)) {
		animation: pt-in var(--pt-duration, 220ms) cubic-bezier(0.2, 0.7, 0.3, 1) both;
	}
	@keyframes -global-pt-out {
		to {
			opacity: 0;
		}
	}
	@keyframes -global-pt-in {
		from {
			opacity: 0;
			transform: translateY(var(--pt-y, 8px));
		}
	}
</style>
