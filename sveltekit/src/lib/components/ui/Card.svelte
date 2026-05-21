<script lang="ts">
	import type { Snippet } from 'svelte';

	type Size = 'xs' | 'sm' | 'md' | 'lg' | 'flush';
	type Tone = 'tonal' | 'filled' | 'outlined';

	let {
		size = 'md',
		tone = 'tonal',
		class: extraClass = '',
		children
	}: {
		size?: Size;
		tone?: Tone;
		class?: string;
		children: Snippet;
	} = $props();

	const padding = $derived(
		size === 'xs'
			? 'p-2'
			: size === 'sm'
				? 'p-3'
				: size === 'md'
					? 'p-4'
					: size === 'lg'
						? 'p-6'
						: 'p-0'
	);
	const preset = $derived(
		tone === 'tonal'
			? 'preset-tonal'
			: tone === 'filled'
				? 'preset-filled-surface-100-900'
				: 'border border-surface-200-800'
	);
</script>

<div class="card {preset} {padding} {extraClass}">
	{@render children()}
</div>
