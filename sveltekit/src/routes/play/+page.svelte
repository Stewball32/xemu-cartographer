<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Play"
	milestone="M09"
	description="Logged-in player auto-routes to the kiosk for the container their gamertag is currently in. Renders an idle state when you're not in an active match."
	backHref={resolve('/')}
	backLabel="Back home"
/>
