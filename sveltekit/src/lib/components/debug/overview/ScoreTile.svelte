<script lang="ts">
	let {
		label,
		value,
		limit,
		accent
	}: {
		label: string;
		value: number;
		limit: number;
		accent: { bg: string; dot: string };
	} = $props();

	const pct = $derived(limit > 0 ? Math.max(0, Math.min(100, (value / limit) * 100)) : 0);
</script>

<div class="flex min-w-0 flex-col gap-1.5 card preset-tonal p-2">
	<div class="flex items-baseline justify-between gap-2">
		<span
			class="flex items-center gap-1.5 truncate text-[10px] font-semibold tracking-wide uppercase"
		>
			<span class="block size-2 shrink-0 rounded-sm {accent.dot}"></span>
			<span class="truncate">{label}</span>
		</span>
		<span class="text-surface-700-200 shrink-0 font-mono text-xs tabular-nums">
			{#if limit > 0}
				<span class="font-bold text-surface-950-50">{value}</span>
				<span class="text-surface-700-200">/{limit}</span>
			{:else}
				<span class="font-bold text-surface-950-50">{value}</span>
				<span class="ml-1 text-[10px] italic">no limit</span>
			{/if}
		</span>
	</div>
	{#if limit > 0}
		<div class="relative h-1.5 overflow-hidden rounded-full bg-surface-300-700">
			<div class="h-full {accent.dot}" style="width: {pct}%"></div>
		</div>
	{/if}
</div>
