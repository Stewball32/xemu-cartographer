<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { LoaderIcon, RefreshCwIcon } from '@lucide/svelte';
	import { SegmentedControl } from '@skeletonlabs/skeleton-svelte';
	import { describeAsyncError, toaster } from '$lib/stores/toaster';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import type { LogsWhich } from '$lib/types/containers';
	import { fetchContainerLogs } from './logs-vm';

	let { name }: { name: string } = $props();

	let source = $state<LogsWhich>('xemu');
	let logs = $state<Record<LogsWhich, string>>({ xemu: '', browser: '' });
	let loading = $state(false);

	const currentText = $derived(logs[source]);
	const placeholder = $derived(loading ? 'Loading…' : '(no logs)');

	async function refresh() {
		if (loading) return;
		// Snapshot which source this fetch targets — `source` may change mid-flight
		// when the user flips the toggle; we still want the result to land in the
		// right slot.
		const target = source;
		try {
			loading = true;
			const next = await fetchContainerLogs(name, target);
			if (logs[target] !== next) logs[target] = next;
		} catch (err) {
			toaster.error({ title: 'Logs fetch failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function onSourceChange(next: string | null) {
		if (next !== 'xemu' && next !== 'browser') return;
		source = next;
		refresh();
	}

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	onMount(() => {
		refresh();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			refresh();
		}, 5000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
		pollTimer = null;
	});
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center gap-2">
		<SegmentedControl
			value={source}
			onValueChange={(d) => onSourceChange(d.value)}
			aria-label="Log source"
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

		<button
			type="button"
			class="ms-auto btn-icon preset-tonal"
			aria-label="Refresh logs"
			title="Refresh logs"
			onclick={refresh}
			disabled={loading}
		>
			{#if loading}
				<LoaderIcon class="size-4 animate-spin" />
			{:else}
				<RefreshCwIcon class="size-4" />
			{/if}
		</button>
	</div>

	<CodeBlock
		code={currentText || placeholder}
		classes="max-h-[70vh] overflow-y-auto"
		preClasses="p-3 text-xs font-mono whitespace-pre-wrap"
		copyable={!!currentText}
	/>
</div>
