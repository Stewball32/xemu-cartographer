<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Players"
	milestone="M23"
	description="Searchable, sortable player directory. Doubles as the user-search surface that M23's invitation flow consumes from /u/[username]/. Lands with M23 (team membership workflows)."
	backHref={resolve('/')}
	backLabel="Back home"
/>
