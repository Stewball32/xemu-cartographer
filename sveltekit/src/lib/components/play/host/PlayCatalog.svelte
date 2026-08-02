<script lang="ts">
	// Phase: catalog. The player has no box yet — pick a game from the library
	// and request an instance. GET /api/play/isos populates the grid; clicking a
	// card fires POST /api/play/request (via the onrequest callback), which
	// provisions + boots a box straight into that game.
	import type { IsoOption } from '$lib/utils/play-hosting';
	import { GamepadIcon, LoaderIcon, RocketIcon } from '@lucide/svelte';

	interface Props {
		isos: IsoOption[];
		loading: boolean;
		error: string | null;
		/** id of the ISO currently being requested (disables the grid), or null. */
		requestingId: string | null;
		onrequest: (isoId: string) => void;
	}

	let { isos, loading, error, requestingId, onrequest }: Props = $props();

	const busy = $derived(requestingId !== null);
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="h5">Start a game</h2>
		<p class="text-sm text-surface-600-400">
			Pick a game and we'll spin up a box for you to host. Friends join over System Link — no setup
			on your end.
		</p>
	</div>

	{#if loading}
		<div class="flex items-center gap-2 card preset-tonal p-6 text-sm">
			<LoaderIcon class="size-4 animate-spin" />
			<span>Loading the game library…</span>
		</div>
	{:else if error}
		<div class="card preset-tonal-error p-4 text-sm">{error}</div>
	{:else if isos.length === 0}
		<div class="flex flex-col items-center gap-2 card preset-tonal p-8 text-center">
			<GamepadIcon class="size-8 text-surface-500" />
			<p class="font-medium">No games are available to play right now.</p>
			<p class="max-w-md text-sm text-surface-600-400">
				An admin needs to add a game to the library before you can request a box.
			</p>
		</div>
	{:else}
		<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each isos as iso (iso.id)}
				<button
					type="button"
					class="flex flex-col gap-2 card preset-tonal p-4 text-left transition-colors hover:preset-filled-primary-500 disabled:opacity-50"
					disabled={busy}
					onclick={() => onrequest(iso.id)}
				>
					<div class="flex items-start gap-2">
						<GamepadIcon class="mt-0.5 size-5 shrink-0" />
						<span class="font-semibold">{iso.name}</span>
						{#if requestingId === iso.id}
							<LoaderIcon class="ml-auto size-4 shrink-0 animate-spin" />
						{:else}
							<RocketIcon class="ml-auto size-4 shrink-0 opacity-60" />
						{/if}
					</div>
					{#if iso.description}
						<p class="line-clamp-3 text-xs text-surface-600-400">{iso.description}</p>
					{/if}
					{#if iso.title_id}
						<span class="mt-auto font-mono text-[0.65rem] text-surface-500">{iso.title_id}</span>
					{/if}
				</button>
			{/each}
		</div>
		<p class="text-xs text-surface-500">
			You can host one box at a time. It's released automatically when left idle.
		</p>
	{/if}
</div>
