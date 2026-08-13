<script lang="ts">
	/**
	 * ConfettiBurst — imperative confetti for success moments.
	 * Save to src/lib/components/fx/ConfettiBurst.svelte
	 *
	 *   <ConfettiBurst bind:this={confetti} />
	 *   <button onclick={(e) => { confetti.fire(e.clientX, e.clientY); save(); }}>Done</button>
	 *
	 * fire(x?, y?, count?) takes viewport coords (defaults to center). Colors
	 * come from the active theme (primary/secondary/tertiary/warning 500).
	 * No-op under prefers-reduced-motion. Mount once per page, near the root.
	 */
	let canvas: HTMLCanvasElement | undefined = $state();
	let raf = 0;

	export function fire(x?: number, y?: number, count = 90) {
		if (!canvas || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const ctx = canvas.getContext('2d');
		if (!ctx) return;
		const dpr = window.devicePixelRatio || 1;
		const w = window.innerWidth;
		const h = window.innerHeight;
		canvas.width = w * dpr;
		canvas.height = h * dpr;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		const cs = getComputedStyle(document.documentElement);
		const colors = [
			'--color-primary-500',
			'--color-secondary-500',
			'--color-tertiary-500',
			'--color-warning-500'
		]
			.map((v) => cs.getPropertyValue(v).trim())
			.filter(Boolean);
		if (!colors.length) colors.push('#38bdf8');
		const cx = x ?? w / 2;
		const cy = y ?? h / 2;
		const parts = Array.from({ length: count }, () => {
			const angle = Math.random() * Math.PI * 2;
			const speed = Math.random() * 7 + 3;
			return {
				x: cx,
				y: cy,
				vx: Math.cos(angle) * speed,
				vy: Math.sin(angle) * speed - 4,
				size: Math.random() * 6 + 3,
				color: colors[(Math.random() * colors.length) | 0],
				rot: Math.random() * Math.PI,
				vr: (Math.random() - 0.5) * 0.3
			};
		});
		const t0 = performance.now();
		let lastT = t0;
		cancelAnimationFrame(raf);
		const step = (now: number) => {
			const dt = Math.min(32, now - lastT) / 16.7;
			lastT = now;
			const life = (now - t0) / 1600;
			ctx.clearRect(0, 0, w, h);
			if (life >= 1) return;
			for (const p of parts) {
				p.vy += 0.22 * dt;
				p.x += p.vx * dt;
				p.y += p.vy * dt;
				p.rot += p.vr * dt;
				ctx.save();
				ctx.globalAlpha = 1 - life;
				ctx.translate(p.x, p.y);
				ctx.rotate(p.rot);
				ctx.fillStyle = p.color;
				ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size * 0.6);
				ctx.restore();
			}
			raf = requestAnimationFrame(step);
		};
		raf = requestAnimationFrame(step);
	}
</script>

<canvas
	bind:this={canvas}
	style="position: fixed; inset: 0; width: 100vw; height: 100vh; pointer-events: none; z-index: 9999;"
	aria-hidden="true"
></canvas>
