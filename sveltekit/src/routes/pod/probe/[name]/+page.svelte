<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { ArrowLeftIcon, BugIcon } from '@lucide/svelte';
	import { adminGet, AdminFetchError } from '$lib/utils/admin-api';
	import { apiBaseURL } from '$lib/utils/api-base';
	import { auth } from '$lib/stores/auth.svelte';
	import type { ScraperInspect } from '$lib/types/scraper';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import ProbeTab from '$lib/components/debug/probe/ProbeTab.svelte';
	import { setDebugContext } from '$lib/components/debug/context.js';

	let { data } = $props();
	const name = $derived(data.name);

	let inspect = $state<ScraperInspect | null>(null);
	let inspectAt = $state<number | undefined>(undefined);
	let now = $state(Date.now());

	// /tmp/qmp/<name>.sock is the convention the discovery watcher uses for
	// container-provisioned xemu instances — preseed it so the Run buttons work
	// out of the box for the common case.
	const defaultSock = $derived(`/tmp/qmp/${name}.sock`);

	function relativeTime(ts: number | undefined): string {
		if (!ts) return 'never';
		const diff = now - ts;
		if (diff < 1000) return 'just now';
		if (diff < 60_000) return `${Math.floor(diff / 1000)}s ago`;
		if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
		return `${Math.floor(diff / 3_600_000)}h ago`;
	}

	setDebugContext({
		get inspect() {
			return inspect;
		},
		get inspectAt() {
			return inspectAt;
		},
		get now() {
			return now;
		},
		relativeTime
	});

	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let nowTimer: ReturnType<typeof setInterval> | null = null;

	async function refreshInspect() {
		try {
			inspect = await adminGet<ScraperInspect>(`scraper/${encodeURIComponent(name)}/inspect`);
			inspectAt = Date.now();
		} catch (err) {
			if (err instanceof AdminFetchError && err.status === 404) {
				inspect = null;
			} else {
				console.warn('inspect fetch failed', err);
			}
		}
	}

	onMount(() => {
		refreshInspect();
		pollTimer = setInterval(() => {
			if (document.visibilityState === 'visible') refreshInspect();
		}, 3000);
		nowTimer = setInterval(() => (now = Date.now()), 1000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
		if (nowTimer) clearInterval(nowTimer);
	});

	type ToolKey = 'probe' | 'probeTitle' | 'sampleDeltas' | 'scanString';
	type ToolState = { result: string; error: string; loading: boolean };

	const tools = $state<Record<ToolKey, ToolState>>({
		probe: { result: '', error: '', loading: false },
		probeTitle: { result: '', error: '', loading: false },
		sampleDeltas: { result: '', error: '', loading: false },
		scanString: { result: '', error: '', loading: false }
	});

	let probeSock = $state('');
	let probeTitleSock = $state('');
	let probeTitleSamples = $state('30');
	let probeTitleInterval = $state('100');
	let deltasSock = $state('');
	// 0x80050000–0x80060000 is the Xbox kernel-space sweet spot for catching
	// fast-changing globals; small enough to scan in ~1s windows.
	let deltasStart = $state('0x80050000');
	let deltasEnd = $state('0x80060000');
	let deltasInterval = $state('1000');
	let deltasMax = $state('200');
	let scanSock = $state('');
	let scanQ = $state('');
	let scanEncoding = $state<'utf16le' | 'utf8' | 'ascii' | 'hex'>('utf16le');
	let scanMax = $state('20');

	async function runTool(key: ToolKey, path: string, params: Record<string, string>) {
		const t = tools[key];
		t.error = '';
		t.result = '';
		t.loading = true;
		try {
			if (!auth.token) throw new Error('not authenticated');
			const filtered = Object.fromEntries(Object.entries(params).filter(([, v]) => v !== ''));
			const url = `${apiBaseURL()}/api/admin/xemu/${path}?${new URLSearchParams(filtered).toString()}`;
			const res = await fetch(url, { headers: { Authorization: auth.token } });
			const text = await res.text();
			let pretty = text;
			try {
				pretty = JSON.stringify(JSON.parse(text), null, 2);
			} catch {
				// non-JSON body, keep as-is
			}
			if (!res.ok) t.error = `HTTP ${res.status}`;
			t.result = pretty;
		} catch (err) {
			t.error = err instanceof Error ? err.message : String(err);
		} finally {
			t.loading = false;
		}
	}
</script>

<div class="mx-auto flex max-w-7xl flex-col gap-4">
	<!-- Sibling /pod/ routes are scaffolded in parallel units (M7 Unit 2 + 4) —
	     use string hrefs until those route files land so resolve()'s typed
	     route table doesn't reject them. -->
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
	<a class="flex items-center gap-1 anchor text-sm" href="/pod/">
		<ArrowLeftIcon class="size-4" />
		Back to pods
	</a>
	<PageHeader title={name} description="Address hunting — live diagnostics + ad-hoc memory probes.">
		{#snippet actions()}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a class="btn preset-tonal btn-sm" href="/pod/debug/{name}/">
				<BugIcon class="size-4" />
				<span>Debug page</span>
			</a>
		{/snippet}
	</PageHeader>

	<section class="flex flex-col gap-3">
		<header class="flex items-baseline gap-2">
			<h2 class="h4">Live scraper diagnostics</h2>
			<span class="text-surface-500-400 text-xs">refreshed {relativeTime(inspectAt)}</span>
		</header>

		{#if !inspect}
			<div class="card preset-tonal p-3 text-sm">
				No scraper attached for this instance. Start it from
				<a class="anchor" href={resolve('/pod/view/[name]', { name })}>/pod/view/{name}/</a>.
			</div>
		{:else}
			<ProbeTab {name} />
		{/if}
	</section>

	<section class="flex flex-col gap-3">
		<header class="flex items-baseline gap-2">
			<h2 class="h4">Ad-hoc xemu probe tools</h2>
			<span class="text-surface-500-400 text-xs">
				one-shot requests against <code>/api/admin/xemu/*</code>
			</span>
		</header>

		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			<div class="flex flex-col gap-3 card preset-tonal p-4">
				<div class="flex items-baseline justify-between gap-2">
					<h3 class="h5">probe</h3>
					<code class="text-surface-500-400 text-xs">GET /xemu/probe</code>
				</div>
				<label class="label flex flex-col gap-1 text-xs">
					<span>sock</span>
					<input class="input" type="text" placeholder={defaultSock} bind:value={probeSock} />
				</label>
				<button
					type="button"
					class="preset-filled-primary btn w-full"
					disabled={tools.probe.loading}
					onclick={() => runTool('probe', 'probe', { sock: probeSock || defaultSock })}
				>
					{tools.probe.loading ? 'Running…' : 'Run probe'}
				</button>
				{#if tools.probe.error}
					<div class="text-xs text-error-500">{tools.probe.error}</div>
				{/if}
				{#if tools.probe.result}
					<pre
						class="max-h-80 overflow-x-auto overflow-y-auto rounded-container preset-tonal-surface p-3 font-mono text-xs whitespace-pre">{tools
							.probe.result}</pre>
				{/if}
			</div>

			<div class="flex flex-col gap-3 card preset-tonal p-4">
				<div class="flex items-baseline justify-between gap-2">
					<h3 class="h5">probe-title</h3>
					<code class="text-surface-500-400 text-xs">GET /xemu/probe-title</code>
				</div>
				<label class="label flex flex-col gap-1 text-xs">
					<span>sock</span>
					<input class="input" type="text" placeholder={defaultSock} bind:value={probeTitleSock} />
				</label>
				<label class="label flex flex-col gap-1 text-xs">
					<span>samples</span>
					<input class="input" type="text" inputmode="numeric" bind:value={probeTitleSamples} />
				</label>
				<label class="label flex flex-col gap-1 text-xs">
					<span>interval_ms</span>
					<input class="input" type="text" inputmode="numeric" bind:value={probeTitleInterval} />
				</label>
				<button
					type="button"
					class="preset-filled-primary btn w-full"
					disabled={tools.probeTitle.loading}
					onclick={() =>
						runTool('probeTitle', 'probe-title', {
							sock: probeTitleSock || defaultSock,
							samples: probeTitleSamples,
							interval_ms: probeTitleInterval
						})}
				>
					{tools.probeTitle.loading ? 'Running…' : 'Run probe-title'}
				</button>
				{#if tools.probeTitle.error}
					<div class="text-xs text-error-500">{tools.probeTitle.error}</div>
				{/if}
				{#if tools.probeTitle.result}
					<pre
						class="max-h-80 overflow-x-auto overflow-y-auto rounded-container preset-tonal-surface p-3 font-mono text-xs whitespace-pre">{tools
							.probeTitle.result}</pre>
				{/if}
			</div>

			<div class="flex flex-col gap-3 card preset-tonal p-4">
				<div class="flex items-baseline justify-between gap-2">
					<h3 class="h5">sample-deltas</h3>
					<code class="text-surface-500-400 text-xs">GET /xemu/sample-deltas</code>
				</div>
				<label class="label flex flex-col gap-1 text-xs">
					<span>sock</span>
					<input class="input" type="text" placeholder={defaultSock} bind:value={deltasSock} />
				</label>
				<div class="grid grid-cols-2 gap-2">
					<label class="label flex flex-col gap-1 text-xs">
						<span>start</span>
						<input class="input" type="text" bind:value={deltasStart} />
					</label>
					<label class="label flex flex-col gap-1 text-xs">
						<span>end</span>
						<input class="input" type="text" bind:value={deltasEnd} />
					</label>
				</div>
				<div class="grid grid-cols-2 gap-2">
					<label class="label flex flex-col gap-1 text-xs">
						<span>interval_ms</span>
						<input class="input" type="text" inputmode="numeric" bind:value={deltasInterval} />
					</label>
					<label class="label flex flex-col gap-1 text-xs">
						<span>max</span>
						<input class="input" type="text" inputmode="numeric" bind:value={deltasMax} />
					</label>
				</div>
				<button
					type="button"
					class="preset-filled-primary btn w-full"
					disabled={tools.sampleDeltas.loading}
					onclick={() =>
						runTool('sampleDeltas', 'sample-deltas', {
							sock: deltasSock || defaultSock,
							start: deltasStart,
							end: deltasEnd,
							interval_ms: deltasInterval,
							max: deltasMax
						})}
				>
					{tools.sampleDeltas.loading ? 'Running…' : 'Run sample-deltas'}
				</button>
				{#if tools.sampleDeltas.error}
					<div class="text-xs text-error-500">{tools.sampleDeltas.error}</div>
				{/if}
				{#if tools.sampleDeltas.result}
					<pre
						class="max-h-80 overflow-x-auto overflow-y-auto rounded-container preset-tonal-surface p-3 font-mono text-xs whitespace-pre">{tools
							.sampleDeltas.result}</pre>
				{/if}
			</div>

			<div class="flex flex-col gap-3 card preset-tonal p-4">
				<div class="flex items-baseline justify-between gap-2">
					<h3 class="h5">scan-string</h3>
					<code class="text-surface-500-400 text-xs">GET /xemu/scan-string</code>
				</div>
				<label class="label flex flex-col gap-1 text-xs">
					<span>sock</span>
					<input class="input" type="text" placeholder={defaultSock} bind:value={scanSock} />
				</label>
				<label class="label flex flex-col gap-1 text-xs">
					<span>query</span>
					<input
						class="input"
						type="text"
						placeholder="text or hex (with encoding=hex)"
						bind:value={scanQ}
					/>
				</label>
				<div class="grid grid-cols-2 gap-2">
					<label class="label flex flex-col gap-1 text-xs">
						<span>encoding</span>
						<select class="select" bind:value={scanEncoding}>
							<option value="utf16le">utf16le</option>
							<option value="utf8">utf8</option>
							<option value="ascii">ascii</option>
							<option value="hex">hex</option>
						</select>
					</label>
					<label class="label flex flex-col gap-1 text-xs">
						<span>max</span>
						<input class="input" type="text" inputmode="numeric" bind:value={scanMax} />
					</label>
				</div>
				<button
					type="button"
					class="preset-filled-primary btn w-full"
					disabled={tools.scanString.loading}
					onclick={() =>
						runTool('scanString', 'scan-string', {
							sock: scanSock || defaultSock,
							q: scanQ,
							encoding: scanEncoding,
							max: scanMax
						})}
				>
					{tools.scanString.loading ? 'Running…' : 'Run scan-string'}
				</button>
				{#if tools.scanString.error}
					<div class="text-xs text-error-500">{tools.scanString.error}</div>
				{/if}
				{#if tools.scanString.result}
					<pre
						class="max-h-80 overflow-x-auto overflow-y-auto rounded-container preset-tonal-surface p-3 font-mono text-xs whitespace-pre">{tools
							.scanString.result}</pre>
				{/if}
			</div>
		</div>
	</section>
</div>
