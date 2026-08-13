<script lang="ts">
	/**
	 * BuildStamp — build/commit stamp for spot-checking stale bundles.
	 * Save to src/lib/components/fx/BuildStamp.svelte
	 *
	 * Requires version wiring in svelte.config.js (see USAGE.md):
	 *   kit: { version: { name: <git commit>, pollInterval: 60_000 } }
	 *
	 *   <BuildStamp />                        <!-- fixed bottom-right chip -->
	 *   <BuildStamp inline label="v1.4.2" />  <!-- e.g. sidebar footer -->
	 *   <BuildStamp>                          <!-- anchor mode: zero footprint -->
	 *     <a href="/"><img src={logo} alt="" class="size-8" /></a>
	 *   </BuildStamp>
	 *
	 * Anchor mode (pass children): wraps your app icon; nothing renders until
	 * hover/focus, then a small card shows the running commit (click copies).
	 * When SvelteKit detects a newer deploy, an amber dot appears on the
	 * icon's corner and the card gains a Reload button.
	 */
	import { version } from '$app/environment';
	import { updated } from '$app/state';
	import type { Snippet } from 'svelte';

	let {
		label = '', // optional human version shown before the commit, e.g. 'v1.4.2'
		inline = false, // chip mode: false = fixed to a viewport corner
		corner = 'bottom-right',
		children
	}: {
		label?: string;
		inline?: boolean;
		corner?: 'bottom-right' | 'bottom-left';
		children?: Snippet;
	} = $props();

	const short = version.slice(0, 7);

	function copy() {
		navigator.clipboard?.writeText(version);
	}
	function click() {
		if (updated.current) location.reload();
		else copy();
	}
</script>

{#if children}
	<!-- min-w-0 so a truncating child (e.g. an app name beside the logo) can still
	     shrink: this span becomes the flex item in the host layout. -->
	<span class="group relative inline-flex min-w-0">
		{@render children()}
		{#if updated.current}
			<span
				class="pointer-events-none absolute -top-0.5 -right-0.5 size-2 animate-pulse rounded-full ring-2 ring-surface-50-950"
				style="background: var(--color-warning-500);"
			></span>
		{/if}
		<!-- pt-2 (not mt-2) keeps hover unbroken across the gap -->
		<span
			class="pointer-events-none absolute top-full left-0 z-50 pt-2 opacity-0 transition-opacity duration-150 group-focus-within:pointer-events-auto group-focus-within:opacity-100 group-hover:pointer-events-auto group-hover:opacity-100"
		>
			<span
				class="flex items-center gap-2 card preset-filled-surface-100-900 px-3 py-2 font-mono text-[11px] leading-none whitespace-nowrap shadow-xl"
			>
				<span
					class="size-1.5 shrink-0 rounded-full"
					style="background: {updated.current
						? 'var(--color-warning-500)'
						: 'var(--color-success-500)'};"
				></span>
				{#if label}<span>{label}</span><span class="opacity-50">·</span>{/if}
				{#if updated.current}
					<span>new deploy live</span>
					<button
						type="button"
						class="rounded-base preset-tonal px-1.5 py-1 hover:opacity-80"
						onclick={() => location.reload()}
					>
						Reload
					</button>
				{:else}
					<button
						type="button"
						class="opacity-70 hover:underline hover:opacity-100"
						title="click to copy full commit"
						onclick={copy}
					>
						{short}
					</button>
				{/if}
			</span>
		</span>
	</span>
{:else}
	<button
		onclick={click}
		title={updated.current
			? 'New deployment available — click to reload'
			: `build ${version} — click to copy`}
		class="flex items-center gap-1.5 font-mono text-[10px] leading-none opacity-35 transition-opacity hover:opacity-90"
		style={inline
			? ''
			: `position: fixed; ${corner === 'bottom-left' ? 'left' : 'right'}: 10px; bottom: 10px; z-index: 40;`}
	>
		<span
			class="size-1.5 rounded-full"
			style="background: {updated.current
				? 'var(--color-warning-500)'
				: 'var(--color-success-500)'};"
		></span>
		{#if label}<span>{label}</span><span class="opacity-50">·</span>{/if}
		<span>{updated.current ? 'update available' : short}</span>
	</button>
{/if}
