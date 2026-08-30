<script lang="ts">
	// One in-game-style option row: label · ‹ value ›. Options wrap around; the
	// value reads muted at the factory default and accent once changed. Disabled
	// mode is the H2 controls' WIP preview (bytes not mapped yet).
	import { ChevronLeftIcon, ChevronRightIcon } from '@lucide/svelte';

	let {
		label,
		options,
		value,
		defaultIndex = 0,
		disabled = false,
		onchange
	}: {
		label: string;
		options: string[];
		value: number;
		defaultIndex?: number;
		disabled?: boolean;
		onchange?: (i: number) => void;
	} = $props();

	function cycle(d: number) {
		if (disabled || options.length === 0) return;
		onchange?.((value + d + options.length) % options.length);
	}
</script>

<div
	class="flex items-center gap-2.5 rounded-lg border border-surface-500/20 py-1.5 pr-1.5 pl-3.5
		{disabled ? 'opacity-60' : ''}"
>
	<span class="min-w-0 flex-1 truncate text-[12.5px] font-semibold">{label}</span>
	<span class="inline-flex flex-none items-center gap-0.5">
		<button
			type="button"
			class="btn-icon preset-tonal"
			title="Previous option"
			{disabled}
			onclick={() => cycle(-1)}
		>
			<ChevronLeftIcon class="size-4" />
		</button>
		<span
			class="min-w-26 text-center font-mono text-[11.5px] font-bold whitespace-nowrap
				{value === defaultIndex ? 'text-surface-600-400' : 'text-primary-600-400'}"
		>
			{options[value] ?? '—'}
		</span>
		<button
			type="button"
			class="btn-icon preset-tonal"
			title="Next option"
			{disabled}
			onclick={() => cycle(1)}
		>
			<ChevronRightIcon class="size-4" />
		</button>
	</span>
</div>
