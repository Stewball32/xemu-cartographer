<script lang="ts">
	/**
	 * ParticleField — theme-aware floating particles on <canvas>.
	 * Save to src/lib/components/fx/ParticleField.svelte
	 *
	 * Drop inside any `position: relative` container (it fills it), e.g.
	 *   <section class="relative">
	 *     <ParticleField density={40} />
	 *     <div class="relative">...content...</div>
	 *   </section>
	 * Colors default to the active theme's primary/secondary/tertiary 500s,
	 * so it follows data-theme and light/dark automatically.
	 */
	import { onMount } from 'svelte';

	let {
		density = 40, // particles per ~1M px²... scaled to area
		speed = 1, // drift multiplier
		colors = undefined as string[] | undefined,
		maxSize = 2.5, // px radius
		opacity = 0.6,
		class: className = ''
	} = $props();

	let canvas: HTMLCanvasElement;

	onMount(() => {
		const context = canvas.getContext('2d');
		if (!context) return;
		// Re-bind as a narrowed const so the hoisted function declarations below see it as non-null.
		const ctx: CanvasRenderingContext2D = context;

		const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
		let raf = 0;
		let running = false;
		let w = 0;
		let h = 0;
		let dpr = 1;

		type P = {
			x: number;
			y: number;
			r: number;
			c: string;
			vy: number;
			sway: number;
			phase: number;
			tw: number;
		};
		let parts: P[] = [];

		function themeColors(): string[] {
			if (colors?.length) return colors;
			const cs = getComputedStyle(document.documentElement);
			return ['--color-primary-500', '--color-secondary-500', '--color-tertiary-500']
				.map((v) => cs.getPropertyValue(v).trim())
				.filter(Boolean);
		}

		function seed() {
			const cols = themeColors();
			if (!cols.length) return;
			const count = Math.round(((w * h) / 1_000_000) * density * 25);
			parts = Array.from({ length: count }, () => ({
				x: Math.random() * w,
				y: Math.random() * h,
				r: 0.8 + Math.random() * (maxSize - 0.8),
				c: cols[Math.floor(Math.random() * cols.length)],
				vy: (0.08 + Math.random() * 0.25) * speed,
				sway: 8 + Math.random() * 18,
				phase: Math.random() * Math.PI * 2,
				tw: 0.4 + Math.random() * 0.6
			}));
		}

		function resize() {
			const rect = canvas.parentElement?.getBoundingClientRect();
			if (!rect) return;
			dpr = Math.min(window.devicePixelRatio || 1, 2);
			w = rect.width;
			h = rect.height;
			canvas.width = w * dpr;
			canvas.height = h * dpr;
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			seed();
			if (reduced.matches) draw(0); // static scatter, no animation
		}

		function draw(t: number) {
			ctx.clearRect(0, 0, w, h);
			for (const p of parts) {
				const x = p.x + Math.sin(t / 4000 + p.phase) * p.sway;
				const a = opacity * p.tw * (0.6 + 0.4 * Math.sin(t / 1600 + p.phase));
				ctx.globalAlpha = Math.max(0.08, a);
				ctx.fillStyle = p.c;
				ctx.beginPath();
				ctx.arc(x, p.y, p.r, 0, Math.PI * 2);
				ctx.fill();
			}
			ctx.globalAlpha = 1;
		}

		function tick(t: number) {
			if (!running) return;
			for (const p of parts) {
				p.y -= p.vy;
				if (p.y < -4) {
					p.y = h + 4;
					p.x = Math.random() * w;
				}
			}
			draw(t);
			raf = requestAnimationFrame(tick);
		}

		function start() {
			if (running || reduced.matches) return;
			running = true;
			raf = requestAnimationFrame(tick);
		}
		function stop() {
			running = false;
			cancelAnimationFrame(raf);
		}

		const ro = new ResizeObserver(resize);
		ro.observe(canvas.parentElement ?? canvas);
		// Pause offscreen + when the tab is hidden
		const io = new IntersectionObserver(([e]) => (e.isIntersecting ? start() : stop()));
		io.observe(canvas);
		const onVis = () => (document.hidden ? stop() : start());
		document.addEventListener('visibilitychange', onVis);
		// Re-read colors when data-theme or .dark changes
		const mo = new MutationObserver(seed);
		mo.observe(document.documentElement, {
			attributes: true,
			attributeFilter: ['data-theme', 'class']
		});

		resize();
		start();
		return () => {
			stop();
			ro.disconnect();
			io.disconnect();
			mo.disconnect();
			document.removeEventListener('visibilitychange', onVis);
		};
	});
</script>

<canvas
	bind:this={canvas}
	class="pointer-events-none absolute inset-0 h-full w-full {className}"
	aria-hidden="true"
></canvas>
