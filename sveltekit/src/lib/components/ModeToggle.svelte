<script lang="ts">
	import { mode } from '$lib/stores/mode.svelte';
	import { SunIcon, MoonIcon } from '@lucide/svelte';

	const isDark = $derived(mode.current === 'dark');
	const label = $derived(isDark ? 'Switch to light mode' : 'Switch to dark mode');
</script>

<button
	type="button"
	class="btn-icon preset-tonal btn-sm"
	onclick={() => mode.toggle()}
	title={label}
	aria-label={label}
>
	{#if isDark}
		<SunIcon class="size-4" />
	{:else}
		<MoonIcon class="size-4" />
	{/if}
</button>
