<script lang="ts">
	import { fieldAnnotations, type FieldMark } from '$lib/stores/fieldAnnotations.svelte';

	let { key }: { key: string } = $props();

	const mark = $derived<FieldMark>(fieldAnnotations.get(key));

	const symbol = $derived(mark === 'ok' ? '✓' : mark === 'broken' ? '✗' : '?');
	const tone = $derived(
		mark === 'ok'
			? 'bg-success-500 text-success-contrast-500'
			: mark === 'broken'
				? 'bg-error-500 text-error-contrast-500'
				: 'bg-surface-200-800 text-surface-700-300'
	);
	const labelText = $derived(
		mark === 'ok' ? 'Plausible' : mark === 'broken' ? 'Broken' : 'Unverified'
	);
</script>

<button
	type="button"
	class="inline-flex size-4 shrink-0 items-center justify-center rounded text-[9px] leading-none font-bold {tone} hover:ring-1 hover:ring-current"
	onclick={(e) => {
		e.stopPropagation();
		fieldAnnotations.cycle(key);
	}}
	title="{labelText} — click to cycle"
	aria-label="Field validation: {labelText}"
>
	{symbol}
</button>
