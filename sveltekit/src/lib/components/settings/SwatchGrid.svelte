<script lang="ts">
	// Named color-swatch grid (armor/emblem palettes): header row with the
	// selected color's name, then dot + name buttons at a ≥38px touch target.
	import type { PaletteColor } from '$lib/utils/emblem';

	let {
		label,
		colors,
		selected,
		onpick
	}: {
		label: string;
		colors: PaletteColor[];
		selected: number;
		onpick: (i: number) => void;
	} = $props();
</script>

<div class="flex flex-col gap-1.5">
	<div class="flex justify-between gap-2 text-xs text-surface-600-400">
		<span>{label}</span>
		<span class="font-semibold opacity-70">{colors[selected]?.name ?? '—'}</span>
	</div>
	<div class="grid grid-cols-[repeat(auto-fill,minmax(106px,1fr))] gap-1.5">
		{#each colors as c, i (i)}
			<button
				type="button"
				title={c.name}
				aria-pressed={i === selected}
				class="inline-flex min-h-9.5 min-w-0 items-center gap-1.5 rounded-md border px-2 py-1 text-left text-[11.5px] font-semibold transition-colors
					{i === selected
					? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
					: 'border-surface-500/20 bg-surface-100-900 text-surface-600-400 hover:text-surface-950-50'}"
				onclick={() => onpick(i)}
			>
				<span
					class="size-4.5 flex-none rounded-[5px] border border-black/45 shadow-[inset_0_1px_0_rgba(255,255,255,0.15)]"
					style="background:{c.hex}"
				></span>
				<span class="truncate">{c.name}</span>
			</button>
		{/each}
	</div>
</div>
