<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ArrowLeftIcon, PlayIcon, SquareIcon, Trash2Icon, RefreshCwIcon } from '@lucide/svelte';
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

	const name = $derived(page.params.name ?? '');

	let detail = $state<ContainerDetail | null>(null);
	let status = $state<RowStatus>('loading');
	let loading = $state(true);
	let confirmDelete = $state(false);

	let logsWhich = $state<LogsWhich>('xemu');
	let logsTail = $state(200);
	let logsText = $state('');
	let logsLoading = $state(false);

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

	// Button rows: each entry is [label, key from KEYSYM].
	const dpad = [
		['↑', 'Up'],
		['←', 'Left'],
		['↓', 'Down'],
		['→', 'Right']
	] as const;
	const face = [
		['Y', 'y'],
		['X', 'x'],
		['B', 'b'],
		['A', 'a']
	] as const;
	const sysButtons = [
		['Back', 'BackSpace'],
		['Start', 'Return']
	] as const;
	const shoulders = [
		['LB', '1'],
		['LT', 'w'],
		['RT', 'o'],
		['RB', '2']
	] as const;
	const sticks = [
		['L3', '3'],
		['R3', '4']
	] as const;
	const leftStick = [
		['L↑', 'e'],
		['L←', 's'],
		['L↓', 'd'],
		['L→', 'f']
	] as const;
	const rightStick = [
		['R↑', 'i'],
		['R←', 'j'],
		['R↓', 'k'],
		['R→', 'l']
	] as const;
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-4">
	<header class="flex flex-wrap items-center gap-3">
		<a href={resolve('/containers/')} class="btn-icon preset-tonal" aria-label="Back">
			<ArrowLeftIcon class="size-4" />
		</a>
		<h1 class="h2">{name}</h1>
		<span class={statusClass(status)}>{status}</span>
		<div class="ms-auto flex flex-wrap items-center gap-3">
			{#if detail?.scraper}
				<div class="text-sm">
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

	<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
		<!-- Kiosk iframe -->
		<div class="overflow-hidden card p-0 lg:col-span-2">
			<div class="flex items-center justify-between p-2 text-xs">
				<span class="text-surface-600-400">Kiosk view</span>
				{#if detail}
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={kioskURL()} target="_blank" rel="noopener" class="anchor"> open in new tab </a>
				{/if}
			</div>
			{#if detail && isRunning}
				<iframe
					src={kioskURL()}
					title="Kiosk view of {name}"
					class="h-[60vh] w-full border-0"
					allowfullscreen
				></iframe>
			{:else}
				<div class="flex h-[60vh] items-center justify-center text-sm text-surface-600-400">
					{loading ? 'Loading…' : 'Container not running. Press Start to launch the kiosk.'}
				</div>
			{/if}
		</div>

		<!-- Controls -->
		<div class="flex flex-col gap-4 card p-4">
			<div class="flex items-center justify-between">
				<h2 class="h4">Controls</h2>
				<span class="text-xs {vncConnected ? 'text-success-500' : 'text-surface-600-400'}">
					{vncConnected ? 'connected' : isRunning ? 'connecting…' : 'offline'}
				</span>
			</div>

			<div class="grid grid-cols-2 gap-3">
				<div>
					<div class="mb-1 text-xs text-surface-600-400">D-pad</div>
					<div class="grid grid-cols-3 gap-1">
						<div></div>
						{#each dpad.slice(0, 1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
						<div></div>
						{#each dpad.slice(1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
					</div>
				</div>

				<div>
					<div class="mb-1 text-xs text-surface-600-400">Face</div>
					<div class="grid grid-cols-3 gap-1">
						<div></div>
						{#each face.slice(0, 1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
						<div></div>
						{#each face.slice(1, 3) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
						<div></div>
						{#each face.slice(3) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
					</div>
				</div>
			</div>

			<div>
				<div class="mb-1 text-xs text-surface-600-400">Shoulders / Triggers</div>
				<div class="grid grid-cols-4 gap-1">
					{#each shoulders as [label, key] (key)}
						<button
							class="btn preset-tonal"
							class:preset-filled={pressed[key]}
							onpointerdown={(e) => handlePointerDown(e, key)}
							onpointerup={(e) => handlePointerUp(e, key)}
							onpointercancel={(e) => handlePointerUp(e, key)}
							onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
						>
					{/each}
				</div>
			</div>

			<div>
				<div class="mb-1 text-xs text-surface-600-400">System</div>
				<div class="grid grid-cols-2 gap-1">
					{#each sysButtons as [label, key] (key)}
						<button
							class="btn preset-tonal"
							class:preset-filled={pressed[key]}
							onpointerdown={(e) => handlePointerDown(e, key)}
							onpointerup={(e) => handlePointerUp(e, key)}
							onpointercancel={(e) => handlePointerUp(e, key)}
							onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
						>
					{/each}
				</div>
			</div>

			<div class="grid grid-cols-2 gap-3">
				<div>
					<div class="mb-1 text-xs text-surface-600-400">Left stick</div>
					<div class="grid grid-cols-3 gap-1">
						<div></div>
						{#each leftStick.slice(0, 1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal btn-sm"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
						<div></div>
						{#each leftStick.slice(1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal btn-sm"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
					</div>
				</div>
				<div>
					<div class="mb-1 text-xs text-surface-600-400">Right stick</div>
					<div class="grid grid-cols-3 gap-1">
						<div></div>
						{#each rightStick.slice(0, 1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal btn-sm"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
						<div></div>
						{#each rightStick.slice(1) as [label, key] (key)}
							<button
								class="btn aspect-square preset-tonal btn-sm"
								class:preset-filled={pressed[key]}
								onpointerdown={(e) => handlePointerDown(e, key)}
								onpointerup={(e) => handlePointerUp(e, key)}
								onpointercancel={(e) => handlePointerUp(e, key)}
								onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
							>
						{/each}
					</div>
				</div>
			</div>

			<div>
				<div class="mb-1 text-xs text-surface-600-400">Stick clicks</div>
				<div class="grid grid-cols-2 gap-1">
					{#each sticks as [label, key] (key)}
						<button
							class="btn preset-tonal btn-sm"
							class:preset-filled={pressed[key]}
							onpointerdown={(e) => handlePointerDown(e, key)}
							onpointerup={(e) => handlePointerUp(e, key)}
							onpointercancel={(e) => handlePointerUp(e, key)}
							onlostpointercapture={(e) => handlePointerUp(e, key)}>{label}</button
						>
					{/each}
				</div>
			</div>
		</div>
	</div>

	<!-- Logs -->
	<div class="flex flex-col gap-2 card p-4">
		<div class="flex items-center gap-2">
			<h2 class="h4">Logs</h2>
			<div class="ms-auto flex items-center gap-1">
				<button
					type="button"
					class="btn btn-sm {logsWhich === 'xemu' ? 'preset-filled' : 'preset-tonal'}"
					onclick={() => {
						logsWhich = 'xemu';
						loadLogs();
					}}
				>
					xemu
				</button>
				<button
					type="button"
					class="btn btn-sm {logsWhich === 'browser' ? 'preset-filled' : 'preset-tonal'}"
					onclick={() => {
						logsWhich = 'browser';
						loadLogs();
					}}
				>
					browser
				</button>
				<button
					type="button"
					class="btn-icon preset-tonal btn-sm"
					aria-label="Refresh logs"
					onclick={loadLogs}
					disabled={logsLoading}
				>
					<RefreshCwIcon class="size-4" />
				</button>
			</div>
		</div>
		<pre
			class="max-h-[40vh] min-h-[15vh] overflow-auto rounded bg-surface-200-800 p-2 font-mono text-xs whitespace-pre-wrap">{logsText ||
				(logsLoading ? 'Loading…' : '(no logs)')}</pre>
	</div>
</div>

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
