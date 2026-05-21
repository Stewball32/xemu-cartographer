<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Identity"
	milestone="M07"
	description="CRUD UI for gamertags, teams, and team rosters (trosters). Users manage their own gamertags and default pick; team captains/managers manage their team's roster."
	backHref={resolve('/admin/')}
	backLabel="Back to admin"
/>
