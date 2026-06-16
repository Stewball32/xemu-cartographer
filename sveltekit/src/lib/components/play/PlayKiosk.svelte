<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { apiBaseURL, wsBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import KioskFrame from '$lib/components/kiosk/KioskFrame.svelte';
	import XboxController from '$lib/components/kiosk/XboxController.svelte';
	import { VNCKeyboard } from '$lib/utils/vnc-keyboard';

	// The container the caller's gamertag is rostered in (resolved server-side
	// by /api/me/match). Reuses the same kiosk HTTP proxy + VNC relay as the
	// admin view — the M09 backend narrows both to admit roster members, so a
	// player reaches their own match without admin rights. Unlike ControlsTab
	// there is no admin container-detail fetch and no start/stop controls: if
	// you're in a match, the container is running by definition.
	let { name }: { name: string } = $props();

	let vnc: VNCKeyboard | null = null;
	let vncConnected = $state(false);
	let kioskSrc = $state('');

	function kioskURL(): string {
		return `${apiBaseURL()}/api/admin/containers/${encodeURIComponent(name)}/kiosk/?token=${encodeURIComponent(auth.token ?? '')}`;
	}

	function vncURL(): string {
		return `${wsBaseURL()}/api/admin/containers/${encodeURIComponent(name)}/vnc?token=${encodeURIComponent(auth.token ?? '')}`;
	}

	onMount(() => {
		// untrack so a cross-tab authStore token rotation doesn't rewrite src
		// and tear down KioskFrame's {#key src} noVNC session (same rationale
		// as ControlsTab).
		kioskSrc = untrack(() => kioskURL());
		vnc = new VNCKeyboard(vncURL(), (c) => (vncConnected = c));
		vnc.connect();
	});

	onDestroy(() => {
		vnc?.disconnect();
		vnc = null;
	});
</script>

<div class="flex flex-col gap-4 lg:grid lg:grid-cols-[2fr_1fr] lg:gap-4">
	<KioskFrame {name} src={kioskSrc} running={true} {vncConnected} externalHref={kioskURL()} />

	<div class="flex min-h-0 flex-col gap-2 overflow-hidden card preset-tonal p-3">
		<header class="flex items-center justify-between text-xs">
			<span class="font-medium">Xbox controller</span>
			<span class={vncConnected ? 'text-success-500' : 'text-surface-600-400'}>
				{vncConnected ? 'VNC connected' : 'connecting…'}
			</span>
		</header>
		<XboxController
			disabled={!vncConnected}
			onPress={(sym) => vnc?.sendKey(sym, true)}
			onRelease={(sym) => vnc?.sendKey(sym, false)}
		/>
	</div>
</div>
