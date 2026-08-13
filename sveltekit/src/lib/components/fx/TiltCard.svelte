<script lang="ts">
	/**
	 * TiltCard — pointer-tracking 3D tilt with an optional moving glare.
	 * Save to src/lib/components/fx/TiltCard.svelte
	 *
	 *   <TiltCard class="card p-6" max={8}>…</TiltCard>
	 *
	 * Inactive on coarse (touch) pointers and under prefers-reduced-motion.
	 */
	import type { Snippet } from 'svelte';

	let {
		max = 8, // deg of tilt at the edges
		scale = 1.02,
		glare = true,
		class: className = '',
		children
	}: {
		max?: number;
		scale?: number;
		glare?: boolean;
		class?: string;
		children: Snippet;
	} = $props();

	let el: HTMLElement;
	let glareEl: HTMLElement | undefined = $state();

	function active() {
		return (
			!window.matchMedia('(prefers-reduced-motion: reduce)').matches &&
			window.matchMedia('(pointer: fine)').matches
		);
	}
	function move(e: PointerEvent) {
		if (!active()) return;
		const r = el.getBoundingClientRect();
		const px = (e.clientX - r.left) / r.width - 0.5;
		const py = (e.clientY - r.top) / r.height - 0.5;
		el.style.transition = 'transform 60ms linear';
		el.style.transform = `perspective(650px) rotateX(${(-py * max).toFixed(2)}deg) rotateY(${(px * max).toFixed(2)}deg) scale(${scale})`;
		if (glareEl)
			glareEl.style.background = `radial-gradient(circle at ${((px + 0.5) * 100).toFixed(1)}% ${((py + 0.5) * 100).toFixed(1)}%, color-mix(in oklab, var(--color-surface-50) 16%, transparent), transparent 60%)`;
	}
	function leave() {
		el.style.transition = 'transform 300ms ease';
		el.style.transform = 'none';
		if (glareEl) glareEl.style.background = 'none';
	}
</script>

<!-- decorative hover tilt only — pointer handlers add no functionality, so no interactive role -->
<div
	bind:this={el}
	role="presentation"
	class={className}
	style="position: relative; will-change: transform;"
	onpointermove={move}
	onpointerleave={leave}
>
	{#if glare}
		<div
			bind:this={glareEl}
			style="position: absolute; inset: 0; border-radius: inherit; pointer-events: none;"
			aria-hidden="true"
		></div>
	{/if}
	{@render children()}
</div>
