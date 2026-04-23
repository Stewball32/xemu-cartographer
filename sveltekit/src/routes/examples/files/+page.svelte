<script lang="ts">
	import { TreeView, createTreeViewCollection } from '@skeletonlabs/skeleton-svelte';
	import {
		FileIcon,
		FolderIcon,
		FileTextIcon,
		FileCodeIcon,
		FileImageIcon,
		UploadIcon,
		FolderPlusIcon,
		DownloadIcon
	} from '@lucide/svelte';

	interface Node {
		id: string;
		name: string;
		kind?: 'doc' | 'code' | 'image' | 'file';
		size?: string;
		modified?: string;
		children?: Node[];
	}

	const collection = createTreeViewCollection<Node>({
		nodeToValue: (node) => node.id,
		nodeToString: (node) => node.name,
		rootNode: {
			id: 'root',
			name: 'Files',
			children: [
				{
					id: 'documents',
					name: 'Documents',
					children: [
						{
							id: 'reports',
							name: 'Reports',
							children: [
								{ id: 'q1', name: 'Q1 Report.pdf', kind: 'doc', size: '2.4 MB', modified: 'Apr 2' },
								{ id: 'q2', name: 'Q2 Report.pdf', kind: 'doc', size: '3.1 MB', modified: 'Apr 5' }
							]
						},
						{ id: 'roadmap', name: 'Roadmap.md', kind: 'doc', size: '12 KB', modified: 'Mar 28' },
						{
							id: 'meeting',
							name: 'Meeting Notes.txt',
							kind: 'doc',
							size: '4 KB',
							modified: 'Apr 10'
						}
					]
				},
				{
					id: 'code',
					name: 'Code',
					children: [
						{ id: 'main', name: 'main.go', kind: 'code', size: '8 KB', modified: 'Apr 11' },
						{ id: 'app', name: 'App.svelte', kind: 'code', size: '3 KB', modified: 'Apr 11' },
						{ id: 'readme', name: 'README.md', kind: 'doc', size: '6 KB', modified: 'Apr 10' }
					]
				},
				{
					id: 'images',
					name: 'Images',
					children: [
						{ id: 'hero', name: 'hero.jpg', kind: 'image', size: '1.2 MB', modified: 'Apr 8' },
						{ id: 'logo', name: 'logo.svg', kind: 'image', size: '18 KB', modified: 'Mar 15' }
					]
				},
				{ id: 'notes', name: 'todo.txt', kind: 'doc', size: '2 KB', modified: 'Apr 12' }
			]
		}
	});

	let selectedFolder = $state('documents');

	function flatFiles(node: Node): Node[] {
		if (!node.children) return [node];
		return node.children.flatMap((c) => (c.children ? [] : [c]));
	}

	function findNode(node: Node, id: string): Node | null {
		if (node.id === id) return node;
		if (!node.children) return null;
		for (const c of node.children) {
			const r = findNode(c, id);
			if (r) return r;
		}
		return null;
	}

	let currentFolder = $derived(findNode(collection.rootNode, selectedFolder));
	let filesInFolder = $derived(currentFolder ? flatFiles(currentFolder) : []);

	function iconFor(kind: Node['kind']) {
		switch (kind) {
			case 'code':
				return FileCodeIcon;
			case 'image':
				return FileImageIcon;
			case 'doc':
				return FileTextIcon;
			default:
				return FileIcon;
		}
	}
</script>

<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<div>
		<h1 class="h2">Files</h1>
		<p class="text-sm opacity-70">Browse folders and files in your workspace.</p>
	</div>
	<div class="flex gap-2">
		<button class="btn preset-tonal btn-sm">
			<FolderPlusIcon class="size-4" />
			<span>New Folder</span>
		</button>
		<button class="btn preset-filled btn-sm">
			<UploadIcon class="size-4" />
			<span>Upload</span>
		</button>
	</div>
</div>

<div class="grid grid-cols-1 gap-4 md:grid-cols-[18rem_1fr]">
	<!-- Tree -->
	<aside class="card p-4">
		<TreeView {collection}>
			<TreeView.Tree>
				{#each collection.rootNode.children ?? [] as node, index (node.id)}
					{@render treeNode(node, [index])}
				{/each}
			</TreeView.Tree>
		</TreeView>
	</aside>

	<!-- File list -->
	<section class="card p-0">
		<div class="flex items-center justify-between border-b border-surface-200-800 p-4">
			<h2 class="h5">{currentFolder?.name ?? 'Files'}</h2>
			<span class="text-xs opacity-60">{filesInFolder.length} items</span>
		</div>
		<div class="table-wrap">
			<table class="table">
				<thead>
					<tr>
						<th>Name</th>
						<th class="hidden sm:table-cell">Size</th>
						<th class="hidden md:table-cell">Modified</th>
						<th></th>
					</tr>
				</thead>
				<tbody class="[&>tr]:hover:preset-tonal-primary">
					{#each filesInFolder as file (file.id)}
						{@const Icon = iconFor(file.kind)}
						<tr>
							<td>
								<div class="flex items-center gap-2">
									<Icon class="size-4 text-primary-500" />
									<span class="text-sm">{file.name}</span>
								</div>
							</td>
							<td class="hidden text-sm opacity-70 sm:table-cell">{file.size}</td>
							<td class="hidden text-sm opacity-70 md:table-cell">{file.modified}</td>
							<td class="text-right">
								<button class="btn-icon btn-icon-sm preset-tonal" aria-label="Download">
									<DownloadIcon class="size-4" />
								</button>
							</td>
						</tr>
					{/each}
					{#if filesInFolder.length === 0}
						<tr>
							<td colspan="4" class="text-center text-sm opacity-60">No files in this folder.</td>
						</tr>
					{/if}
				</tbody>
			</table>
		</div>
	</section>
</div>

{#snippet treeNode(node: Node, indexPath: number[])}
	<TreeView.NodeProvider value={{ node, indexPath }}>
		{#if node.children}
			<TreeView.Branch>
				<TreeView.BranchControl onclick={() => (selectedFolder = node.id)}>
					<TreeView.BranchIndicator>
						<FolderIcon class="size-4 text-primary-500" />
					</TreeView.BranchIndicator>
					<TreeView.BranchText>{node.name}</TreeView.BranchText>
				</TreeView.BranchControl>
				<TreeView.BranchContent>
					{#each node.children as child, i (child.id)}
						{@render treeNode(child, [...indexPath, i])}
					{/each}
				</TreeView.BranchContent>
			</TreeView.Branch>
		{:else}
			{@const LeafIcon = iconFor(node.kind)}
			<TreeView.Item>
				<LeafIcon class="size-4 opacity-70" />
				<span>{node.name}</span>
			</TreeView.Item>
		{/if}
	</TreeView.NodeProvider>
{/snippet}
