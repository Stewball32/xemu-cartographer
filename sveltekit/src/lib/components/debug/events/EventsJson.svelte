<script lang="ts">
	import { ChevronRightIcon, FileJsonIcon, FolderIcon } from '@lucide/svelte';
	import { TreeView, createTreeViewCollection } from '@skeletonlabs/skeleton-svelte';
	import type { AnyEvent } from '$lib/types/scraper-v2';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import { buildTreeRoot, scopeAtPath, EVENT_SHAPE, type XNode } from '../shared/json-scope';

	// Rolling log of event envelopes (newest first per the v2 WS store).
	let { events, instance }: { events: AnyEvent[]; instance: string } = $props();

	let selectedValue = $state<string[]>([]);
	let expandedValue = $state<string[]>([]);

	// Tree value scheme: every event lives under a top-level `event:<idx>`
	// node — the index is the position in the newest-first rolling log so
	// re-selection survives a re-render. Selecting a deeper node scopes
	// within that event; the leading segment is the event id.
	type EventEntry = { event: AnyEvent; nodeId: string; label: string };
	const entries = $derived<EventEntry[]>(
		events.map((ev, idx) => ({
			event: ev,
			nodeId: `event:${idx}`,
			label: `${ev.seq}:${ev.tick}  ${ev.event_type}`
		}))
	);

	function buildLogTree(items: EventEntry[]): XNode {
		const root: XNode = { id: '', name: 'events log', children: [] };
		for (const e of items) {
			const child = buildTreeRoot(e.event, EVENT_SHAPE);
			// Hoist the per-event tree under a stable id; remap descendant ids
			// so selection round-trips through scopeAtPath via the parent id
			// prefix.
			const wrapped: XNode = {
				id: e.nodeId,
				name: e.label,
				children: child.children?.map((c) => prefixIds(c, e.nodeId))
			};
			root.children!.push(wrapped);
		}
		return root;
	}

	function prefixIds(node: XNode, prefix: string): XNode {
		const next: XNode = {
			id: `${prefix}.${node.id}`,
			name: node.name,
			children: node.children?.map((c) => prefixIds(c, prefix))
		};
		return next;
	}

	// Reset the selection when the instance changes; the per-event indexes
	// remain stable as long as we keep watching the same rolling log.
	let lastInstance = $state('');
	$effect(() => {
		if (instance !== lastInstance) {
			lastInstance = instance;
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
			rootNode: buildLogTree(entries)
		})
	);

	// Scope: every selected id starts with the event-entry id (`event:<idx>`).
	// Group selections by entry so multi-select within one event composes via
	// the shared scopeAtPath, while across-entry selections stay grouped per
	// entry (the user can still see which event each slice came from).
	const scoped = $derived.by<unknown>(() => {
		if (events.length === 0) return null;
		if (selectedValue.length === 0) return entries[0]?.event ?? null;

		const byEntry: Record<string, string[]> = {};
		for (const id of selectedValue) {
			const dot = id.indexOf('.');
			const entryId = dot >= 0 ? id.slice(0, dot) : id;
			const innerPath = dot >= 0 ? id.slice(dot + 1) : '';
			const list = byEntry[entryId] ?? [];
			if (innerPath) list.push(innerPath);
			byEntry[entryId] = list;
		}

		const out: Record<string, unknown> = {};
		for (const [entryId, paths] of Object.entries(byEntry)) {
			const match = entries.find((e) => e.nodeId === entryId);
			if (!match) continue;
			out[entryId] = paths.length > 0 ? scopeAtPath(match.event, paths, EVENT_SHAPE) : match.event;
		}
		return out;
	});
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
		{#if entries.length === 0}
			<div class="text-surface-500-400 text-xs">no event envelopes to walk</div>
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
