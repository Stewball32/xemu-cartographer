<script lang="ts">
	import TabPlaceholder from '../TabPlaceholder.svelte';

	let { name }: { name: string } = $props();
</script>

<TabPlaceholder
	title="Debug"
	description={`Placeholder for "${name}" — will render the debug envelope (players, state inputs, score probe).`}
/>
