<script lang="ts">
	import { resolve } from '$app/paths';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';
</script>

<PagePlaceholder
	title="Roles"
	milestone="M08"
	description="Role + permission management. Promote users to admin, scope per-collection write access, and define future roles (team captain, league commissioner)."
	backHref={resolve('/admin/')}
	backLabel="Back to admin"
/>
