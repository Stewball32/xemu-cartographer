<script lang="ts">
	/**
	 * AnimateIn — staggered entrance for direct children when scrolled into view.
	 * Save to src/lib/components/fx/AnimateIn.svelte
	 *
	 *   <AnimateIn stagger={60} class="grid gap-4 sm:grid-cols-3">
	 *     {#each features as f}<div class="card p-6">...</div>{/each}
	 *   </AnimateIn>
	 *
	 * Progressive enhancement: content is visible without JS (static build),
	 * then hidden+revealed only after hydration. Honors prefers-reduced-motion.
	 */
	import { onMount } from 'svelte';
	import type { Snippet } from 'svelte';

	let {
		stagger = 60, // ms between children
		delay = 0, // ms before the first child
		y = 12, // px rise distance
		duration = 500,
		once = true, // false = replay every time it re-enters the viewport
		class: className = '',
		children
	}: {
		stagger?: number;
		delay?: number;
		y?: number;
		duration?: number;
		once?: boolean;
		class?: string;
		children: Snippet;
	} = $props();

	let root: HTMLElement;

	export function replay() {
		hide();
		requestAnimationFrame(() => requestAnimationFrame(show));
	}

	let items: HTMLElement[] = [];

	function hide() {
		items.forEach((el) => {
			el.style.transition = 'none';
			el.style.opacity = '0';
			el.style.transform = `translateY(${y}px)`;
		});
	}

	function show() {
		items.forEach((el, i) => {
			el.style.transition = `opacity ${duration}ms ease-out ${delay + i * stagger}ms, transform ${duration}ms cubic-bezier(0.2, 0.7, 0.3, 1) ${delay + i * stagger}ms`;
			el.style.opacity = '1';
			el.style.transform = 'translateY(0)';
		});
	}

	onMount(() => {
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		items = Array.from(root.children) as HTMLElement[];
		hide();
		const io = new IntersectionObserver(
			([e]) => {
				if (e.isIntersecting) {
					show();
					if (once) io.disconnect();
				} else if (!once) {
					hide();
				}
			},
			{ threshold: 0.15 }
		);
		io.observe(root);
		return () => io.disconnect();
	});
</script>

<div bind:this={root} class={className}>
	{@render children()}
</div>
