<script lang="ts">
	// Draft-then-save footer bar — edits queue in the page's draft state; this
	// bar is the single commit point (Nameplates is the exception: instant
	// writes, no bar).
	import { LoaderIcon } from '@lucide/svelte';

	let {
		dirty,
		busy = false,
		saveLabel = 'Save changes',
		onsave,
		ondiscard
	}: {
		dirty: boolean;
		busy?: boolean;
		saveLabel?: string;
		onsave: () => void;
		ondiscard: () => void;
	} = $props();
</script>

<div
	class="flex flex-wrap items-center gap-3 border-t border-surface-200-800 pt-3"
	aria-live="polite"
>
	{#if dirty}
		<span class="size-2 rounded-full bg-primary-500 shadow-[0_0_8px] shadow-primary-500/60"></span>
		<span class="text-sm font-medium">Unsaved changes</span>
	{:else}
		<span class="text-xs text-surface-500"
			>No pending edits — changes queue here until you save.</span
		>
	{/if}
	<span class="flex-1"></span>
	<button class="btn preset-tonal btn-sm" onclick={ondiscard} disabled={!dirty || busy}>
		Discard
	</button>
	<button class="btn preset-filled btn-sm" onclick={onsave} disabled={!dirty || busy}>
		{#if busy}<LoaderIcon class="size-4 animate-spin" />{/if}
		<span>{saveLabel}</span>
	</button>
</div>
