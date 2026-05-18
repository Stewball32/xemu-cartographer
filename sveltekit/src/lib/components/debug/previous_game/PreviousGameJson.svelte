<script lang="ts">
	import { ChevronRightIcon, FileJsonIcon, FolderIcon } from '@lucide/svelte';
	import { TreeView, createTreeViewCollection } from '@skeletonlabs/skeleton-svelte';
	import type { EnvelopeV2, PreviousGamePayload } from '$lib/types/scraper-v2';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import { buildTreeRoot, scopeAtPath, type XNode } from './json-scope';

	let { envelope }: { envelope: EnvelopeV2<PreviousGamePayload> | null } = $props();

	let selectedValue = $state<string[]>([]);

	// Reset the selection when the envelope identity changes (e.g. a runner
	// restart re-keys the tree); otherwise a stale selectedValue points at
	// an id that no longer exists in the new collection.
	const envelopeKey = $derived(envelope ? `${envelope.instance}|${envelope.v}` : '');
	let lastEnvelopeKey = $state('');
	$effect(() => {
		if (envelopeKey !== lastEnvelopeKey) {
			lastEnvelopeKey = envelopeKey;
			selectedValue = [];
		}
	});

	const collection = $derived.by(() =>
		createTreeViewCollection<XNode>({
			nodeToValue: (n) => n.id,
			nodeToString: (n) => n.name,
			rootNode: buildTreeRoot(envelope)
		})
	);

	const scoped = $derived(scopeAtPath(envelope, selectedValue));
	const codeText = $derived(JSON.stringify(scoped === undefined ? null : scoped, null, 2));

	function reset() {
		selectedValue = [];
	}
</script>

{#snippet treeNode(node: XNode, indexPath: number[])}
	<TreeView.NodeProvider value={{ node, indexPath }}>
		{#if node.children && node.children.length > 0}
			<TreeView.Branch>
				<TreeView.BranchControl
					class="data-[selected]:text-primary-600-300 flex cursor-pointer items-center gap-1 rounded px-2 py-1 hover:bg-surface-200-800/40 data-[selected]:bg-primary-500/15"
				>
					<TreeView.BranchIndicator>
						<ChevronRightIcon class="size-3.5 transition group-data-[state=open]:rotate-90" />
					</TreeView.BranchIndicator>
					<TreeView.BranchText class="inline-flex items-center gap-1.5 font-mono text-xs">
						<FolderIcon class="size-3.5" />
						{node.name}
					</TreeView.BranchText>
				</TreeView.BranchControl>
				<TreeView.BranchContent class="ml-3 border-l border-surface-300-700/40 pl-2">
					{#each node.children as childNode, childIndex (childNode.id)}
						{@render treeNode(childNode, [...indexPath, childIndex])}
					{/each}
				</TreeView.BranchContent>
			</TreeView.Branch>
		{:else}
			<TreeView.Item
				class="data-[selected]:text-primary-600-300 flex cursor-pointer items-center gap-1.5 rounded px-2 py-1 font-mono text-xs hover:bg-surface-200-800/40 data-[selected]:bg-primary-500/15"
			>
				<FileJsonIcon class="size-3.5" />
				{node.name}
			</TreeView.Item>
		{/if}
	</TreeView.NodeProvider>
{/snippet}

<div class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_2fr]">
	<div class="flex flex-col gap-2 card preset-tonal p-3">
		<div class="flex items-center justify-between gap-2">
			<div class="text-surface-700-200 text-xs font-semibold tracking-wide uppercase">tree</div>
			<button
				type="button"
				class="btn preset-tonal btn-sm"
				disabled={selectedValue.length === 0}
				onclick={reset}
			>
				Reset
			</button>
		</div>
		{#if !envelope}
			<div class="text-surface-500-400 text-xs">no envelope to walk</div>
		{:else}
			<TreeView
				{collection}
				{selectedValue}
				onSelectionChange={(d) => (selectedValue = d.selectedValue)}
			>
				<TreeView.Tree class="flex flex-col gap-0.5">
					{#each collection.rootNode.children ?? [] as node, index (node.id)}
						{@render treeNode(node, [index])}
					{/each}
				</TreeView.Tree>
			</TreeView>
		{/if}
	</div>

	<div class="flex flex-col gap-2">
		<div class="flex items-center justify-between gap-2">
			<div class="text-surface-700-200 text-xs font-semibold tracking-wide uppercase">
				json
				{#if selectedValue.length > 0}
					<span class="text-surface-500-400 font-mono normal-case">
						· {selectedValue[0]}
					</span>
				{/if}
			</div>
		</div>
		<CodeBlock code={codeText} />
	</div>
</div>
