<script lang="ts">
	// Organizer shell. The old in-page tab bar is gone — each organizer page is
	// its own rail/sidebar entry now (src/lib/config/navigation.ts), so this
	// layout only supplies the shared container. The route group is gated to
	// organizers/admins by +layout.ts.
	let { children } = $props();
</script>

<div class="mx-auto flex w-full max-w-7xl flex-col gap-4 p-4 sm:gap-6 sm:p-6">
	{@render children()}
</div>
