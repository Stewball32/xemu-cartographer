<!--
	AccentChip.svelte — tinted chip/badge (SS6): 14% tint bg, 38% tint border,
	accent text, 6px radius, caps. One accent per surface.

	Props: tone — 'primary' | 'secondary' | 'success' | 'warning' | 'error' | 'tertiary'
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	let {
		tone = 'primary',
		class: className = '',
		children
	}: {
		tone?: 'primary' | 'secondary' | 'success' | 'warning' | 'error' | 'tertiary';
		class?: string;
		children?: Snippet;
	} = $props();
	const t = $derived('--color-' + tone);
</script>

<span
	class="sc-chip {className}"
	style="--t-500:var({t}-500);--t-400:var({t}-400);--t-300:var({t}-300);"
>
	{@render children?.()}
</span>

<style>
	.sc-chip {
		display: inline-flex;
		align-items: center;
		gap: 0.5em;
		padding: 3px 9px;
		border-radius: 6px;
		background: color-mix(in srgb, var(--t-500) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--t-400) 38%, transparent);
		color: var(--t-300);
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		white-space: nowrap;
	}
</style>
