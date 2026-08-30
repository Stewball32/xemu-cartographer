<script lang="ts">
	// Master-detail scaffold shared by Offsets / Discs / Gametypes / Rulesets:
	// desktop = list beside detail; tablet = `compact` strip (pills) above the
	// detail when supplied, else the same split; phone = the list IS the page,
	// opening swaps to the detail with a back arrow (page supplies open/onback).
	import { ArrowLeftIcon } from '@lucide/svelte';
	import type { Snippet } from 'svelte';

	let {
		open,
		onback,
		backLabel = 'Back',
		listWidth = '280px',
		list,
		compact,
		detail
	}: {
		/** whether a detail is open (drives the phone swap). */
		open: boolean;
		onback: () => void;
		backLabel?: string;
		listWidth?: string;
		list: Snippet;
		/** optional tablet strip (e.g. pills) shown above the detail. */
		compact?: Snippet;
		detail: Snippet;
	} = $props();
</script>

<!-- Phone (< md): list ⇄ detail swap -->
<div class="flex flex-col gap-4 md:hidden">
	{#if open}
		<button class="btn w-fit preset-tonal btn-sm" onclick={onback} aria-label="Back to the list">
			<ArrowLeftIcon class="size-4" /><span>{backLabel}</span>
		</button>
		{@render detail()}
	{:else}
		{@render list()}
	{/if}
</div>

<!-- Tablet (md, < lg): compact strip above the detail when supplied -->
{#if compact}
	<div class="hidden flex-col gap-4 md:flex lg:hidden">
		{@render compact()}
		{@render detail()}
	</div>
	<div class="hidden gap-4 lg:grid" style="grid-template-columns: {listWidth} minmax(0, 1fr)">
		{@render list()}
		{@render detail()}
	</div>
{:else}
	<div class="hidden gap-4 md:grid" style="grid-template-columns: {listWidth} minmax(0, 1fr)">
		{@render list()}
		{@render detail()}
	</div>
{/if}
