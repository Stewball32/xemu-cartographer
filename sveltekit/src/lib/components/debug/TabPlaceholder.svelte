<script lang="ts">
	interface Props {
		title: string;
		description: string;
	}

	let { title, description }: Props = $props();
</script>

<div class="mx-auto max-w-2xl card preset-tonal p-6 text-center">
	<h2 class="mb-2 h4">{title}</h2>
	<p class="text-surface-700-200 text-sm">{description}</p>
</div>
