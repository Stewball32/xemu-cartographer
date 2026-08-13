<script lang="ts">
	/**
	 * CountUp — animates a number from 0 when it scrolls into view.
	 * Save to src/lib/components/fx/CountUp.svelte
	 *
	 *   <p class="text-3xl font-bold"><CountUp value={1234} /></p>
	 *   <CountUp value={8.3} decimals={1} prefix="+" suffix="%" />
	 *
	 * Renders the final value without JS; honors prefers-reduced-motion.
	 */
	import { onMount } from 'svelte';

	let {
		value,
		duration = 1200,
		decimals = 0,
		prefix = '',
		suffix = '',
		locale = true // thousands separators via toLocaleString
	}: {
		value: number;
		duration?: number;
		decimals?: number;
		prefix?: string;
		suffix?: string;
		locale?: boolean;
	} = $props();

	// Intentional initial-value capture: start at the final value so the number is correct
	// without JS / under reduced motion; the IO animation resets it to 0 and counts up.
	// Re-render with {#key ...} when `value` can change after mount.
	// svelte-ignore state_referenced_locally
	let shown = $state(value);
	let el: HTMLElement;

	const fmt = (n: number) =>
		locale
			? n.toLocaleString(undefined, {
					minimumFractionDigits: decimals,
					maximumFractionDigits: decimals
				})
			: n.toFixed(decimals);

	onMount(() => {
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const io = new IntersectionObserver(
			([e]) => {
				if (!e.isIntersecting) return;
				io.disconnect();
				const t0 = performance.now();
				const step = (t: number) => {
					const p = Math.min(1, (t - t0) / duration);
					const ease = 1 - Math.pow(2, -10 * p); // easeOutExpo
					shown = value * (p === 1 ? 1 : ease);
					if (p < 1) requestAnimationFrame(step);
				};
				shown = 0;
				requestAnimationFrame(step);
			},
			{ threshold: 0.4 }
		);
		io.observe(el);
		return () => io.disconnect();
	});
</script>

<span bind:this={el}>{prefix}{fmt(shown)}{suffix}</span>
