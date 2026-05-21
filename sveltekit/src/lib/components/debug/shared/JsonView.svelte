<script lang="ts">
	import { ChevronRightIcon, FileJsonIcon, FolderIcon } from '@lucide/svelte';
	import { TreeView, createTreeViewCollection } from '@skeletonlabs/skeleton-svelte';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import { buildTreeRoot, scopeAtPath, type EnvelopeShape, type XNode } from './json-scope';

	let {
		envelope,
		shape,
		resetKey
	}: {
		envelope: unknown | null;
		shape: EnvelopeShape;
		// When this string changes, the selection clears. Default keys on
		// envelope identity (instance + protocol version) so a runner restart
		// or a shape switch doesn't leave dangling node ids in selectedValue.
		resetKey?: string;
	} = $props();

	let selectedValue = $state<string[]>([]);
	let expandedValue = $state<string[]>([]);

	const computedResetKey = $derived.by(() => {
		if (resetKey !== undefined) return resetKey;
		if (envelope && typeof envelope === 'object') {
			const env = envelope as Record<string, unknown>;
			const inst = typeof env.instance === 'string' ? env.instance : '';
			const v = typeof env.v === 'number' ? env.v : '';
			return `${inst}|${v}`;
		}
		return '';
	});
	let lastResetKey = $state('');
	$effect(() => {
		if (computedResetKey !== lastResetKey) {
			lastResetKey = computedResetKey;
			selectedValue = [];
			expandedValue = [];
		}
	});

	function toggleIn(arr: string[], v: string): string[] {
		return arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v];
	}

	// Intercept tree-row clicks in capture phase so plain clicks toggle selection
	// (Zag's default is replace-on-click, with toggle gated behind ctrl/cmd —
	// not discoverable for a debug inspector). Chevron clicks route to expansion
	// instead so the operator can navigate without disturbing selection.
	function handleTreeClick(event: MouseEvent) {
		const target = event.target as HTMLElement;
		const valueEl = target.closest('[data-value]') as HTMLElement | null;
		if (!valueEl) return;
		const value = valueEl.getAttribute('data-value');
		if (!value) return;
		event.preventDefault();
		event.stopPropagation();
		event.stopImmediatePropagation();
		const isChevron = !!target.closest('[data-part="branch-indicator"]');
		const isBranch = valueEl.hasAttribute('data-state');
		if (isChevron && isBranch) {
			expandedValue = toggleIn(expandedValue, value);
		} else {
			selectedValue = toggleIn(selectedValue, value);
		}
	}

	const collection = $derived.by(() =>
		createTreeViewCollection<XNode>({
			nodeToValue: (n) => n.id,
			nodeToString: (n) => n.name,
			rootNode: buildTreeRoot(envelope, shape)
		})
	);

	const scoped = $derived(scopeAtPath(envelope, selectedValue, shape));
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
	<div class="flex flex-col gap-2 card preset-tonal p-3" onclickcapture={handleTreeClick}>
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
				selectionMode="multiple"
				{selectedValue}
				{expandedValue}
				onSelectionChange={(d) => (selectedValue = d.selectedValue)}
				onExpandedChange={(d) => (expandedValue = d.expandedValue)}
			>
				<TreeView.Tree class="flex flex-col gap-0.5">
					{#each collection.rootNode.children ?? [] as node, index (node.id)}
						{@render treeNode(node, [index])}
					{/each}
				</TreeView.Tree>
			</TreeView>
			<div class="text-surface-500-400 text-xs">click name to toggle · click ▸ to expand</div>
		{/if}
	</div>

	<div class="flex flex-col gap-2">
		<div class="flex items-center justify-between gap-2">
			<div class="text-surface-700-200 text-xs font-semibold tracking-wide uppercase">
				json
				{#if selectedValue.length === 1}
					<span class="text-surface-500-400 font-mono normal-case">· {selectedValue[0]}</span>
				{:else if selectedValue.length > 1}
					<span class="text-surface-500-400 font-mono normal-case">
						· {selectedValue.length} paths
					</span>
				{/if}
			</div>
		</div>
		<CodeBlock code={codeText} />
	</div>
</div>
