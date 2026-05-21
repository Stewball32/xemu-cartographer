<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Teams"
	milestone="M15"
	description="Teams browser. Each row links to the team detail page with roster + aggregated stats."
	backHref={resolve('/')}
	backLabel="Back home"
/>
