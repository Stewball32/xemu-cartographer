<script lang="ts">
	import { SegmentedControl, Progress, Accordion, Avatar } from '@skeletonlabs/skeleton-svelte';
	import {
		UsersIcon,
		ActivityIcon,
		FolderIcon,
		TrendingUpIcon,
		PlusIcon,
		UserPlusIcon,
		DownloadIcon,
		ChevronDownIcon
	} from '@lucide/svelte';
	import type { Component } from 'svelte';

	interface StatCard {
		label: string;
		value: string;
		trend: string;
		trendUp: boolean;
		progress: number;
		icon: Component<{ class?: string }>;
		color: string;
	}

	let period = $state('week');

	const statsByPeriod: Record<string, StatCard[]> = {
		today: [
			{
				label: 'Users',
				value: '48',
				trend: '+3%',
				trendUp: true,
				progress: 48,
				icon: UsersIcon,
				color: 'primary'
			},
			{
				label: 'Activity',
				value: '312',
				trend: '+8%',
				trendUp: true,
				progress: 65,
				icon: ActivityIcon,
				color: 'secondary'
			},
			{
				label: 'Projects',
				value: '7',
				trend: '0%',
				trendUp: true,
				progress: 17,
				icon: FolderIcon,
				color: 'tertiary'
			},
			{
				label: 'Growth',
				value: '+2.1%',
				trend: '+2.1%',
				trendUp: true,
				progress: 21,
				icon: TrendingUpIcon,
				color: 'success'
			}
		],
		week: [
			{
				label: 'Users',
				value: '1,234',
				trend: '+12%',
				trendUp: true,
				progress: 78,
				icon: UsersIcon,
				color: 'primary'
			},
			{
				label: 'Activity',
				value: '5,678',
				trend: '+5%',
				trendUp: true,
				progress: 85,
				icon: ActivityIcon,
				color: 'secondary'
			},
			{
				label: 'Projects',
				value: '42',
				trend: '-2%',
				trendUp: false,
				progress: 60,
				icon: FolderIcon,
				color: 'tertiary'
			},
			{
				label: 'Growth',
				value: '+8.3%',
				trend: '+8.3%',
				trendUp: true,
				progress: 83,
				icon: TrendingUpIcon,
				color: 'success'
			}
		],
		month: [
			{
				label: 'Users',
				value: '4,891',
				trend: '+18%',
				trendUp: true,
				progress: 92,
				icon: UsersIcon,
				color: 'primary'
			},
			{
				label: 'Activity',
				value: '23,456',
				trend: '+15%',
				trendUp: true,
				progress: 94,
				icon: ActivityIcon,
				color: 'secondary'
			},
			{
				label: 'Projects',
				value: '56',
				trend: '+7%',
				trendUp: true,
				progress: 75,
				icon: FolderIcon,
				color: 'tertiary'
			},
			{
				label: 'Growth',
				value: '+12.7%',
				trend: '+12.7%',
				trendUp: true,
				progress: 95,
				icon: TrendingUpIcon,
				color: 'success'
			}
		]
	};

	let stats = $derived(statsByPeriod[period] ?? statsByPeriod.week);

	const activityRows = [
		{
			avatar: 'https://i.pravatar.cc/40?img=1',
			name: 'Alex Chen',
			action: 'Deployed v2.4.1',
			date: 'Today, 2:34 PM',
			status: 'Success',
			statusPreset: 'preset-filled-success-500'
		},
		{
			avatar: 'https://i.pravatar.cc/40?img=2',
			name: 'Sarah Kim',
			action: 'Opened pull request #42',
			date: 'Today, 1:15 PM',
			status: 'Pending',
			statusPreset: 'preset-filled-warning-500'
		},
		{
			avatar: 'https://i.pravatar.cc/40?img=3',
			name: 'Marcus Johnson',
			action: 'Updated user permissions',
			date: 'Today, 11:20 AM',
			status: 'Complete',
			statusPreset: 'preset-filled-primary-500'
		},
		{
			avatar: 'https://i.pravatar.cc/40?img=4',
			name: 'Emily Reeves',
			action: 'Resolved issue #87',
			date: 'Yesterday, 4:50 PM',
			status: 'Closed',
			statusPreset: 'preset-filled-surface-400-600'
		},
		{
			avatar: 'https://i.pravatar.cc/40?img=5',
			name: 'James Park',
			action: 'Added new API endpoint',
			date: 'Yesterday, 3:10 PM',
			status: 'Success',
			statusPreset: 'preset-filled-success-500'
		}
	];

	const quickActions = [
		{
			icon: UserPlusIcon,
			title: 'Invite Team Member',
			description: 'Send an invite link to a new collaborator',
			btnLabel: 'Invite'
		},
		{
			icon: PlusIcon,
			title: 'Create Project',
			description: 'Start a new project from a template',
			btnLabel: 'Create'
		},
		{
			icon: DownloadIcon,
			title: 'Export Data',
			description: 'Download reports in CSV or JSON format',
			btnLabel: 'Export'
		}
	];

	const projects = [
		{
			name: 'Website Redesign',
			description: 'Complete overhaul of the marketing site with new brand guidelines.',
			progress: 75,
			tags: ['SvelteKit', 'Tailwind', 'Figma']
		},
		{
			name: 'API v3 Migration',
			description: 'Migrating all endpoints from REST v2 to the new GraphQL layer.',
			progress: 45,
			tags: ['Go', 'GraphQL', 'Docker']
		},
		{
			name: 'Mobile App Beta',
			description: 'Cross-platform mobile application targeting iOS and Android.',
			progress: 30,
			tags: ['React Native', 'TypeScript', 'Firebase']
		},
		{
			name: 'Security Audit',
			description: 'Quarterly security review and penetration testing of all services.',
			progress: 90,
			tags: ['Security', 'Compliance', 'DevOps']
		}
	];
</script>

<!-- Header -->
<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<h1 class="h2">Dashboard</h1>
	<SegmentedControl value={period} onValueChange={(e) => (period = e.value ?? 'week')}>
		<SegmentedControl.Control>
			<SegmentedControl.Indicator />
			<SegmentedControl.Item value="today">
				<SegmentedControl.ItemText>Today</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
			<SegmentedControl.Item value="week">
				<SegmentedControl.ItemText>This Week</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
			<SegmentedControl.Item value="month">
				<SegmentedControl.ItemText>This Month</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
		</SegmentedControl.Control>
	</SegmentedControl>
</div>

<!-- Stat Cards -->
<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
	{#each stats as stat (stat.label)}
		<div class="space-y-3 card p-6 card-hover">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-3">
					<stat.icon class="size-8 text-{stat.color}-500" />
					<h2 class="h4">{stat.label}</h2>
				</div>
				<span
					class="badge {stat.trendUp
						? 'preset-filled-success-500'
						: 'preset-filled-error-500'} text-xs">{stat.trend}</span
				>
			</div>
			<p class="text-3xl font-bold">{stat.value}</p>
			<Progress value={stat.progress} max={100}>
				<Progress.Track class="h-1.5 preset-outlined-surface-200-800">
					<Progress.Range class="bg-{stat.color}-500" />
				</Progress.Track>
			</Progress>
		</div>
	{/each}
</div>

<!-- Middle Row: Activity Table + Quick Actions -->
<div class="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
	<!-- Recent Activity -->
	<div class="space-y-4 card p-6">
		<h2 class="h4">Recent Activity</h2>
		<div class="table-wrap">
			<table class="table">
				<thead>
					<tr>
						<th>User</th>
						<th>Action</th>
						<th class="hidden md:table-cell">Date</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody class="[&>tr]:hover:preset-tonal-primary">
					{#each activityRows as row (row.name + row.action)}
						<tr>
							<td>
								<div class="flex items-center gap-2">
									<Avatar class="size-8">
										<Avatar.Image src={row.avatar} />
										<Avatar.Fallback>{row.name.charAt(0)}</Avatar.Fallback>
									</Avatar>
									<span class="text-sm">{row.name}</span>
								</div>
							</td>
							<td class="text-sm">{row.action}</td>
							<td class="hidden text-sm opacity-70 md:table-cell">{row.date}</td>
							<td><span class="badge {row.statusPreset} text-xs">{row.status}</span></td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>

	<!-- Quick Actions -->
	<div class="space-y-4 card p-6">
		<h2 class="h4">Quick Actions</h2>
		<div class="space-y-3">
			{#each quickActions as action (action.title)}
				<div class="card p-4 card-hover">
					<div class="flex items-center gap-4">
						<div class="flex items-center justify-center rounded-full bg-surface-200-800 p-2">
							<action.icon class="size-5 text-primary-500" />
						</div>
						<div class="flex-1">
							<p class="font-semibold">{action.title}</p>
							<p class="text-sm opacity-70">{action.description}</p>
						</div>
						<button class="btn preset-tonal-primary btn-sm">{action.btnLabel}</button>
					</div>
				</div>
			{/each}
		</div>
	</div>
</div>

<!-- Project Status -->
<div class="mt-6 space-y-4 card p-6">
	<h2 class="h4">Project Status</h2>
	<Accordion multiple collapsible>
		{#each projects as project (project.name)}
			<Accordion.Item value={project.name}>
				<Accordion.ItemTrigger>
					<span>{project.name}</span>
					<Accordion.ItemIndicator>
						<ChevronDownIcon class="size-4" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent>
					<div class="space-y-3 py-2">
						<p class="text-sm opacity-70">{project.description}</p>
						<div class="flex items-center gap-3">
							<Progress value={project.progress} max={100} class="flex-1">
								<Progress.Track class="h-2 preset-outlined-surface-200-800">
									<Progress.Range
										class={project.progress >= 75
											? 'bg-success-500'
											: project.progress >= 50
												? 'bg-warning-500'
												: 'bg-primary-500'}
									/>
								</Progress.Track>
							</Progress>
							<span class="text-sm font-semibold">{project.progress}%</span>
						</div>
						<div class="flex flex-wrap gap-2">
							{#each project.tags as tag (tag)}
								<span class="badge preset-outlined-primary-500 text-xs">{tag}</span>
							{/each}
						</div>
					</div>
				</Accordion.ItemContent>
			</Accordion.Item>
		{/each}
	</Accordion>
</div>
