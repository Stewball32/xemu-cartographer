<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { resolve } from '$app/paths';
	import { LoaderIcon, PlayIcon, RotateCwIcon, SquareIcon } from '@lucide/svelte';
	import { adminGet, adminPost, AdminFetchError } from '$lib/utils/admin-api';
	import { apiBaseURL, wsBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import { toaster, toastPromise } from '$lib/stores/toaster';
	import KioskFrame, { type SnapshotSlot } from '$lib/components/kiosk/KioskFrame.svelte';
	import XboxController from '$lib/components/kiosk/XboxController.svelte';
	import { VNCKeyboard, KEYSYM } from '$lib/utils/vnc-keyboard';
	import type { ContainerDetail, ContainerStatus } from '$lib/types/containers';

	let { name }: { name: string } = $props();

	type RowStatus = ContainerStatus | 'loading' | string;
	type BusyAction = 'start' | 'stop' | 'restart';

	let detail = $state<ContainerDetail | null>(null);
	let status = $state<RowStatus>('loading');
	let loading = $state(true);
	let busyAction = $state<BusyAction | null>(null);

	let vnc: VNCKeyboard | null = null;
	let vncConnected = $state(false);

	let kioskSrc = $state('');

	let detailTimer: ReturnType<typeof setInterval> | null = null;

	const isRunning = $derived(status === 'running');

	function statusBadgeClass(s: RowStatus): string {
		switch (s) {
			case 'running':
				return 'badge preset-filled-success-500';
			case 'exited':
			case 'stopped':
				return 'badge preset-tonal-error';
			case 'created':
			case 'paused':
			case 'stopping':
				return 'badge preset-tonal-warning';
			case 'loading':
				return 'badge preset-tonal';
			default:
				return 'badge preset-tonal-surface';
		}
	}

	async function loadDetail() {
		try {
			const d = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
			detail = d;
			status = d.status;
		} catch (err) {
			if (err instanceof AdminFetchError && err.status === 404) {
				detail = null;
				status = 'unknown';
			} else {
				console.warn('detail fetch failed', err);
				status = 'unknown';
			}
		} finally {
			loading = false;
		}
	}

	function startPolling() {
		stopPolling();
		detailTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			loadDetail();
		}, 3000);
	}

	function stopPolling() {
		if (detailTimer !== null) {
			clearInterval(detailTimer);
			detailTimer = null;
		}
	}

	function vncURL(): string {
		if (!detail) return '';
		return `${wsBaseURL()}/api/admin/containers/${encodeURIComponent(name)}/vnc?token=${encodeURIComponent(auth.token ?? '')}`;
	}

	function kioskURL(): string {
		if (!detail) return '';
		return `${apiBaseURL()}/api/admin/containers/${encodeURIComponent(name)}/kiosk/?token=${encodeURIComponent(auth.token ?? '')}`;
	}

	function connectVNC() {
		if (!detail) return;
		vnc?.disconnect();
		vnc = new VNCKeyboard(vncURL(), (c) => (vncConnected = c));
		vnc.connect();
	}

	function sendVNCTap(key: string) {
		const sym = KEYSYM[key];
		if (sym == null || !vnc) return;
		vnc.sendKey(sym, true);
		setTimeout(() => vnc?.sendKey(sym, false), 60);
	}

	async function handleStart() {
		busyAction = 'start';
		status = 'loading';
		try {
			await toastPromise(adminPost(`containers/${encodeURIComponent(name)}/start`), {
				loading: { title: 'Starting', description: name },
				success: { title: 'Started', description: name },
				errorTitle: 'Start failed'
			});
		} catch {
			// toast already shown
		} finally {
			busyAction = null;
			await loadDetail();
		}
	}

	async function handleStop() {
		busyAction = 'stop';
		status = 'loading';
		try {
			await toastPromise(adminPost(`containers/${encodeURIComponent(name)}/stop`), {
				loading: { title: 'Stopping', description: name },
				success: { title: 'Stopped', description: name },
				errorTitle: 'Stop failed'
			});
		} catch {
			// toast already shown
		} finally {
			busyAction = null;
			await loadDetail();
		}
	}

	// No /restart endpoint on the backend — emulate with stop + start.
	async function handleRestart() {
		busyAction = 'restart';
		status = 'loading';
		try {
			await toastPromise(
				(async () => {
					await adminPost(`containers/${encodeURIComponent(name)}/stop`);
					await adminPost(`containers/${encodeURIComponent(name)}/start`);
				})(),
				{
					loading: { title: 'Restarting', description: name },
					success: { title: 'Restarted', description: name },
					errorTitle: 'Restart failed'
				}
			);
		} catch {
			// toast already shown
		} finally {
			busyAction = null;
			await loadDetail();
		}
	}

	async function onSnapshotSlot(slot: SnapshotSlot) {
		sendVNCTap(slot.key);
		toaster.info({ title: slot.label, description: slot.body });
	}

	function onResetVM() {
		vnc?.sendChord(['Control_L', 'r']);
		toaster.info({ title: 'Reset', description: 'Sent Ctrl+R to VM' });
	}

	$effect(() => {
		if (isRunning && detail && !vnc) {
			connectVNC();
		}
		if (!isRunning && vnc) {
			vnc.disconnect();
			vnc = null;
			vncConnected = false;
		}
	});

	// Bind the iframe src when running, without tracking auth.token — PB's
	// authStore syncs across tabs via the `storage` event, so a reactive read
	// would rewrite src on every cross-tab token rotation and KioskFrame's
	// {#key src} would tear down the noVNC session.
	$effect(() => {
		if (!detail || !isRunning) {
			kioskSrc = '';
			return;
		}
		kioskSrc = untrack(
			() =>
				`${apiBaseURL()}/api/admin/containers/${encodeURIComponent(name)}/kiosk/?token=${encodeURIComponent(auth.token ?? '')}`
		);
	});

	onMount(async () => {
		await loadDetail();
		startPolling();
	});

	onDestroy(() => {
		stopPolling();
		vnc?.disconnect();
		vnc = null;
	});
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center gap-2 card preset-tonal p-3">
		<span class={statusBadgeClass(status)}>{status}</span>
		{#if detail?.info}
			<span class="text-xs text-surface-600-400">
				ports: kiosk {detail.info.ports.browser_web} · vnc {detail.info.ports.browser_vnc}
			</span>
		{/if}
		<div class="ms-auto flex flex-wrap gap-2">
			<button
				type="button"
				class="preset-filled-primary btn btn-sm"
				disabled={loading || busyAction !== null || isRunning}
				onclick={handleStart}
			>
				{#if busyAction === 'start'}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<PlayIcon class="size-4" />
				{/if}
				<span>Start</span>
			</button>
			<button
				type="button"
				class="btn preset-tonal-error btn-sm"
				disabled={loading || busyAction !== null || !isRunning}
				onclick={handleStop}
			>
				{#if busyAction === 'stop'}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<SquareIcon class="size-4" />
				{/if}
				<span>Stop</span>
			</button>
			<button
				type="button"
				class="btn preset-tonal btn-sm"
				disabled={loading || busyAction !== null || !isRunning}
				onclick={handleRestart}
			>
				{#if busyAction === 'restart'}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<RotateCwIcon class="size-4" />
				{/if}
				<span>Restart</span>
			</button>
		</div>
	</div>

	{#if !detail && !loading}
		<div class="card preset-tonal p-3 text-sm">
			No container record for <code>{name}</code>. Create one on
			<a class="anchor" href={resolve('/pod/')}>/pod/</a>.
		</div>
	{:else}
		<div class="flex flex-col gap-4 lg:grid lg:grid-cols-[2fr_1fr] lg:gap-4">
			<KioskFrame
				{name}
				src={kioskSrc}
				running={isRunning}
				{loading}
				{vncConnected}
				externalHref={detail ? kioskURL() : undefined}
				onSnapshot={onSnapshotSlot}
				onReset={onResetVM}
			/>

			<div class="flex min-h-0 flex-col gap-2 overflow-hidden card preset-tonal p-3">
				<header class="flex items-center justify-between text-xs">
					<span class="font-medium">Xbox controller</span>
					<span class={vncConnected ? 'text-success-500' : 'text-surface-600-400'}>
						{vncConnected ? 'VNC connected' : isRunning ? 'connecting…' : 'offline'}
					</span>
				</header>
				<XboxController
					disabled={!vncConnected}
					onPress={(sym) => vnc?.sendKey(sym, true)}
					onRelease={(sym) => vnc?.sendKey(sym, false)}
				/>
			</div>
		</div>
	{/if}
</div>
