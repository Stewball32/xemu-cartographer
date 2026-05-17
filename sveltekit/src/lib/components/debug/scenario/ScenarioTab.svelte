<script lang="ts">
	import TabPlaceholder from '../TabPlaceholder.svelte';

	let { name }: { name: string } = $props();
</script>

<TabPlaceholder
	title="Scenario"
	description={`Placeholder for "${name}" — will render the scenario envelope (map, memory regions, object types, spawns, tag defs).`}
/>
