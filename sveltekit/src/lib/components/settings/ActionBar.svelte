<script lang="ts">
	// The contextual sticky action bar: note on the left (mono for save
	// telemetry, with a green status dot on the game tabs), actions on the
	// right. Wraps on narrow. Accounts renders no bar at all.
	import type { Snippet } from 'svelte';

	let {
		note,
		mono = false,
		dot = false,
		children
	}: {
		note: string;
		mono?: boolean;
		dot?: boolean;
		children: Snippet;
	} = $props();
</script>

<div
	class="sticky bottom-3 z-5 flex flex-wrap items-center justify-between gap-x-3.5 gap-y-2.5 rounded-xl border border-surface-500/25 bg-surface-50-950/95 px-4 py-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_12px_32px_rgba(0,0,0,0.5)] backdrop-blur-sm"
>
	<span class="inline-flex min-w-0 items-center gap-2">
		{#if dot}
			<span
				class="size-2 flex-none rounded-full bg-success-500 shadow-[0_0_6px] shadow-success-500/60"
			></span>
		{/if}
		<span class="text-[11.5px] opacity-70 {mono ? 'font-mono' : ''}">{note}</span>
	</span>
	<span class="inline-flex flex-wrap items-center gap-2.5">
		{@render children()}
	</span>
</div>
