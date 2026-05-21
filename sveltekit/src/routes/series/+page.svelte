<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Series"
	milestone="M14"
	description="Series browser. Filter by team, format, status (live / completed). Each row links to the series detail page; '+ New' starts the creation flow."
	backHref={resolve('/')}
	backLabel="Back home"
/>
