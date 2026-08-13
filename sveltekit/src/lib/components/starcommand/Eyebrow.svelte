<!--
	Eyebrow.svelte — section label: Inter 700 caps, .3em tracking, muted (SS3).
	Props: tone — 'muted' | 'primary' | 'secondary'
-->
<script lang="ts">
	import type { Snippet } from 'svelte';
	let {
		tone = 'muted',
		class: className = '',
		children
	}: { tone?: 'muted' | 'primary' | 'secondary'; class?: string; children?: Snippet } = $props();
</script>

<span class="sc-eyebrow {className}" data-tone={tone}>{@render children?.()}</span>

<style>
	.sc-eyebrow {
		display: inline-block;
		font-family: 'Inter', system-ui, sans-serif;
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.3em;
		text-transform: uppercase;
		color: var(--color-tertiary-500);
	}
	.sc-eyebrow[data-tone='primary'] {
		color: var(--color-primary-400);
	}
	.sc-eyebrow[data-tone='secondary'] {
		color: var(--color-secondary-300);
	}
</style>
