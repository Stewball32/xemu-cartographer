<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';

	const id = $derived(page.params.id);
</script>

<PagePlaceholder
	title={`Series #${id}`}
	milestone="M14"
	description="Series-in-progress UI. Header (format, participants, category), per-game scoreboard, current standing, next map if pick/ban committed. Auto-updates via PB realtime."
	backHref={resolve('/series/')}
	backLabel="Back to series"
/>
