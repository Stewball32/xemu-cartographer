<script lang="ts">
	import { ArrowLeftIcon, ConstructionIcon } from '@lucide/svelte';

	let {
		title,
		milestone,
		description,
		backHref,
		backLabel = 'Back'
	}: {
		title: string;
		milestone: string;
		description: string;
		backHref: string;
		backLabel?: string;
	} = $props();
</script>

<div class="mx-auto flex max-w-3xl flex-col gap-4">
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
	<a href={backHref} class="inline-flex items-center gap-1 anchor text-sm">
		<ArrowLeftIcon class="size-4" />
		{backLabel}
	</a>
	<h1 class="h2">{title}</h1>
	<div class="card preset-tonal-warning p-3">
		<div class="flex items-center gap-2 text-sm">
			<ConstructionIcon class="size-4" />
			<span class="font-medium">Placeholder for {milestone}</span>
		</div>
		<p class="mt-1 text-sm">{description}</p>
	</div>
</div>
