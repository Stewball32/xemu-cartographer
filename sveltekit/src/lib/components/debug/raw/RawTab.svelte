<script lang="ts">
	import { SegmentedControl } from '@skeletonlabs/skeleton-svelte';
	import { CopyIcon } from '@lucide/svelte';
	import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
	import { toaster } from '$lib/stores/toaster';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import { useDebugContext } from '../context.js';
	import { buildRawVm, type RawSource } from './raw-vm';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	let source = $state<RawSource>('inspect');
	const vm = $derived.by(() => buildRawVm(name, source, ctx.inspect, scraperWSV2));

	async function copyJson() {
		const json = vm.displayJson;
		if (!json) return;
		try {
			await navigator.clipboard.writeText(json);
			toaster.success({
				title: 'Copied',
				description: `${source === 'inspect' ? 'Inspect snapshot' : 'Live WS state'} copied to clipboard.`
			});
		} catch {
			// Fallback to download blob, matching exportAnnotations() in +page.svelte.
			const blob = new Blob([json], { type: 'application/json' });
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `${name}-${source}.json`;
			a.click();
			URL.revokeObjectURL(url);
		}
	}
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<SegmentedControl value={source} onValueChange={(d) => (source = d.value as RawSource)}>
			<SegmentedControl.Indicator />
			<SegmentedControl.Item value="inspect">
				<SegmentedControl.ItemText>inspect (REST)</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
			<SegmentedControl.Item value="live">
				<SegmentedControl.ItemText>live (WS)</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
		</SegmentedControl>
		<button
			type="button"
			class="btn preset-tonal btn-sm"
			disabled={!vm.displayJson}
			onclick={copyJson}
		>
			<CopyIcon class="size-4" />
			<span>Copy</span>
		</button>
	</div>

	<div class="max-h-[70vh] overflow-y-auto card preset-tonal p-0">
		{#if !vm.displayJson}
			<div class="text-surface-500-400 p-6 text-center text-sm">
				{source === 'inspect'
					? 'No inspect snapshot yet — wait for the next 3s poll once a scraper is attached.'
					: 'No live WS state yet — wait for the first per-class envelope.'}
			</div>
		{:else}
			<CodeBlock code={vm.displayJson} copyable={false} classes="m-0" />
		{/if}
	</div>
</div>
