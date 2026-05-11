<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		description,
		actions,
		class: extraClass = ''
	}: {
		title: string;
		description?: string;
		actions?: Snippet;
		class?: string;
	} = $props();
</script>

<header class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between {extraClass}">
	<div class="min-w-0">
		<h1 class="h2">{title}</h1>
		{#if description}
			<p class="mt-1 text-sm text-surface-700-300">{description}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex flex-wrap items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</header>
