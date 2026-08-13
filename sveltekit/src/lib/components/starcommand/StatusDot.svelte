<!--
	StatusDot.svelte — glowing instrument-panel status dot (SS6).

	Props:
	  tone  — 'success' | 'primary' | 'secondary' | 'warning' | 'error' | 'muted'
	  size  — px (default 9)
	  pulse — breathing glow (reduced-motion safe)
	  label — optional caps label beside the dot
-->
<script lang="ts">
	let {
		tone = 'success',
		size = 9,
		pulse = false,
		label = ''
	}: {
		tone?: 'success' | 'primary' | 'secondary' | 'warning' | 'error' | 'muted';
		size?: number;
		pulse?: boolean;
		label?: string;
	} = $props();
	const colors: Record<string, string> = {
		success: 'var(--color-success-400)',
		primary: 'var(--color-primary-400)',
		secondary: 'var(--color-secondary-400)',
		warning: 'var(--color-warning-400)',
		error: 'var(--color-error-500)',
		muted: 'var(--color-tertiary-500)'
	};
</script>

<span class="wrap">
	<i class="dot" class:pulse style="width:{size}px;height:{size}px;color:{colors[tone]};"></i>
	{#if label}<span class="label">{label}</span>{/if}
</span>

<style>
	.wrap {
		display: inline-flex;
		align-items: center;
		gap: 0.5em;
	}
	.dot {
		display: inline-block;
		border-radius: 50%;
		background: currentColor;
		box-shadow: 0 0 10px currentColor;
	}
	.pulse {
		animation: sc-dot-pulse 2.6s ease-in-out infinite;
	}
	.label {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.3em;
		text-transform: uppercase;
		color: var(--color-tertiary-400);
	}
	@keyframes sc-dot-pulse {
		0%,
		100% {
			box-shadow: 0 0 6px currentColor;
			opacity: 0.75;
		}
		50% {
			box-shadow: 0 0 14px currentColor;
			opacity: 1;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.pulse {
			animation: none;
		}
	}
</style>
