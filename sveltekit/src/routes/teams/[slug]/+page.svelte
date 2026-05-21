<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';

	const slug = $derived(page.params.slug);
</script>

<PagePlaceholder
	title={slug ?? ''}
	milestone="M15"
	description="Team stats and roster. K/D/A rolled across only the games where players were repping this team. Per-game-type filters mirror the user profile."
	backHref={resolve('/teams/')}
	backLabel="Back to teams"
/>
