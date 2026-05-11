<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		open,
		onClose,
		title,
		children,
		footer,
		size = 'md'
	}: {
		open: boolean;
		onClose: () => void;
		title?: string;
		children: Snippet;
		footer?: Snippet;
		size?: 'sm' | 'md' | 'lg';
	} = $props();

	const widthClass = $derived(
		size === 'sm' ? 'max-w-sm' : size === 'md' ? 'max-w-md' : 'max-w-2xl'
	);

	function onBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) onClose();
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={open ? onKey : undefined} />

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
		role="presentation"
		onclick={onBackdropClick}
	>
		<div
			class="w-full card preset-tonal {widthClass} max-h-[90vh] overflow-y-auto p-6"
			role="dialog"
			aria-modal="true"
			aria-label={title}
		>
			{#if title}
				<h2 class="mb-4 h4">{title}</h2>
			{/if}
			{@render children()}
			{#if footer}
				<div class="mt-6 flex flex-wrap justify-end gap-2">
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}
