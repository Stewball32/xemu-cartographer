<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';

	const slug = $derived(page.params.slug);
</script>

<PagePlaceholder
	title={slug ?? ''}
	milestone="M23"
	description="Team detail: name + slug + Founded by, current roster with owner/manager badges, former-members section. Lands with M23 (team membership workflows)."
	backHref={resolve('/teams/')}
	backLabel="Back to teams"
/>
