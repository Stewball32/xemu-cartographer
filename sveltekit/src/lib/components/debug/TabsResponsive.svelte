<script lang="ts" module>
	export type TabItem = { value: string; label: string; count?: number };
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Tabs } from '@skeletonlabs/skeleton-svelte';

	let {
		value,
		onValueChange,
		items,
		ariaLabel,
		children,
		class: extraClass = ''
	}: {
		value: string;
		onValueChange: (next: string) => void;
		items: TabItem[];
		ariaLabel: string;
		children: Snippet;
		class?: string;
	} = $props();
</script>

<div class={extraClass}>
	<div class="mb-3 sm:hidden">
		<label class="sr-only" for="tabs-responsive-select">{ariaLabel}</label>
		<select
			id="tabs-responsive-select"
			class="select"
			{value}
			onchange={(e) => onValueChange(e.currentTarget.value)}
			aria-label={ariaLabel}
		>
			{#each items as it (it.value)}
				<option value={it.value}>
					{it.label}{it.count !== undefined ? ` (${it.count})` : ''}
				</option>
			{/each}
		</select>
	</div>

	<Tabs {value} onValueChange={(d) => onValueChange(d.value)}>
		<Tabs.List class="hidden sm:flex">
			{#each items as it (it.value)}
				<Tabs.Trigger value={it.value}>
					{it.label}
					{#if it.count !== undefined}
						<span class="text-surface-700-200 ml-1 text-xs">({it.count})</span>
					{/if}
				</Tabs.Trigger>
			{/each}
			<Tabs.Indicator />
		</Tabs.List>
		{@render children()}
	</Tabs>
</div>
