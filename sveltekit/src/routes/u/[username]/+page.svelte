<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';

	const username = $derived(page.params.username);
</script>

<PagePlaceholder
	title={`@${username}`}
	milestone="M15"
	description="User profile with K/D/A rolled across all of this user's gamertags, per-game-type filters, date-range scoping, and a match-history feed linking to game + series detail pages."
	backHref={resolve('/')}
	backLabel="Back home"
/>
