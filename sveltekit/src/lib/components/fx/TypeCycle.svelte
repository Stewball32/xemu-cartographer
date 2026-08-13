<script lang="ts">
	/**
	 * TypeCycle — types, holds, deletes, and rotates through words.
	 * Save to src/lib/components/fx/TypeCycle.svelte
	 *
	 *   <h1>Build <TypeCycle words={['dashboards', 'inboxes', 'wizards']} class="text-primary-400" /></h1>
	 *
	 * Reserves the widest word so the line never reflows. Under
	 * prefers-reduced-motion it swaps whole words instead of typing.
	 */
	import { onMount } from 'svelte';

	let {
		words,
		typeSpeed = 70, // ms per char typed
		deleteSpeed = 40, // ms per char deleted
		hold = 1600, // ms a completed word stays
		caret = true,
		class: className = ''
	}: {
		words: string[];
		typeSpeed?: number;
		deleteSpeed?: number;
		hold?: number;
		caret?: boolean;
		class?: string;
	} = $props();

	let text = $state('');
	let timer: ReturnType<typeof setTimeout>;

	onMount(() => {
		let wi = 0;
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
			const swap = () => {
				text = words[wi];
				wi = (wi + 1) % words.length;
				timer = setTimeout(swap, hold + 900);
			};
			swap();
		} else {
			const type = (i: number) => {
				text = words[wi].slice(0, i);
				if (i < words[wi].length) timer = setTimeout(() => type(i + 1), typeSpeed);
				else timer = setTimeout(() => del(words[wi].length), hold);
			};
			const del = (i: number) => {
				text = words[wi].slice(0, i);
				if (i > 0) timer = setTimeout(() => del(i - 1), deleteSpeed);
				else {
					wi = (wi + 1) % words.length;
					timer = setTimeout(() => type(1), 300);
				}
			};
			type(1);
		}
		return () => clearTimeout(timer);
	});

	const widest = $derived(words.reduce((a, b) => (b.length > a.length ? b : a), ''));
</script>

<span
	class={className}
	style="display: inline-grid; text-align: left;"
	aria-label={words.join(', ')}
>
	<span style="visibility: hidden; grid-area: 1 / 1; white-space: nowrap;" aria-hidden="true"
		>{widest}&nbsp;</span
	>
	<span style="grid-area: 1 / 1; white-space: nowrap;" aria-hidden="true"
		>{text}{#if caret}<span class="tc-caret"></span>{/if}</span
	>
</span>

<style>
	.tc-caret {
		display: inline-block;
		width: 2px;
		height: 1em;
		margin-left: 2px;
		vertical-align: -0.1em;
		background: currentColor;
		animation: tc-blink 1s step-end infinite;
	}
	@keyframes tc-blink {
		50% {
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.tc-caret {
			animation: none;
		}
	}
</style>
