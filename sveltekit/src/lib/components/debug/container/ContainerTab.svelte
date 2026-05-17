<script lang="ts">
	import TabPlaceholder from '../TabPlaceholder.svelte';

	let { name }: { name: string } = $props();
</script>

<TabPlaceholder
	title="Container"
	description={`Placeholder for "${name}" — will render the podman container details (status, ports, mounts).`}
/>
