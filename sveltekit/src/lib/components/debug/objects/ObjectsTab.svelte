<script lang="ts">
	import TabPlaceholder from '../TabPlaceholder.svelte';

	let { name }: { name: string } = $props();
</script>

<TabPlaceholder
	title="Objects"
	description={`Placeholder for "${name}" — will render the objects envelope (live object + projectile arrays at ~30 Hz).`}
/>
