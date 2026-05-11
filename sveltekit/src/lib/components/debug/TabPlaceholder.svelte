<script lang="ts">
	interface Props {
		title: string;
		description: string;
	}

	let { title, description }: Props = $props();
</script>

<div class="card preset-tonal mx-auto max-w-2xl p-6 text-center">
	<h2 class="h4 mb-2">{title}</h2>
	<p class="text-surface-700-200 text-sm">{description}</p>
</div>
