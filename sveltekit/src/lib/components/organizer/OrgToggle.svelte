<script lang="ts">
	// The organizer pages' small switch (34×18 track, 14px knob, primary glow
	// when on) with an optional trailing label.
	let {
		on,
		label = '',
		disabled = false,
		onflip
	}: {
		on: boolean;
		label?: string;
		disabled?: boolean;
		onflip: () => void;
	} = $props();
</script>

<button
	type="button"
	class="flex items-center gap-2 disabled:opacity-50"
	role="switch"
	aria-checked={on}
	aria-label={label || 'toggle'}
	{disabled}
	onclick={(e) => {
		e.stopPropagation();
		onflip();
	}}
>
	<span
		class="relative h-[18px] w-[34px] flex-none rounded-full transition-colors
			{on ? 'bg-primary-500 shadow-[0_0_10px] shadow-primary-500/45' : 'bg-surface-500/30'}"
	>
		<span
			class="absolute top-0.5 left-0.5 size-3.5 rounded-full bg-surface-50 transition-transform
				{on ? 'translate-x-4' : 'translate-x-0'}"
		></span>
	</span>
	{#if label}<span class="text-sm">{label}</span>{/if}
</button>
