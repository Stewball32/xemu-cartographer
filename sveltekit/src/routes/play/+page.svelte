<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { Gamepad2Icon, LoaderIcon } from '@lucide/svelte';
	import { apiBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		reducePlayState,
		INITIAL_PLAY_STATE,
		type PlayState,
		type MeMatchResponse
	} from '$lib/utils/play-match';
	import PlayKiosk from '$lib/components/play/PlayKiosk.svelte';

	let play = $state<PlayState>(INITIAL_PLAY_STATE);

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	// Ask the server which container (if any) the caller's gamertag is rostered
	// in. host:summary is admin-only, so the match is resolved server-side
	// rather than by reading the cross-instance feed on the client.
	async function resolveMatch() {
		if (!auth.token) {
			play = { phase: 'idle', container: null };
			return;
		}
		try {
			const res = await fetch(`${apiBaseURL()}/api/me/match`, {
				headers: { Authorization: auth.token }
			});
			if (!res.ok) {
				play = { phase: 'idle', container: null };
				return;
			}
			play = reducePlayState((await res.json()) as MeMatchResponse);
		} catch {
			// Network hiccup — fall back to idle; the next poll/focus retries.
			play = { phase: 'idle', container: null };
		}
	}

	function onFocus() {
		resolveMatch();
	}

	onMount(() => {
		resolveMatch();
		window.addEventListener('focus', onFocus);
		// Poll-on-focus is the primary refresh; the interval catches a gamertag
		// moving to a different container while the tab stays foregrounded.
		pollTimer = setInterval(() => {
			if (document.visibilityState === 'visible') resolveMatch();
		}, 5000);
	});

	onDestroy(() => {
		window.removeEventListener('focus', onFocus);
		if (pollTimer !== null) clearInterval(pollTimer);
	});
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<Gamepad2Icon class="size-5" />
		<h1 class="h4">Play</h1>
	</header>

	{#if play.phase === 'loading'}
		<div class="flex items-center gap-2 card preset-tonal p-6 text-sm">
			<LoaderIcon class="size-4 animate-spin" />
			<span>Finding your match…</span>
		</div>
	{:else if play.phase === 'matched' && play.container}
		<!-- Re-key on container so the kiosk + VNC tear down and reconnect when a
		     gamertag moves to a different machine. -->
		{#key play.container}
			<PlayKiosk name={play.container} />
		{/key}
	{:else}
		<div class="flex flex-col items-center gap-2 card preset-tonal p-10 text-center">
			<Gamepad2Icon class="size-8 text-surface-500" />
			<p class="font-medium">You're not in an active match right now.</p>
			<p class="max-w-md text-sm text-surface-600-400">
				When one of your gamertags joins a game on a tracked console, your kiosk shows up here
				automatically. Make sure the gamertag you're playing under is added under
				<a class="anchor" href={resolve('/settings/')}>Settings</a>.
			</p>
		</div>
	{/if}
</div>
