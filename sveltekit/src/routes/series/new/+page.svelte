<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="New series"
	milestone="M14"
	description="Series creation form. Pick format (single, exact-N, best-of-N, first-to-X), participants (teams or ad-hoc gamertags), category, and optionally run a pick/ban round."
	backHref={resolve('/series/')}
	backLabel="Back to series"
/>
