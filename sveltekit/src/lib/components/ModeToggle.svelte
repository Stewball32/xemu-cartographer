<script lang="ts">
	import { mode } from '$lib/stores/mode.svelte';
	import { SunIcon, MoonIcon } from '@lucide/svelte';
	import { Switch } from '@skeletonlabs/skeleton-svelte';

	const isDark = $derived(mode.current === 'dark');
	const label = $derived(isDark ? 'Switch to light mode' : 'Switch to dark mode');
</script>

<Switch
	class="h-6 w-11"
	checked={isDark}
	onCheckedChange={(details) => mode.set(details.checked ? 'dark' : 'light')}
	aria-label={label}
	title={label}
>
	<Switch.Control
		class="inline-flex  cursor-pointer items-center rounded-full bg-surface-300-700 transition-colors data-[state=checked]:bg-primary-500"
	>
		<Switch.Thumb
			class="flex size-5 items-center justify-center rounded-full bg-white shadow-sm transition-transform "
		>
			{#if isDark}
				<MoonIcon class="size-3 text-primary-700" />
			{:else}
				<SunIcon class="size-3 text-warning-500" />
			{/if}
		</Switch.Thumb>
	</Switch.Control>
	<Switch.HiddenInput />
</Switch>
