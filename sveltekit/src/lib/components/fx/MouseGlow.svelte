<script lang="ts">
	/**
	 * MouseGlow — a soft theme-tinted glow that follows the pointer.
	 * Save to src/lib/components/fx/MouseGlow.svelte
	 *
	 * Drop inside any `position: relative` container:
	 *   <section class="relative">
	 *     <MouseGlow />
	 *     <div class="relative">...content...</div>
	 *   </section>
	 * Color defaults to the active theme's --color-primary-500.
	 */
	import { onMount } from 'svelte';

	let {
		size = 380, // px diameter
		strength = 0.18, // 0..1 color mix
		color = undefined as string | undefined
	} = $props();

	let wrap: HTMLElement;
	let glow: HTMLElement;

	onMount(() => {
		const parent = wrap.parentElement;
		if (!parent) return;
		const c =
			color ??
			getComputedStyle(document.documentElement).getPropertyValue('--color-primary-500').trim() ??
			'currentColor';
		glow.style.background = `radial-gradient(circle, color-mix(in oklab, ${c} ${Math.round(strength * 100)}%, transparent) 0%, transparent 65%)`;
		const move = (e: PointerEvent) => {
			const r = parent.getBoundingClientRect();
			glow.style.left = e.clientX - r.left + 'px';
			glow.style.top = e.clientY - r.top + 'px';
			glow.style.opacity = '1';
		};
		const leave = () => (glow.style.opacity = '0');
		parent.addEventListener('pointermove', move);
		parent.addEventListener('pointerleave', leave);
		return () => {
			parent.removeEventListener('pointermove', move);
			parent.removeEventListener('pointerleave', leave);
		};
	});
</script>

<div
	bind:this={wrap}
	class="pointer-events-none absolute inset-0 overflow-hidden"
	aria-hidden="true"
>
	<div
		bind:this={glow}
		style="position: absolute; width: {size}px; height: {size}px; transform: translate(-50%, -50%); border-radius: 50%; opacity: 0; transition: opacity 300ms ease; left: 50%; top: 50%;"
	></div>
</div>
