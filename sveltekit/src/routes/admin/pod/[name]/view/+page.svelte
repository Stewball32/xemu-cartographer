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
		last_kind: string;
		last_intent: string;
		last_keys: string[] | null;
		last_reason: string;
		machine_count: number;
		player_count: number;
		team_count: number;
		countdown_active: boolean;
		authority: string;
		main_menu_raw: number;
		ui_widget_blocks: number;
		ui_highlighted: number;
		ui_max_tick: number;
		tree_built: boolean;
		readout_seq: number;
		readout_age_ms: number;
		nav_candidates: Record<string, number> | null;
	};

	// CHANGE TRACKING. The whole point of this panel is answering "which signal
	// actually updates on THIS screen?" — e.g. on SELECT PROFILE the dela does NOT
	// move, so the runner can't confirm on it. We remember each signal's previous
	// value and when it last changed, and mark the ones that just moved. Sit on a
	// screen, navigate, and the rows that light up are the usable confirm signals.
	type Sig = { label: string; value: string; group: string; changedAt: number };

	// lastSeen is DELIBERATELY NOT $state. It was, and the $effect that maintained it
	// both READ it (spreading the previous map) and WROTE it — a self-triggering effect,
	// which Svelte 5 aborts with effect_update_depth_exceeded. That crash killed the
	// component's reactivity, so the panel rendered once and then FROZE: the exact
	// "panel doesn't update" bug. Bookkeeping now lives in a plain Map mutated while the
	// signals list is (re)built, so nothing reactive is written and no effect is needed.
	const lastSeen = new Map<string, { value: string; at: number }>();
	let diag = $state<Diagnostics | null>(null);
	let diagError = $state<string | null>(null);
	let diagTimer: ReturnType<typeof setInterval> | null = null;

	const cur = (c: { index: number; count: number; valid: boolean }) =>
		`${c.count ? c.index + 1 : 0}/${c.count}${c.valid ? '' : ' (invalid)'}`;

	// Every live read, grouped. Order = most useful first for nav debugging.
	const signals = $derived.by<Sig[]>(() => {
		const d = diag;
		if (!d) return [];
		const now = Date.now();
		const raw: Array<{ label: string; value: string; group: string }> = [
			{ group: 'screen', label: 'screen', value: d.screen || 'unknown' },
			{ group: 'screen', label: 'menu_item', value: `${d.menu_item_name} (${d.menu_item})` },
			{ group: 'screen', label: 'dela', value: d.dela || '—' },
			{ group: 'screen', label: 'menu_focus', value: `0x${(d.menu_focus >>> 0).toString(16)}` },
			{ group: 'screen', label: 'game_connection', value: connName(d.game_connection) },
			{ group: 'screen', label: 'pregame_sentinel', value: d.pregame_sentinel ? '0xDEADBEEF' : 'absent' },
			// LIVENESS first: readout_seq advances every scraper tick, so it — not the
			// GAME tick — proves the panel is live. The game tick is legitimately 0 at
			// the menus, which is exactly what made a frozen panel indistinguishable
			// from a healthy one sitting in the front end.
			{ group: 'live', label: 'readout seq', value: String(d.readout_seq) },
			{ group: 'live', label: 'readout age', value: `${d.readout_age_ms} ms` },
			{ group: 'live', label: 'tick (game — 0 in menus)', value: String(d.tick) },
			{ group: 'cold', label: 'tree built?', value: d.tree_built ? 'YES — woken' : 'NO — cold/un-woken' },
			{ group: 'cold', label: 'ui widget blocks', value: String(d.ui_widget_blocks) },
			{ group: 'cold', label: 'ui highlighted', value: String(d.ui_highlighted) },
			{ group: 'cold', label: 'ui max tick', value: `0x${(d.ui_max_tick >>> 0).toString(16)}` },
			{ group: 'cold', label: 'main_menu (raw)', value: String(d.main_menu_raw) },
			{ group: 'select', label: 'map cursor', value: cur(d.map_cursor) },
			{ group: 'select', label: 'map highlighted', value: d.highlighted_map || '—' },
			{ group: 'select', label: 'gametype cursor', value: cur(d.gametype_cursor) },
			{ group: 'select', label: 'gametype highlighted', value: d.highlighted_gametype || '—' },
			{ group: 'select', label: 'map picked → loaded', value: `${d.selected_map || '—'} → ${d.map || '—'}` },
			{ group: 'select', label: 'gametype picked → loaded', value: `${d.selected_gametype || '—'} → ${d.gametype || '—'}` },
			{ group: 'runner', label: 'authority', value: d.authority || '—' },
			{ group: 'runner', label: 'last action', value: `${d.last_kind}${d.last_intent ? ` [${d.last_intent}]` : ''}` },
			{ group: 'runner', label: 'last keys', value: (d.last_keys ?? []).join(',') || '—' },
			{ group: 'runner', label: 'last reason', value: d.last_reason || '—' },
			{ group: 'lobby', label: 'machines', value: String(d.machine_count) },
			{ group: 'lobby', label: 'players', value: String(d.player_count) },
			{ group: 'lobby', label: 'teams', value: String(d.team_count) },
			{ group: 'lobby', label: 'countdown', value: d.countdown_active ? 'active' : 'no' },
			// Raw candidate offsets from the capture-and-diff hunts — unclassified, for
			// eyeballing which one tracks a screen. Sorted so the row order is stable.
			...Object.entries(d.nav_candidates ?? {})
				.sort(([a], [b]) => a.localeCompare(b))
				.map(([label, v]) => ({
					group: 'cand',
					label,
					value: `0x${(v >>> 0).toString(16).padStart(8, '0')}`
				})),
			{ group: 'lobby', label: 'enumerated', value: `${d.enumerated_maps?.length ?? 0} maps / ${d.enumerated_gametypes?.length ?? 0} gametypes` }
		];
		// Fold the change bookkeeping in here (before render) so a row's "changed" mark
		// is correct on the very poll it changes — and so no reactive state is written.
		return raw.map((sig) => {
			const prev = lastSeen.get(sig.label);
			if (!prev) lastSeen.set(sig.label, { value: sig.value, at: 0 });
			else if (prev.value !== sig.value) lastSeen.set(sig.label, { value: sig.value, at: now });
			return { ...sig, changedAt: lastSeen.get(sig.label)!.at };
		});
	});
	const GROUPS = [
		{ key: 'live', title: 'Liveness (is this panel updating?)' },
		{ key: 'screen', title: 'Screen / nav signals' },
		{ key: 'cold', title: 'Cold-boot / widget-tree candidates' },
		{ key: 'select', title: 'Map + gametype select' },
		{ key: 'runner', title: 'Runner decision' },
		{ key: 'lobby', title: 'Lobby' },
		{ key: 'cand', title: 'Candidate offsets (raw — which one tracks?)' }
	];

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
				<!-- Grouped live reads. A row is highlighted for a few seconds after its
				     value CHANGES — sit on a screen, navigate, and the rows that light up
				     are the signals usable to confirm a press there. -->
				<div class="grid gap-3 lg:grid-cols-2">
					{#each GROUPS as g (g.key)}
						<div class="flex flex-col gap-1">
							<h4 class="text-[0.65rem] font-semibold tracking-wide text-surface-500 uppercase">
								{g.title}
							</h4>
							<table class="w-full text-xs">
								<tbody>
									{#each signals.filter((x) => x.group === g.key) as sig (sig.label)}
										{@const ago = sig.changedAt ? Date.now() - sig.changedAt : null}
										{@const fresh = ago !== null && ago < 3000}
										<tr class={fresh ? 'bg-success-500/15' : ''}>
											<td class="w-40 py-0.5 align-top text-surface-600-400">{sig.label}</td>
											<td class="py-0.5 font-mono break-all">{sig.value}</td>
											<td class="w-16 py-0.5 text-right align-top text-[0.6rem] text-surface-500">
												{#if ago === null}—{:else if fresh}<span class="text-success-500">changed</span
													>{:else}{Math.round(ago / 1000)}s{/if}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</div>
