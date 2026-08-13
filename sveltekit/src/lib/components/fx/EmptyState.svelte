<script lang="ts">
	/**
	 * EmptyState — friendly themed zero-data placeholder.
	 * Save to src/lib/components/fx/EmptyState.svelte
	 *
	 *   <EmptyState title="No messages yet" description="When someone writes to you, it lands here.">
	 *     {#snippet icon()}<InboxIcon size={20} />{/snippet}
	 *     {#snippet action()}<button class="btn preset-filled">New message</button>{/snippet}
	 *   </EmptyState>
	 */
	import type { Snippet } from 'svelte';

	let {
		title,
		description = '',
		class: className = '',
		icon,
		action
	}: {
		title: string;
		description?: string;
		class?: string;
		icon?: Snippet;
		action?: Snippet;
	} = $props();
</script>

<div
	class="flex flex-col items-center gap-2 rounded-container border border-dashed border-surface-200-800 px-4 py-8 text-center {className}"
>
	<div
		class="mb-1 flex size-11 items-center justify-center rounded-full text-primary-400"
		style="background: color-mix(in oklab, var(--color-primary-500) 12%, transparent);"
	>
		{#if icon}
			{@render icon()}
		{:else}
			<svg
				width="20"
				height="20"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
				><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"></polyline><path
					d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"
				></path></svg
			>
		{/if}
	</div>
	<p class="font-semibold">{title}</p>
	{#if description}<p class="max-w-xs text-sm opacity-60">{description}</p>{/if}
	{#if action}<div class="mt-2">{@render action()}</div>{/if}
</div>
