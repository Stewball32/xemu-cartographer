<script lang="ts">
	import TabPlaceholder from '../TabPlaceholder.svelte';

	let { name }: { name: string } = $props();
</script>

<TabPlaceholder
	title="Summary"
	description={`Placeholder for "${name}" — will render the cross-instance summary envelope (host list from host:summary).`}
/>
