<!--
	Panel.svelte — Star Command translucent panel (design system SS4 recipe).
	Token-driven: correct under any Skeleton v5 theme, tuned for dark surfaces.

	Props:
	  bevel  — skin-style bevel: light top/left, near-black bottom/right
	  blur   — backdrop-filter blur (default true)
	  accent — 'none' | 'primary' | 'secondary' — tinted border + soft glow
	           (one accent per surface: blue = LAN/ops, orange = events/CTA)
	  class  — padding/layout classes, e.g. "p-6"
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	let {
		bevel = false,
		blur = true,
		accent = 'none',
		class: className = '',
		children
	}: {
		bevel?: boolean;
		blur?: boolean;
		accent?: 'none' | 'primary' | 'secondary';
		class?: string;
		children?: Snippet;
	} = $props();
</script>

<div class="sc-panel {className}" class:blur class:bevel data-accent={accent}>
	{@render children?.()}
</div>

<style>
	.sc-panel {
		position: relative;
		background: color-mix(in srgb, var(--color-surface-900) 86%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-surface-50) 9%, transparent);
		border-radius: var(--radius-container);
	}
	.blur {
		backdrop-filter: blur(5px);
	}
	.bevel {
		border-top-color: color-mix(in srgb, var(--color-surface-50) 12%, transparent);
		border-left-color: color-mix(in srgb, var(--color-surface-50) 10%, transparent);
		border-bottom-color: oklch(0.05 0.01 266 / 0.6);
		border-right-color: oklch(0.05 0.01 266 / 0.6);
	}
	.sc-panel[data-accent='primary'] {
		border-color: color-mix(in srgb, var(--color-primary-500) 35%, transparent);
		box-shadow: 0 0 24px color-mix(in srgb, var(--color-primary-500) 15%, transparent);
	}
	.sc-panel[data-accent='secondary'] {
		border-color: color-mix(in srgb, var(--color-secondary-500) 40%, transparent);
		box-shadow: 0 0 24px color-mix(in srgb, var(--color-secondary-500) 18%, transparent);
	}
</style>
