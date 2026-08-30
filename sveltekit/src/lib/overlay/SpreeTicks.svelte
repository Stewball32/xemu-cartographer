<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Spree tally — one Lucide crosshair per kill on the CURRENT run, floating
	// above the bar's top-left. Each new tick pops up from behind the frame
	// (the strip clips at the bar's top edge); death cascades them back behind
	// it left→right, then clears. Steel by default, orange + glow at ×3, tally
	// groups of five (13px gap after each 5th), capped at 8 icons + "+N".
	//
	// Drive with the live wire values: `spree` drops to 0 on death instantly,
	// so the cascade renders from the last seen count while `dead` is true.
	import { untrack } from 'svelte';

	let { spree = 0, dead = false } = $props();

	let shown = $state(0);
	let dying = $state(false);
	let timer;

	$effect(() => {
		const s = spree;
		const d = dead;
		untrack(() => {
			if (!d) {
				clearTimeout(timer);
				dying = false;
				shown = s;
			} else if (shown > 0 && !dying) {
				dying = true;
				timer = setTimeout(
					() => {
						dying = false;
						shown = 0;
					},
					350 + Math.min(shown, 9) * 80 + 250
				);
			}
		});
	});

	const n = $derived(dead || dying ? shown : spree);
	const capped = $derived(Math.min(n, 8));
	const hot = $derived(n >= 3);
	const color = $derived(hot ? '#F7941D' : '#9FB4D0');
</script>

{#if n > 0 && (!dead || dying)}
	<div class="spree-clip" title="spree ×{n}">
		{#each Array.from({ length: capped }) as _, i (i)}
			<i
				class:down={dying}
				style="{i > 0 && i % 5 === 0 ? 'margin-left:13px;' : ''}{dying
					? `animation-delay:${(i * 0.08).toFixed(2)}s`
					: ''}"
			>
				<svg
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke={color}
					stroke-width="2"
					stroke-linecap="round"
					style={hot ? 'filter:drop-shadow(0 0 4px rgba(247,148,29,0.6))' : ''}
				>
					<circle cx="12" cy="12" r="10" />
					<line x1="22" y1="12" x2="18" y2="12" />
					<line x1="6" y1="12" x2="2" y2="12" />
					<line x1="12" y1="6" x2="12" y2="2" />
					<line x1="12" y1="22" x2="12" y2="18" />
				</svg>
			</i>
		{/each}
		{#if n > 8}
			<i class:down={dying} style={dying ? `animation-delay:${(capped * 0.08).toFixed(2)}s` : ''}>
				<span class="over" style="color:{color}">+{n - 8}</span>
			</i>
		{/if}
	</div>
{/if}

<style>
	/* Clips at the bar's top edge so ticks genuinely emerge from behind it. */
	.spree-clip {
		position: absolute;
		left: 18px;
		bottom: 100%;
		height: 24px;
		overflow: hidden;
		display: flex;
		align-items: flex-end;
		gap: 3px;
		pointer-events: none;
	}
	.spree-clip i {
		display: flex;
		align-items: flex-end;
		animation: tick-up 0.38s cubic-bezier(0.34, 1.56, 0.64, 1) both;
	}
	.spree-clip i.down {
		animation: tick-down 0.32s cubic-bezier(0.55, 0, 1, 0.45) both;
	}
	.over {
		font-family: Inter, system-ui, sans-serif;
		font-size: 12.5px;
		font-weight: 700;
	}
	@keyframes tick-up {
		0% {
			transform: translateY(135%);
			opacity: 0.4;
		}
		70% {
			transform: translateY(-12%);
			opacity: 1;
		}
		100% {
			transform: translateY(0);
			opacity: 1;
		}
	}
	@keyframes tick-down {
		0% {
			transform: translateY(0);
			opacity: 1;
		}
		100% {
			transform: translateY(140%);
			opacity: 0.3;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.spree-clip i {
			animation: none;
		}
	}
</style>
