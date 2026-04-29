<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		ArrowLeftIcon,
		PlayIcon,
		SquareIcon,
		Trash2Icon,
		RefreshCwIcon,
		RotateCcwIcon,
		CameraIcon,
		ExternalLinkIcon,
		CopyIcon,
		CheckIcon
	} from '@lucide/svelte';
	import { Popover, Portal, SegmentedControl, Tabs } from '@skeletonlabs/skeleton-svelte';
	import { adminGet, adminPost, adminDelete, AdminFetchError } from '$lib/utils/admin-api';
	import { apiBaseURL, wsBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import { toaster } from '$lib/stores/toaster';
	import { VNCKeyboard, KEYSYM } from '$lib/utils/vnc-keyboard';
	import type {
		ContainerDetail,
		ContainerStatus,
		LogsResponse,
		LogsWhich
	} from '$lib/types/containers';

	type RowStatus = ContainerStatus | 'loading' | string;
	type ConfirmAction = { title: string; body: string; run: () => void };

	const name = $derived(page.params.name ?? '');

	let detail = $state<ContainerDetail | null>(null);
	let status = $state<RowStatus>('loading');
	let loading = $state(true);
	let confirmDelete = $state(false);
	let confirmAction = $state<ConfirmAction | null>(null);

	let logsWhich = $state<LogsWhich>('xemu');
	let logsTail = $state(200);
	let logsText = $state('');
	let logsLoading = $state(false);
	let logsCopied = $state(false);

	let vnc: VNCKeyboard | null = null;
	let vncConnected = $state(false);
	let pressed = $state<Record<string, boolean>>({});

	let detailTimer: ReturnType<typeof setInterval> | null = null;
	let logsTimer: ReturnType<typeof setInterval> | null = null;

	function describeError(err: unknown): string {
		if (err instanceof AdminFetchError) return err.message;
		if (err instanceof Error) return err.message;
		return String(err);
	}

	async function loadDetail() {
		try {
			const d = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
			detail = d;
			status = d.status;
		} catch (err) {
			if (err instanceof AdminFetchError && err.status === 404) {
				toaster.error({ title: 'Not found', description: name });
				goto(resolve('/containers/'));
				return;
			}
			console.warn('detail fetch failed', err);
			status = 'unknown';
		} finally {
			loading = false;
		}
	}

	async function copyLogs() {
		if (!logsText) return;
		try {
			await navigator.clipboard.writeText(logsText);
			logsCopied = true;
			setTimeout(() => (logsCopied = false), 1500);
		} catch (err) {
			toaster.error({ title: 'Copy failed', description: describeError(err) });
		}
	}

	async function loadLogs() {
		try {
			logsLoading = true;
			const res = await adminGet<LogsResponse>(
				`containers/${encodeURIComponent(name)}/logs?which=${logsWhich}&tail=${logsTail}`
			);
			logsText = res.logs;
		} catch (err) {
			console.warn('logs fetch failed', err);
		} finally {
			logsLoading = false;
		}
	}

	function startPolling() {
		stopPolling();
		detailTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			loadDetail();
		}, 3000);
		logsTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			loadLogs();
		}, 5000);
	}

	function stopPolling() {
		if (detailTimer !== null) {
			clearInterval(detailTimer);
			detailTimer = null;
		}
		if (logsTimer !== null) {
			clearInterval(logsTimer);
			logsTimer = null;
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

	function handlePointerDown(e: PointerEvent, key: string) {
		const sym = KEYSYM[key];
		if (sym == null) return;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		pressed = { ...pressed, [key]: true };
		vnc?.sendKey(sym, true);
	}

	function handlePointerUp(e: PointerEvent, key: string) {
		const sym = KEYSYM[key];
		if (sym == null) return;
		const target = e.currentTarget as HTMLElement;
		if (target.hasPointerCapture(e.pointerId)) target.releasePointerCapture(e.pointerId);
		if (!pressed[key]) return;
		pressed = { ...pressed, [key]: false };
		vnc?.sendKey(sym, false);
	}

	// Tap-and-release for one-shot keys (Reset, snapshot slots).
	// TODO: wire QMP system_reset / savevm / loadvm in internal/xemu/qmp.go
	//       for slot-aware snapshots beyond xemu's F-key hotkeys.
	function sendVNCTap(key: string) {
		const sym = KEYSYM[key];
		if (sym == null || !vnc) return;
		vnc.sendKey(sym, true);
		setTimeout(() => vnc?.sendKey(sym, false), 60);
	}

	// Press all keys down in order, then release in reverse — for modifier chords
	// like Ctrl+R (xemu's reset).
	function sendVNCChord(keys: string[]) {
		if (!vnc) return;
		const syms: number[] = [];
		for (const k of keys) {
			const sym = KEYSYM[k];
			if (sym == null) return;
			syms.push(sym);
		}
		for (const sym of syms) vnc.sendKey(sym, true);
		setTimeout(() => {
			for (let i = syms.length - 1; i >= 0; i--) vnc?.sendKey(syms[i], false);
		}, 60);
	}

	function askConfirm(title: string, body: string, run: () => void) {
		confirmAction = { title, body, run };
	}

	function runConfirmed() {
		const a = confirmAction;
		confirmAction = null;
		a?.run();
	}

	async function handleStart() {
		status = 'loading';
		try {
			await adminPost(`containers/${encodeURIComponent(name)}/start`);
			toaster.success({ title: 'Starting', description: name });
			await loadDetail();
		} catch (err) {
			toaster.error({ title: 'Start failed', description: describeError(err) });
			await loadDetail();
		}
	}

	async function handleStop() {
		status = 'loading';
		try {
			await adminPost(`containers/${encodeURIComponent(name)}/stop`);
			toaster.success({ title: 'Stopping', description: name });
			await loadDetail();
		} catch (err) {
			toaster.error({ title: 'Stop failed', description: describeError(err) });
			await loadDetail();
		}
	}

	async function handleDelete() {
		try {
			await adminDelete(`containers/${encodeURIComponent(name)}`);
			toaster.success({ title: 'Deleted', description: name });
			confirmDelete = false;
			goto(resolve('/containers/'));
		} catch (err) {
			toaster.error({ title: 'Delete failed', description: describeError(err) });
		}
	}

	function statusClass(s: RowStatus): string {
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

	const isRunning = $derived(status === 'running');

	// Reconnect VNC when status flips to running.
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

	onMount(async () => {
		await loadDetail();
		await loadLogs();
		startPolling();
	});

	onDestroy(() => {
		stopPolling();
		vnc?.disconnect();
		vnc = null;
	});

	// Reset sends Ctrl+R (xemu's hotkey for system reset); S1-S4 tap F5-F8 to
	// invoke xemu's quick save/load shortcuts. The exact in-game effect depends
	// on whether xemu's current build binds those keys — see TODO at sendVNCTap
	// for QMP-backed alternatives.
	const resetAction = { label: 'Reset', body: 'Reset the running VM?' } as const;
	const snapshotActions = [
		{ label: 'S1', key: 'F5', body: 'Trigger snapshot slot 1 (F5)?' },
		{ label: 'S2', key: 'F6', body: 'Trigger snapshot slot 2 (F6)?' },
		{ label: 'S3', key: 'F7', body: 'Trigger snapshot slot 3 (F7)?' },
		{ label: 'S4', key: 'F8', body: 'Trigger snapshot slot 4 (F8)?' }
	] as const;

	type ButtonColor = 'red' | 'green' | 'blue' | 'yellow' | 'gray';
	// Tailwind JIT only generates classes it sees as literal strings, so each
	// (color, pressed) combination must be spelled out — no template literals.
	const BUTTON_CLASSES: Record<ButtonColor, { up: string; down: string }> = {
		red: {
			up: 'bg-red-900 border-2 border-red-500',
			down: 'bg-red-500 border-2 border-red-500'
		},
		green: {
			up: 'bg-green-900 border-2 border-green-500',
			down: 'bg-green-500 border-2 border-green-500'
		},
		blue: {
			up: 'bg-blue-900 border-2 border-blue-500',
			down: 'bg-blue-500 border-2 border-blue-500'
		},
		yellow: {
			up: 'bg-yellow-900 border-2 border-yellow-500',
			down: 'bg-yellow-500 border-2 border-yellow-500'
		},
		gray: {
			up: 'bg-gray-900 border-2 border-gray-500',
			down: 'bg-gray-500 border-2 border-gray-500'
		}
	};
	function buttonClass(key: string, color: ButtonColor): string {
		const c = BUTTON_CLASSES[color];
		return pressed[key] ? c.down : c.up;
	}
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-3 overflow-hidden p-3">
	<header class="flex flex-none flex-wrap items-center gap-2">
		<a href={resolve('/containers/')} class="btn-icon preset-tonal" aria-label="Back">
			<ArrowLeftIcon class="size-4" />
		</a>
		<h1 class="h3 lg:h2">{name}</h1>
		<span class={statusClass(status)}>{status}</span>
		<div class="ms-auto flex flex-wrap items-center gap-3">
			{#if detail?.scraper}
				<div class="hidden text-sm lg:block">
					<div class="font-medium">{detail.scraper.game_title || '—'}</div>
					<div class="text-xs text-surface-600-400">
						{detail.scraper.xbox_name || 'xbox name unknown'}
					</div>
				</div>
			{/if}
			<div class="inline-flex gap-1">
				{#if isRunning}
					<button
						type="button"
						class="btn-icon preset-tonal-warning"
						aria-label="Stop"
						title="Stop"
						onclick={handleStop}
					>
						<SquareIcon class="size-4" />
					</button>
				{:else}
					<button
						type="button"
						class="btn-icon preset-tonal-success"
						aria-label="Start"
						title="Start"
						onclick={handleStart}
					>
						<PlayIcon class="size-4" />
					</button>
				{/if}
				<button
					type="button"
					class="btn-icon preset-tonal-error"
					aria-label="Delete"
					title="Delete"
					onclick={() => (confirmDelete = true)}
				>
					<Trash2Icon class="size-4" />
				</button>
			</div>
		</div>
	</header>

	<div class="grid flex-1 grid-cols-1 gap-3 overflow-hidden xl:grid-cols-2 2xl:grid-cols-3">
		<!-- Kiosk iframe -->
		<div class="flex flex-col overflow-hidden card p-0 2xl:col-span-2">
			<div class="flex flex-none items-center justify-around gap-2 p-2 text-xs">
				<span class="text-surface-600-400"
					>Kiosk view
					<span class="ms-1 text-xs {vncConnected ? 'text-success-500' : 'text-surface-600-400'}">
						{vncConnected ? '●' : isRunning ? '…' : '○'}
					</span>
				</span>
				{#if detail}
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href={kioskURL()}
						target="_blank"
						rel="noopener"
						class="btn-icon btn-icon-sm preset-tonal"
						aria-label="Open in new tab"
						title="Open in new tab"
					>
						<ExternalLinkIcon class="size-4" />
					</a>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				{/if}
				<span class="flex"> </span>
				<div class="ms-auto flex items-center gap-1">
					<Popover>
						<Popover.Trigger
							class="btn preset-tonal btn-sm"
							disabled={!vncConnected}
							aria-label="Snapshots"
						>
							<CameraIcon class="size-4" />
							<span>Snapshots</span>
						</Popover.Trigger>
						<Portal>
							<Popover.Positioner>
								<Popover.Content class="flex gap-1 card bg-surface-100-900 p-2">
									{#each snapshotActions as { label, key, body } (key)}
										<Popover.CloseTrigger
											class="btn preset-tonal btn-sm"
											onclick={() => askConfirm(label, body, () => sendVNCTap(key))}
										>
											{label}
										</Popover.CloseTrigger>
									{/each}
								</Popover.Content>
							</Popover.Positioner>
						</Portal>
					</Popover>
					<button
						type="button"
						class="btn-icon btn-icon-sm preset-tonal-warning"
						aria-label="Reset"
						title="Reset"
						disabled={!vncConnected}
						onclick={() =>
							askConfirm(resetAction.label, resetAction.body, () =>
								sendVNCChord(['Control_L', 'r'])
							)}
					>
						<RotateCcwIcon class="size-4" />
					</button>
				</div>
			</div>
			{#if detail && isRunning}
				<iframe
					src={kioskURL()}
					title="Kiosk view of {name}"
					class="aspect-4/3 w-full border-0"
					allowfullscreen
				></iframe>
			{:else}
				<div class="flex flex-1 items-center justify-center text-sm text-surface-600-400">
					{loading ? 'Loading…' : 'Container not running. Press Start to launch the kiosk.'}
				</div>
			{/if}
		</div>

		<!-- Controls + Logs -->
		<div class="flex min-h-0 flex-col gap-2 overflow-hidden card p-3">
			<Tabs defaultValue="controller">
				<Tabs.List>
					<Tabs.Trigger value="controller" class="flex-1">Controller</Tabs.Trigger>
					<Tabs.Trigger value="logs" class="flex-1">Logs</Tabs.Trigger>
					<Tabs.Indicator />
				</Tabs.List>
				<Tabs.Content
					value="controller"
					class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto select-none"
				>
					<!-- Top bar: LB LT | Back Start | RT RB -->
					<div class="flex items-center justify-between gap-2">
						<div class="flex gap-1">
							<button
								class="btn btn-sm {buttonClass('1', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, '1')}
								onpointerup={(e) => handlePointerUp(e, '1')}
								onpointercancel={(e) => handlePointerUp(e, '1')}
								onlostpointercapture={(e) => handlePointerUp(e, '1')}>LB</button
							>
							<button
								class="btn btn-sm {buttonClass('w', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'w')}
								onpointerup={(e) => handlePointerUp(e, 'w')}
								onpointercancel={(e) => handlePointerUp(e, 'w')}
								onlostpointercapture={(e) => handlePointerUp(e, 'w')}>LT</button
							>
						</div>
						<div class="flex gap-1">
							<button
								class="btn btn-sm {buttonClass('BackSpace', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'BackSpace')}
								onpointerup={(e) => handlePointerUp(e, 'BackSpace')}
								onpointercancel={(e) => handlePointerUp(e, 'BackSpace')}
								onlostpointercapture={(e) => handlePointerUp(e, 'BackSpace')}>Back</button
							>
							<button
								class="btn btn-sm {buttonClass('Return', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'Return')}
								onpointerup={(e) => handlePointerUp(e, 'Return')}
								onpointercancel={(e) => handlePointerUp(e, 'Return')}
								onlostpointercapture={(e) => handlePointerUp(e, 'Return')}>Start</button
							>
						</div>
						<div class="flex gap-1">
							<button
								class="btn btn-sm {buttonClass('o', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'o')}
								onpointerup={(e) => handlePointerUp(e, 'o')}
								onpointercancel={(e) => handlePointerUp(e, 'o')}
								onlostpointercapture={(e) => handlePointerUp(e, 'o')}>RT</button
							>
							<button
								class="btn btn-sm {buttonClass('2', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, '2')}
								onpointerup={(e) => handlePointerUp(e, '2')}
								onpointercancel={(e) => handlePointerUp(e, '2')}
								onlostpointercapture={(e) => handlePointerUp(e, '2')}>RB</button
							>
						</div>
					</div>

					<!-- Middle: D-pad ⇋ Face diamond -->
					<div class="flex items-center justify-between gap-2">
						<!-- D-pad -->
						<div class="grid grid-cols-3 grid-rows-3 gap-1">
							<div></div>
							<button
								class="btn aspect-square {buttonClass('Up', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'Up')}
								onpointerup={(e) => handlePointerUp(e, 'Up')}
								onpointercancel={(e) => handlePointerUp(e, 'Up')}
								onlostpointercapture={(e) => handlePointerUp(e, 'Up')}>↑</button
							>
							<div></div>
							<button
								class="btn aspect-square {buttonClass('Left', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'Left')}
								onpointerup={(e) => handlePointerUp(e, 'Left')}
								onpointercancel={(e) => handlePointerUp(e, 'Left')}
								onlostpointercapture={(e) => handlePointerUp(e, 'Left')}>←</button
							>
							<div></div>
							<button
								class="btn aspect-square {buttonClass('Right', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'Right')}
								onpointerup={(e) => handlePointerUp(e, 'Right')}
								onpointercancel={(e) => handlePointerUp(e, 'Right')}
								onlostpointercapture={(e) => handlePointerUp(e, 'Right')}>→</button
							>
							<div></div>
							<button
								class="btn aspect-square {buttonClass('Down', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'Down')}
								onpointerup={(e) => handlePointerUp(e, 'Down')}
								onpointercancel={(e) => handlePointerUp(e, 'Down')}
								onlostpointercapture={(e) => handlePointerUp(e, 'Down')}>↓</button
							>
							<div></div>
						</div>

						<!-- Face diamond (Y top, X left, B right, A bottom) -->
						<div class="grid grid-cols-3 grid-rows-3 gap-1">
							<div></div>
							<button
								class="btn aspect-square rounded-full {buttonClass('y', 'yellow')}"
								onpointerdown={(e) => handlePointerDown(e, 'y')}
								onpointerup={(e) => handlePointerUp(e, 'y')}
								onpointercancel={(e) => handlePointerUp(e, 'y')}
								onlostpointercapture={(e) => handlePointerUp(e, 'y')}>Y</button
							>
							<div></div>
							<button
								class="btn aspect-square rounded-full {buttonClass('x', 'blue')}"
								onpointerdown={(e) => handlePointerDown(e, 'x')}
								onpointerup={(e) => handlePointerUp(e, 'x')}
								onpointercancel={(e) => handlePointerUp(e, 'x')}
								onlostpointercapture={(e) => handlePointerUp(e, 'x')}>X</button
							>
							<div></div>
							<button
								class="btn aspect-square rounded-full {buttonClass('b', 'red')}"
								onpointerdown={(e) => handlePointerDown(e, 'b')}
								onpointerup={(e) => handlePointerUp(e, 'b')}
								onpointercancel={(e) => handlePointerUp(e, 'b')}
								onlostpointercapture={(e) => handlePointerUp(e, 'b')}>B</button
							>
							<div></div>
							<button
								class="btn aspect-square rounded-full {buttonClass('a', 'green')}"
								onpointerdown={(e) => handlePointerDown(e, 'a')}
								onpointerup={(e) => handlePointerUp(e, 'a')}
								onpointercancel={(e) => handlePointerUp(e, 'a')}
								onlostpointercapture={(e) => handlePointerUp(e, 'a')}>A</button
							>
							<div></div>
						</div>
					</div>

					<!-- Bottom: Left stick + Right stick (L3 / R3 in centers) -->
					<div class="flex items-center justify-between gap-2">
						<div class="grid grid-cols-3 grid-rows-3 gap-1">
							<div></div>
							<button
								class="btn aspect-square btn-sm {buttonClass('e', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'e')}
								onpointerup={(e) => handlePointerUp(e, 'e')}
								onpointercancel={(e) => handlePointerUp(e, 'e')}
								onlostpointercapture={(e) => handlePointerUp(e, 'e')}>L↑</button
							>
							<div></div>
							<button
								class="btn aspect-square btn-sm {buttonClass('s', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 's')}
								onpointerup={(e) => handlePointerUp(e, 's')}
								onpointercancel={(e) => handlePointerUp(e, 's')}
								onlostpointercapture={(e) => handlePointerUp(e, 's')}>L←</button
							>
							<button
								class="btn aspect-square btn-sm text-[10px] {buttonClass('3', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, '3')}
								onpointerup={(e) => handlePointerUp(e, '3')}
								onpointercancel={(e) => handlePointerUp(e, '3')}
								onlostpointercapture={(e) => handlePointerUp(e, '3')}>L3</button
							>
							<button
								class="btn aspect-square btn-sm {buttonClass('f', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'f')}
								onpointerup={(e) => handlePointerUp(e, 'f')}
								onpointercancel={(e) => handlePointerUp(e, 'f')}
								onlostpointercapture={(e) => handlePointerUp(e, 'f')}>L→</button
							>
							<div></div>
							<button
								class="btn aspect-square btn-sm {buttonClass('d', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'd')}
								onpointerup={(e) => handlePointerUp(e, 'd')}
								onpointercancel={(e) => handlePointerUp(e, 'd')}
								onlostpointercapture={(e) => handlePointerUp(e, 'd')}>L↓</button
							>
							<div></div>
						</div>

						<div class="grid grid-cols-3 grid-rows-3 gap-1">
							<div></div>
							<button
								class="btn aspect-square btn-sm {buttonClass('i', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'i')}
								onpointerup={(e) => handlePointerUp(e, 'i')}
								onpointercancel={(e) => handlePointerUp(e, 'i')}
								onlostpointercapture={(e) => handlePointerUp(e, 'i')}>R↑</button
							>
							<div></div>
							<button
								class="btn aspect-square btn-sm {buttonClass('j', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'j')}
								onpointerup={(e) => handlePointerUp(e, 'j')}
								onpointercancel={(e) => handlePointerUp(e, 'j')}
								onlostpointercapture={(e) => handlePointerUp(e, 'j')}>R←</button
							>
							<button
								class="btn aspect-square btn-sm text-[10px] {buttonClass('4', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, '4')}
								onpointerup={(e) => handlePointerUp(e, '4')}
								onpointercancel={(e) => handlePointerUp(e, '4')}
								onlostpointercapture={(e) => handlePointerUp(e, '4')}>R3</button
							>
							<button
								class="btn aspect-square btn-sm {buttonClass('l', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'l')}
								onpointerup={(e) => handlePointerUp(e, 'l')}
								onpointercancel={(e) => handlePointerUp(e, 'l')}
								onlostpointercapture={(e) => handlePointerUp(e, 'l')}>R→</button
							>
							<div></div>
							<button
								class="btn aspect-square btn-sm {buttonClass('k', 'gray')}"
								onpointerdown={(e) => handlePointerDown(e, 'k')}
								onpointerup={(e) => handlePointerUp(e, 'k')}
								onpointercancel={(e) => handlePointerUp(e, 'k')}
								onlostpointercapture={(e) => handlePointerUp(e, 'k')}>R↓</button
							>
							<div></div>
						</div>
					</div>
				</Tabs.Content>

				<Tabs.Content value="logs" class="flex min-h-0 flex-1 flex-col gap-2">
					<div class="flex flex-row gap-5 md:flex-col">
						<SegmentedControl
							class="flex flex-3 flex-row gap-1"
							value={logsWhich}
							onValueChange={(d) => {
								logsWhich = d.value as LogsWhich;
								loadLogs();
							}}
						>
							<SegmentedControl.Indicator />
							<SegmentedControl.Item value="xemu">
								<SegmentedControl.ItemText>xemu</SegmentedControl.ItemText>
								<SegmentedControl.ItemHiddenInput />
							</SegmentedControl.Item>
							<SegmentedControl.Item value="browser">
								<SegmentedControl.ItemText>browser</SegmentedControl.ItemText>
								<SegmentedControl.ItemHiddenInput />
							</SegmentedControl.Item>
						</SegmentedControl>

						<div class="flex flex-row gap-1">
							<button
								type="button"
								class="btn-md ms-auto btn-icon preset-tonal"
								aria-label={logsCopied ? 'Copied' : 'Copy logs'}
								title={logsCopied ? 'Copied' : 'Copy logs'}
								onclick={copyLogs}
								disabled={!logsText}
							>
								{#if logsCopied}
									<CheckIcon class="size-4 text-success-500" />
								{:else}
									<CopyIcon class="size-4" />
								{/if}
							</button>
							<button
								type="button"
								class="btn-md btn-icon preset-tonal"
								aria-label="Refresh logs"
								title="Refresh logs"
								onclick={loadLogs}
								disabled={logsLoading}
							>
								<RefreshCwIcon class="size-4" />
							</button>
						</div>
					</div>
					<pre
						class="min-h-0 flex-1 overflow-auto rounded bg-surface-200-800 p-2 font-mono text-xs whitespace-pre-wrap">{logsText ||
							(logsLoading ? 'Loading…' : '(no logs)')}</pre>
				</Tabs.Content>
			</Tabs>
		</div>
	</div>
</div>

{#if confirmAction}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget) confirmAction = null;
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') confirmAction = null;
		}}
	>
		<div class="w-full max-w-md card p-6">
			<h2 class="mb-2 h3">{confirmAction.title}</h2>
			<p class="mb-4 text-sm text-surface-600-400">{confirmAction.body}</p>
			<div class="flex justify-end gap-2">
				<button type="button" class="btn preset-tonal" onclick={() => (confirmAction = null)}>
					Cancel
				</button>
				<button type="button" class="btn preset-filled" onclick={runConfirmed}> Confirm </button>
			</div>
		</div>
	</div>
{/if}

{#if confirmDelete}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
		role="dialog"
		aria-modal="true"
		tabindex="-1"
		onclick={(e) => {
			if (e.target === e.currentTarget) confirmDelete = false;
		}}
		onkeydown={(e) => {
			if (e.key === 'Escape') confirmDelete = false;
		}}
	>
		<div class="w-full max-w-md card p-6">
			<h2 class="mb-2 h3">Delete container</h2>
			<p class="mb-4 text-sm text-surface-600-400">
				Permanently remove <strong>{name}</strong>?
				{#if isRunning}
					<span class="mt-2 block text-error-500">
						This container is currently running. It will be force-stopped before deletion.
					</span>
				{/if}
			</p>
			<div class="flex justify-end gap-2">
				<button type="button" class="btn preset-tonal" onclick={() => (confirmDelete = false)}>
					Cancel
				</button>
				<button type="button" class="btn preset-filled-error-500" onclick={handleDelete}>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}
