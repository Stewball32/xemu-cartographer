<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import {
		ArrowLeftIcon,
		PlayIcon,
		SquareIcon,
		Trash2Icon,
		EraserIcon,
		RefreshCwIcon,
		CopyIcon,
		CheckIcon,
		LoaderIcon
	} from '@lucide/svelte';
	import { SegmentedControl, Tabs } from '@skeletonlabs/skeleton-svelte';
	import { adminGet, adminPost, adminDelete, AdminFetchError } from '$lib/utils/admin-api';
	import { apiBaseURL, wsBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import { toaster, toastPromise, confirmToast } from '$lib/stores/toaster';
	import KioskFrame, { type SnapshotSlot } from '$lib/components/kiosk/KioskFrame.svelte';
	import XboxController from '$lib/components/kiosk/XboxController.svelte';
	import { VNCKeyboard, KEYSYM } from '$lib/utils/vnc-keyboard';
	import type {
		ContainerDetail,
		ContainerStatus,
		LogsResponse,
		LogsWhich
	} from '$lib/types/containers';

	type RowStatus = ContainerStatus | 'loading' | string;
	type BusyAction = 'start' | 'stop' | 'delete' | 'delete-files';

	const name = $derived(page.params.name ?? '');

	let detail = $state<ContainerDetail | null>(null);
	let status = $state<RowStatus>('loading');
	let loading = $state(true);
	let busyAction = $state<BusyAction | null>(null);

	let logsWhich = $state<LogsWhich>('xemu');
	let logsTail = $state(200);
	let logsText = $state('');
	let logsLoading = $state(false);
	let logsCopied = $state(false);

	let vnc: VNCKeyboard | null = null;
	let vncConnected = $state(false);

	let kioskSrc = $state('');

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
				goto(resolve('/pod/'));
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

	async function handleDelete() {
		const runningWarning = isRunning ? ' It is currently running and will be force-stopped.' : '';
		const ok = await confirmToast({
			title: 'Delete container',
			description: `Permanently remove ${name}?${runningWarning}`,
			confirmLabel: 'Delete',
			type: isRunning ? 'error' : 'warning'
		});
		if (!ok) return;
		busyAction = 'delete';
		try {
			await toastPromise(adminDelete(`containers/${encodeURIComponent(name)}`), {
				loading: { title: 'Deleting', description: name },
				success: { title: 'Deleted', description: name },
				errorTitle: 'Delete failed'
			});
			goto(resolve('/pod/'));
		} catch {
			// toast already shown
		} finally {
			busyAction = null;
		}
	}

	async function handleDeleteFiles() {
		const ok = await confirmToast({
			title: 'Delete container files',
			description: `Wipe xemu config + Firefox profile for ${name}? The container record stays; next start recreates dirs.`,
			confirmLabel: 'Delete files'
		});
		if (!ok) return;
		busyAction = 'delete-files';
		try {
			await toastPromise(adminDelete(`containers/${encodeURIComponent(name)}/files`), {
				loading: { title: 'Deleting files', description: name },
				success: { title: 'Files deleted', description: name },
				errorTitle: 'Delete files failed'
			});
			await loadDetail();
		} catch {
			// toast already shown
		} finally {
			busyAction = null;
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
	async function onSnapshotSlot(slot: SnapshotSlot) {
		if (await confirmToast({ title: slot.label, description: slot.body, type: 'info' })) {
			sendVNCTap(slot.key);
		}
	}

	async function onResetVM() {
		if (await confirmToast({ title: 'Reset', description: 'Reset the running VM?' })) {
			sendVNCChord(['Control_L', 'r']);
		}
	}
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-3 overflow-hidden p-3">
	<header class="flex flex-none flex-wrap items-center gap-2">
		<a href={resolve('/pod/')} class="btn-icon preset-tonal" aria-label="Back">
			<ArrowLeftIcon class="size-4" />
		</a>
		<h1 class="h3 lg:h2">{name}</h1>
		<span class={statusClass(status)}>{status}</span>
		<div class="ms-auto flex flex-wrap items-center gap-3">
			{#if detail?.scraper}
				<div class="hidden text-sm lg:block">
					<div class="font-medium">{detail.scraper.title || '—'}</div>
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
						disabled={busyAction !== null}
					>
						{#if busyAction === 'stop'}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<SquareIcon class="size-4" />
						{/if}
					</button>
				{:else}
					<button
						type="button"
						class="btn-icon preset-tonal-success"
						aria-label="Start"
						title="Start"
						onclick={handleStart}
						disabled={busyAction !== null}
					>
						{#if busyAction === 'start'}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<PlayIcon class="size-4" />
						{/if}
					</button>
				{/if}
				<button
					type="button"
					class="btn-icon preset-tonal-error"
					aria-label="Delete container"
					title="Delete container"
					onclick={handleDelete}
					disabled={busyAction !== null}
				>
					{#if busyAction === 'delete'}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<Trash2Icon class="size-4" />
					{/if}
				</button>
				<button
					type="button"
					class="btn-icon preset-tonal-error"
					aria-label="Delete container files"
					title="Delete container files (wipe Firefox profile + xemu config on disk)"
					disabled={isRunning || loading || busyAction !== null}
					onclick={handleDeleteFiles}
				>
					{#if busyAction === 'delete-files'}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<EraserIcon class="size-4" />
					{/if}
				</button>
			</div>
		</div>
	</header>

	<div class="grid flex-1 grid-cols-1 gap-3 overflow-hidden xl:grid-cols-2 2xl:grid-cols-3">
		<!-- Kiosk iframe -->
		<KioskFrame
			class="2xl:col-span-2"
			{name}
			src={kioskSrc}
			running={!!detail && isRunning}
			{loading}
			{vncConnected}
			externalHref={detail ? kioskURL() : undefined}
			onSnapshot={onSnapshotSlot}
			onReset={onResetVM}
		/>

		<!-- Controls + Logs -->
		<div class="flex min-h-0 flex-col gap-2 overflow-hidden card p-3">
			<Tabs defaultValue="controller">
				<Tabs.List>
					<Tabs.Trigger value="controller" class="flex-1">Controller</Tabs.Trigger>
					<Tabs.Trigger value="logs" class="flex-1">Logs</Tabs.Trigger>
					<Tabs.Indicator />
				</Tabs.List>
				<Tabs.Content value="controller" class="flex min-h-0 flex-1 flex-col overflow-y-auto">
					<XboxController
						disabled={!vncConnected}
						onPress={(sym) => vnc?.sendKey(sym, true)}
						onRelease={(sym) => vnc?.sendKey(sym, false)}
					/>
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
								{#if logsLoading}
									<LoaderIcon class="size-4 animate-spin" />
								{:else}
									<RefreshCwIcon class="size-4" />
								{/if}
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
