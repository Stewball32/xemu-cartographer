<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Games"
	milestone="M13"
	description="Admin view over the persisted games + series collections. Browse, search, and re-attach orphaned games to a series after the fact."
	backHref={resolve('/admin/')}
	backLabel="Back to admin"
/>
