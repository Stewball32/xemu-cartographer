<script lang="ts">
	/**
	 * Sparkline — tiny inline trend line in theme colors.
	 * Save to src/lib/components/fx/Sparkline.svelte
	 *
	 *   <Sparkline data={[3, 5, 4, 8, 7, 11]} />
	 *   <Sparkline data={churn} color="var(--color-error-500)" fill={false} dot={false} />
	 *
	 * Draws itself in when scrolled into view (skipped under reduced motion).
	 */
	import { onMount } from 'svelte';

	let {
		data,
		width = 120,
		height = 32,
		color = 'var(--color-primary-500)',
		strokeWidth = 2,
		fill = true, // soft area fill under the line
		dot = true, // mark the last point
		animate = true
	}: {
		data: number[];
		width?: number;
		height?: number;
		color?: string;
		strokeWidth?: number;
		fill?: boolean;
		dot?: boolean;
		animate?: boolean;
	} = $props();

	const pad = 3;
	const pts = $derived.by(() => {
		const min = Math.min(...data);
		const span = Math.max(...data) - min || 1;
		return data.map(
			(v, i) =>
				[
					pad + (i / (data.length - 1 || 1)) * (width - pad * 2),
					height - pad - ((v - min) / span) * (height - pad * 2)
				] as const
		);
	});
	const line = $derived(pts.map((p) => `${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(' '));
	const area = $derived(`${pad},${height - pad} ${line} ${width - pad},${height - pad}`);
	const last = $derived(pts[pts.length - 1]);

	let lineEl: SVGPolylineElement;

	onMount(() => {
		if (!animate || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const len = lineEl.getTotalLength();
		lineEl.style.strokeDasharray = `${len}`;
		lineEl.style.strokeDashoffset = `${len}`;
		const io = new IntersectionObserver(
			([e]) => {
				if (!e.isIntersecting) return;
				lineEl.style.transition = 'stroke-dashoffset 900ms ease-out';
				lineEl.style.strokeDashoffset = '0';
				io.disconnect();
			},
			{ threshold: 0.5 }
		);
		io.observe(lineEl);
		return () => io.disconnect();
	});
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label="trend line">
	{#if fill}
		<polygon points={area} fill={color} opacity="0.12" />
	{/if}
	<polyline
		bind:this={lineEl}
		points={line}
		fill="none"
		stroke={color}
		stroke-width={strokeWidth}
		stroke-linecap="round"
		stroke-linejoin="round"
	/>
	{#if dot && last}
		<circle cx={last[0]} cy={last[1]} r={strokeWidth + 1} fill={color} />
	{/if}
</svg>
