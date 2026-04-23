<script lang="ts">
	import { Avatar } from '@skeletonlabs/skeleton-svelte';
	import {
		PlusIcon,
		MoreHorizontalIcon,
		MessageSquareIcon,
		PaperclipIcon,
		CheckCircle2Icon
	} from '@lucide/svelte';

	interface Card {
		id: number;
		title: string;
		description: string;
		tags: { label: string; preset: string }[];
		assignees: string[];
		comments: number;
		attachments: number;
		done: number;
		total: number;
	}

	interface Column {
		id: string;
		title: string;
		accent: string;
		cards: Card[];
	}

	const columns: Column[] = [
		{
			id: 'todo',
			title: 'To Do',
			accent: 'bg-primary-500',
			cards: [
				{
					id: 1,
					title: 'Design landing hero section',
					description: 'Sketch 3 variations for the homepage hero above the fold.',
					tags: [{ label: 'Design', preset: 'preset-tonal-primary' }],
					assignees: ['https://i.pravatar.cc/40?img=1', 'https://i.pravatar.cc/40?img=2'],
					comments: 4,
					attachments: 2,
					done: 0,
					total: 5
				},
				{
					id: 2,
					title: 'Audit 404 and error pages',
					description: 'Check all error states for consistent styling and helpful copy.',
					tags: [
						{ label: 'QA', preset: 'preset-tonal-secondary' },
						{ label: 'Polish', preset: 'preset-tonal-surface' }
					],
					assignees: ['https://i.pravatar.cc/40?img=3'],
					comments: 1,
					attachments: 0,
					done: 1,
					total: 4
				},
				{
					id: 3,
					title: 'Draft release notes for v2.5',
					description: 'Summarize key features and breaking changes.',
					tags: [{ label: 'Docs', preset: 'preset-tonal-success' }],
					assignees: ['https://i.pravatar.cc/40?img=4'],
					comments: 0,
					attachments: 1,
					done: 0,
					total: 3
				}
			]
		},
		{
			id: 'progress',
			title: 'In Progress',
			accent: 'bg-warning-500',
			cards: [
				{
					id: 4,
					title: 'Implement JWT refresh flow',
					description: 'Rotate refresh tokens on every access exchange and add revocation list.',
					tags: [
						{ label: 'Backend', preset: 'preset-tonal-tertiary' },
						{ label: 'Security', preset: 'preset-tonal-warning' }
					],
					assignees: ['https://i.pravatar.cc/40?img=5', 'https://i.pravatar.cc/40?img=6'],
					comments: 8,
					attachments: 3,
					done: 3,
					total: 6
				},
				{
					id: 5,
					title: 'Wire up billing webhook handler',
					description: 'Listen for invoice.paid and subscription.updated events.',
					tags: [{ label: 'Backend', preset: 'preset-tonal-tertiary' }],
					assignees: ['https://i.pravatar.cc/40?img=7'],
					comments: 3,
					attachments: 0,
					done: 2,
					total: 4
				}
			]
		},
		{
			id: 'done',
			title: 'Done',
			accent: 'bg-success-500',
			cards: [
				{
					id: 6,
					title: 'Migrate CI to GitHub Actions',
					description: 'Replaced legacy Jenkins jobs with matrix workflows.',
					tags: [{ label: 'DevOps', preset: 'preset-tonal-success' }],
					assignees: ['https://i.pravatar.cc/40?img=8'],
					comments: 12,
					attachments: 5,
					done: 7,
					total: 7
				},
				{
					id: 7,
					title: 'Set up dependabot + renovate',
					description: 'Auto-merge minor and patch dependency bumps on green CI.',
					tags: [{ label: 'DevOps', preset: 'preset-tonal-success' }],
					assignees: ['https://i.pravatar.cc/40?img=9'],
					comments: 2,
					attachments: 0,
					done: 3,
					total: 3
				},
				{
					id: 8,
					title: 'Ship dark mode toggle',
					description: 'Persist preference in localStorage and respect prefers-color-scheme.',
					tags: [
						{ label: 'Frontend', preset: 'preset-tonal-primary' },
						{ label: 'UX', preset: 'preset-tonal-secondary' }
					],
					assignees: ['https://i.pravatar.cc/40?img=10', 'https://i.pravatar.cc/40?img=11'],
					comments: 5,
					attachments: 1,
					done: 4,
					total: 4
				}
			]
		}
	];
</script>

<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<div>
		<h1 class="h2">Kanban Board</h1>
		<p class="text-sm opacity-70">Drag-free mock board — cards grouped by status.</p>
	</div>
	<button class="btn preset-filled">
		<PlusIcon class="size-4" />
		<span>New Task</span>
	</button>
</div>

<div class="grid grid-cols-1 gap-4 md:grid-cols-3">
	{#each columns as column (column.id)}
		<div class="flex flex-col gap-3">
			<div class="flex items-center justify-between px-1">
				<div class="flex items-center gap-2">
					<span class="size-2 rounded-full {column.accent}"></span>
					<h2 class="h5">{column.title}</h2>
					<span class="badge preset-tonal-surface text-xs">{column.cards.length}</span>
				</div>
				<button class="btn-icon btn-icon-sm preset-tonal" aria-label="More">
					<MoreHorizontalIcon class="size-4" />
				</button>
			</div>

			<div class="flex flex-col gap-3">
				{#each column.cards as card (card.id)}
					<div class="space-y-3 card p-4 card-hover">
						<div class="flex flex-wrap gap-1">
							{#each card.tags as tag (tag.label)}
								<span class="badge {tag.preset} text-[10px]">{tag.label}</span>
							{/each}
						</div>
						<div>
							<h3 class="text-sm font-semibold">{card.title}</h3>
							<p class="mt-1 text-xs opacity-70">{card.description}</p>
						</div>
						<div class="flex items-center justify-between">
							<div class="flex -space-x-2">
								{#each card.assignees as src (src)}
									<Avatar class="size-6 ring-2 ring-surface-50-950">
										<Avatar.Image {src} />
										<Avatar.Fallback>U</Avatar.Fallback>
									</Avatar>
								{/each}
							</div>
							<div class="flex items-center gap-3 text-xs opacity-60">
								<span class="flex items-center gap-1">
									<CheckCircle2Icon class="size-3" />
									{card.done}/{card.total}
								</span>
								<span class="flex items-center gap-1">
									<MessageSquareIcon class="size-3" />
									{card.comments}
								</span>
								<span class="flex items-center gap-1">
									<PaperclipIcon class="size-3" />
									{card.attachments}
								</span>
							</div>
						</div>
					</div>
				{/each}

				<button class="btn justify-center preset-tonal-surface text-sm">
					<PlusIcon class="size-4" />
					Add card
				</button>
			</div>
		</div>
	{/each}
</div>
