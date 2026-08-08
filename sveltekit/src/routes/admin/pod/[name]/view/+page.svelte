<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { ArrowLeftIcon, LoaderIcon, PlayIcon, RotateCwIcon, SquareIcon } from '@lucide/svelte';
	import { resolve } from '$app/paths';
	import { adminGet, adminPost, AdminFetchError } from '$lib/utils/admin-api';
	import { apiBaseURL, wsBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, toastPromise, toaster } from '$lib/stores/toaster';
	import KioskFrame from '$lib/components/kiosk/KioskFrame.svelte';
	import XboxController from '$lib/components/kiosk/XboxController.svelte';
	import { VNCKeyboard } from '$lib/utils/vnc-keyboard';
	import type { ContainerDetail, ContainerStatus } from '$lib/types/containers';

	let { data } = $props();
	const name = $derived(data.name);

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

	// --- Live scraper-read diagnostics panel -------------------------------------
	type Cursor = { index: number; count: number; valid: boolean };
	type Diagnostics = {
		instance: string;
		present: boolean;
		tick: number;
		screen: string;
		dela: string;
		menu_item: number;
		menu_item_name: string;
		game_connection: number;
		pregame_sentinel: boolean;
		menu_focus: number;
		map_cursor: Cursor;
		gametype_cursor: Cursor;
		map: string;
		gametype: string;
		selected_map: string;
		selected_gametype: string;
		highlighted_map: string;
		highlighted_gametype: string;
		enumerated_maps: string[] | null;
		enumerated_gametypes: string[] | null;
	};
	let diag = $state<Diagnostics | null>(null);
	let diagError = $state<string | null>(null);
	let diagTimer: ReturnType<typeof setInterval> | null = null;

	const CONN_NAMES = ['menu', 'system-link', 'hosting', 'film'];
	function connName(c: number): string {
		return `${c}${CONN_NAMES[c] ? ` (${CONN_NAMES[c]})` : ''}`;
	}

	async function loadDiag() {
		if (!isRunning) {
			diag = null;
			return;
		}
		try {
			diag = await adminGet<Diagnostics>(`scraper/${encodeURIComponent(name)}/diagnostics`);
			diagError = null;
		} catch (err) {
			diag = null;
			// 503 = host-runner subsystem off; 404 = no runner attached. Show a hint,
			// don't spam the console.
			diagError = err instanceof AdminFetchError ? err.message : 'diagnostics unavailable';
		}
	}

	function startDiagPolling() {
		stopDiagPolling();
		diagTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			loadDiag();
		}, 1000);
	}
	function stopDiagPolling() {
		if (diagTimer !== null) {
			clearInterval(diagTimer);
			diagTimer = null;
		}
	}

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
		const ok = await confirmToast({
			type: 'error',
			title: 'Stop pod?',
			description: `${name} — this will end the current session.`,
			confirmLabel: 'Stop',
			cancelLabel: 'Keep running'
		});
		if (!ok) return;
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

	async function handleVMReset() {
		if (!vnc) return;
		const ok = await confirmToast({
			type: 'warning',
			title: 'Reset to dashboard?',
			description: `${name} — sends Ctrl+R to xemu; current play session will be lost.`,
			confirmLabel: 'Reset'
		});
		if (!ok) return;
		vnc.sendChord(['Control_L', 'r']);
		toaster.info({ title: 'Reset', description: 'Sent Ctrl+R to xemu' });
	}

	// No /restart endpoint on the backend — emulate with stop + start.
	async function handleRestart() {
		const ok = await confirmToast({
			type: 'warning',
			title: 'Restart pod?',
			description: `${name} — current session will be lost.`,
			confirmLabel: 'Restart'
		});
		if (!ok) return;
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
		kioskSrc = untrack(() => kioskURL());
	});

	onMount(async () => {
		await loadDetail();
		startPolling();
		loadDiag();
		startDiagPolling();
	});

	onDestroy(() => {
		stopPolling();
		stopDiagPolling();
		vnc?.disconnect();
		vnc = null;
	});
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-3">
	{#snippet restartButton(extraClass: string)}
		<button
			type="button"
			class="btn preset-tonal {extraClass}"
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
	{/snippet}
	{#snippet toggleButton(extraClass: string)}
		{#if isRunning}
			<button
				type="button"
				class="btn preset-tonal-error {extraClass}"
				disabled={loading || busyAction !== null}
				onclick={handleStop}
			>
				{#if busyAction === 'stop'}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<SquareIcon class="size-4" />
				{/if}
				<span>Stop</span>
			</button>
		{:else}
			<button
				type="button"
				class="preset-filled-primary btn {extraClass}"
				disabled={loading || busyAction !== null}
				onclick={handleStart}
			>
				{#if busyAction === 'start'}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<PlayIcon class="size-4" />
				{/if}
				<span>Start</span>
			</button>
		{/if}
	{/snippet}

	<header class="flex flex-wrap items-center gap-2">
		<a
			href={resolve('/admin/pod/[name]', { name })}
			class="btn-icon preset-tonal"
			aria-label="Back to pod"
		>
			<ArrowLeftIcon class="size-4" />
		</a>
		<h1 class="h3 lg:h2">{name}</h1>
		<span class={statusBadgeClass(status)}>{status}</span>
		<div class="ms-auto flex flex-wrap gap-2">
			{@render restartButton('btn-sm')}
			{@render toggleButton('btn-sm')}
		</div>
	</header>

	{#if !detail && !loading}
		<div class="card preset-tonal p-3 text-sm">
			No container record for <code>{name}</code>. Create one on
			<a class="anchor" href={resolve('/admin/pod/')}>/admin/pod/</a>.
		</div>
	{:else}
		<div class="flex flex-col gap-3 lg:grid lg:grid-cols-[2fr_1fr] lg:gap-4">
			<div class="sticky top-0 z-10 lg:static lg:z-auto">
				<KioskFrame
					{name}
					src={kioskSrc}
					running={isRunning}
					{loading}
					{vncConnected}
					externalHref={detail ? kioskURL() : undefined}
				/>
			</div>

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
					onReset={handleVMReset}
				/>
			</div>
		</div>

		<!-- Live scraper-read diagnostics: watch the box AND what the scraper sees. -->
		<section class="card preset-tonal flex flex-col gap-2 p-3">
			<header class="flex flex-wrap items-center gap-2 text-xs">
				<span class="font-medium">Live scraper reads</span>
				{#if diag}
					<span class="badge preset-tonal-surface">tick {diag.tick}</span>
				{/if}
				<span class="ms-auto text-surface-600-400">polling 1s</span>
			</header>

			{#if !isRunning}
				<p class="text-xs text-surface-600-400">Box not running.</p>
			{:else if !diag}
				<p class="text-xs text-warning-500">{diagError ?? 'awaiting scraper…'}</p>
			{:else}
				<div class="grid grid-cols-2 gap-3 text-xs sm:grid-cols-3 lg:grid-cols-4">
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">screen</span>
						<span class="badge preset-filled-primary-500 w-fit">{diag.screen || 'unknown'}</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">menu_item</span>
						<span class="font-mono">
							{diag.menu_item_name}
							<span class="text-surface-600-400">({diag.menu_item})</span>
						</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">game_connection</span>
						<span class="font-mono">{connName(diag.game_connection)}</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">pregame sentinel</span>
						<span class="badge w-fit {diag.pregame_sentinel ? 'preset-tonal-warning' : 'preset-tonal-surface'}">
							{diag.pregame_sentinel ? '0xDEADBEEF' : 'absent'}
						</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">menu_focus (woken?)</span>
						<span class="flex items-center gap-1 font-mono">
							0x{(diag.menu_focus >>> 0).toString(16)}
							<span class="badge {diag.menu_focus ? 'preset-tonal-success' : 'preset-tonal-error'}">
								{diag.menu_focus ? 'woken' : 'cold/0'}
							</span>
						</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">map cursor (selecting)</span>
						<span class="flex flex-wrap items-center gap-1 font-mono">
							{diag.highlighted_map || '—'}
							<span class="text-surface-600-400">@{diag.map_cursor.index}/{diag.map_cursor.count}</span>
							<span class="badge {diag.map_cursor.valid ? 'preset-tonal-success' : 'preset-tonal-error'}">
								{diag.map_cursor.valid ? 'valid' : 'invalid'}
							</span>
						</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">gametype cursor (selecting)</span>
						<span class="flex flex-wrap items-center gap-1 font-mono">
							{diag.highlighted_gametype || '—'}
							<span class="text-surface-600-400">@{diag.gametype_cursor.index}/{diag.gametype_cursor.count}</span>
							<span class="badge {diag.gametype_cursor.valid ? 'preset-tonal-success' : 'preset-tonal-error'}">
								{diag.gametype_cursor.valid ? 'valid' : 'invalid'}
							</span>
						</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">map · pick → loaded</span>
						<span class="font-mono">
							{diag.selected_map || '—'}
							<span class="text-surface-600-400">→ loaded</span>
							{diag.map || '—'}
						</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-surface-600-400">gametype · pick → loaded</span>
						<span class="font-mono">
							{diag.selected_gametype || '—'}
							<span class="text-surface-600-400">→ loaded</span>
							{diag.gametype || '—'}
						</span>
					</div>
				</div>

				<div class="flex flex-col gap-0.5 text-xs">
					<span class="text-surface-600-400">dela · highlighted-widget path</span>
					<code class="bg-surface-200-800 break-all rounded p-1 font-mono">{diag.dela || '—'}</code>
				</div>

				<p class="text-xs text-surface-600-400">
					enumerated: {diag.enumerated_maps?.length ?? 0} maps · {diag.enumerated_gametypes
						?.length ?? 0} gametypes
				</p>
			{/if}
		</section>
	{/if}
</div>
