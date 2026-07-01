<script lang="ts">
	// Single, audited SVG sink for the emblem UI. `inner` is generated entirely
	// from numeric indices + palette hex colors (never user-supplied text), so
	// rendering it with {@html} carries no injection risk. Centralizing it here is
	// what keeps every other emblem component free of {@html}.
	let {
		inner,
		viewBox = '0 0 100 100',
		title = ''
	}: {
		inner: string;
		viewBox?: string;
		title?: string;
	} = $props();

	const markup = $derived(
		`<svg viewBox="${viewBox}" width="100%" height="100%" preserveAspectRatio="xMidYMid meet" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`
	);
</script>

<div class="rawsvg" role={title ? 'img' : 'presentation'} aria-label={title || undefined}>
	<!-- eslint-disable-next-line svelte/no-at-html-tags -- inner is generated from numeric indices + palette hex only -->
	{@html markup}
</div>

<style>
	.rawsvg {
		display: block;
		width: 100%;
		height: 100%;
		line-height: 0;
	}
</style>
