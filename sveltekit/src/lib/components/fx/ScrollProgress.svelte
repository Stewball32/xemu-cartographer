<script lang="ts">
	/**
	 * ScrollProgress — thin reading-progress bar fixed to the top edge.
	 * Save to src/lib/components/fx/ScrollProgress.svelte
	 *
	 *   <ScrollProgress />                      <!-- tracks window scroll -->
	 *   <ScrollProgress target="#docs-pane" />  <!-- tracks a scroll container -->
	 */
	import { onMount } from 'svelte';

	let {
		target = '', // CSS selector of a scroll container; empty = window
		height = 3, // px
		zIndex = 50
	}: { target?: string; height?: number; zIndex?: number } = $props();

	let progress = $state(0);

	onMount(() => {
		const el: HTMLElement | Window = target
			? ((document.querySelector(target) as HTMLElement) ?? window)
			: window;
		const read = () => {
			const [top, max] =
				el instanceof Window
					? [el.scrollY, document.documentElement.scrollHeight - el.innerHeight]
					: [el.scrollTop, el.scrollHeight - el.clientHeight];
			progress = max > 0 ? Math.min(1, top / max) : 0;
		};
		read();
		el.addEventListener('scroll', read, { passive: true });
		window.addEventListener('resize', read);
		return () => {
			el.removeEventListener('scroll', read);
			window.removeEventListener('resize', read);
		};
	});
</script>

<div
	style:position="fixed"
	style:top="0"
	style:left="0"
	style:right="0"
	style:height="{height}px"
	style:z-index={zIndex}
	style:pointer-events="none"
	style:transform="scaleX({progress})"
	style:transform-origin="left"
	style:background="linear-gradient(90deg, var(--color-primary-500), var(--color-tertiary-500))"
	aria-hidden="true"
></div>
