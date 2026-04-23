<script lang="ts">
	import { Avatar, Tabs, Tooltip } from '@skeletonlabs/skeleton-svelte';
	import {
		MapPinIcon,
		CalendarIcon,
		ClockIcon,
		LanguagesIcon,
		PencilIcon,
		ShareIcon
	} from '@lucide/svelte';

	let activityTab = $state('posts');

	const skills = ['SvelteKit', 'Go', 'TypeScript', 'Tailwind CSS', 'PostgreSQL', 'Docker', 'AWS'];

	const aboutDetails = [
		{ icon: MapPinIcon, label: 'Location', value: 'San Francisco, CA' },
		{ icon: CalendarIcon, label: 'Joined', value: 'March 2024' },
		{ icon: ClockIcon, label: 'Timezone', value: 'Pacific Time (PT)' },
		{ icon: LanguagesIcon, label: 'Language', value: 'English' }
	];

	const posts = [
		{
			title: 'Building Reactive UIs with Svelte 5 Runes',
			date: 'Mar 28, 2026',
			excerpt:
				'An exploration of how $state, $derived, and $effect simplify reactive programming in modern Svelte applications.'
		},
		{
			title: 'Why We Switched to PocketBase',
			date: 'Mar 15, 2026',
			excerpt:
				'Our team evaluated several BaaS options and settled on PocketBase for its simplicity and Go-based extensibility.'
		},
		{
			title: 'WebSocket Patterns for Real-Time Apps',
			date: 'Feb 20, 2026',
			excerpt:
				'A deep dive into hub-and-spoke WebSocket architectures with room-based message routing.'
		}
	];

	const activityLog = [
		{ date: 'Apr 2', action: 'Deployed production release v3.1.0' },
		{ date: 'Apr 1', action: 'Reviewed pull request #128 — API rate limiting' },
		{ date: 'Mar 30', action: 'Created project "Mobile App Beta"' },
		{ date: 'Mar 28', action: 'Published blog post about Svelte 5 runes' },
		{ date: 'Mar 25', action: 'Invited 3 new team members' }
	];

	const connections = [
		{
			avatar: 'https://i.pravatar.cc/80?img=1',
			name: 'Alex Chen',
			role: 'Engineer',
			rolePreset: 'preset-tonal-primary'
		},
		{
			avatar: 'https://i.pravatar.cc/80?img=2',
			name: 'Sarah Kim',
			role: 'Designer',
			rolePreset: 'preset-tonal-secondary'
		},
		{
			avatar: 'https://i.pravatar.cc/80?img=3',
			name: 'Marcus Johnson',
			role: 'DevOps',
			rolePreset: 'preset-tonal-tertiary'
		},
		{
			avatar: 'https://i.pravatar.cc/80?img=4',
			name: 'Emily Reeves',
			role: 'Manager',
			rolePreset: 'preset-tonal-success'
		},
		{
			avatar: 'https://i.pravatar.cc/80?img=8',
			name: 'James Park',
			role: 'Engineer',
			rolePreset: 'preset-tonal-primary'
		}
	];
</script>

<h1 class="mb-6 h2">Profile</h1>

<div class="max-w-4xl space-y-6">
	<!-- Profile Header Card -->
	<div class="overflow-hidden card">
		<!-- Banner -->
		<div class="h-28 bg-primary-300-700"></div>

		<!-- Profile Info -->
		<div class="relative px-6 pb-6">
			<div class="-mt-12 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
				<div class="flex items-end gap-4">
					<Avatar class="size-24 ring-4 ring-surface-50-950">
						<Avatar.Image src="https://i.pravatar.cc/160?img=5" />
						<Avatar.Fallback>JD</Avatar.Fallback>
					</Avatar>
					<div class="pb-1">
						<h2 class="h3">Jane Doe</h2>
						<p class="text-sm opacity-70">jane.doe@example.com</p>
					</div>
				</div>
				<div class="flex gap-2">
					<button class="btn preset-filled btn-sm">
						<PencilIcon class="size-4" />
						<span>Edit Profile</span>
					</button>
					<button class="btn preset-tonal btn-sm">
						<ShareIcon class="size-4" />
						<span>Share</span>
					</button>
				</div>
			</div>

			<!-- Bio & Badges -->
			<p class="mt-4 text-sm opacity-80">
				Full-stack developer who enjoys building things with SvelteKit and Go. Passionate about
				real-time applications and developer tooling.
			</p>
			<div class="mt-3 flex flex-wrap gap-2">
				<span class="badge preset-filled-primary-500">Admin</span>
				<span class="badge preset-tonal-secondary">Developer</span>
				<span class="preset-outlined-surface badge">Team Lead</span>
			</div>
		</div>
	</div>

	<!-- Two-column: About + Activity -->
	<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
		<!-- About Card -->
		<div class="space-y-6 card p-6">
			<h3 class="h4">About</h3>

			<div class="space-y-4">
				{#each aboutDetails as detail (detail.label)}
					<div class="flex items-center gap-3">
						<detail.icon class="size-4 text-surface-400-600" />
						<span class="text-sm opacity-70">{detail.label}</span>
						<span class="ml-auto text-sm font-semibold">{detail.value}</span>
					</div>
				{/each}
			</div>

			<hr class="hr" />

			<div class="space-y-3">
				<h4 class="text-sm font-semibold">Skills</h4>
				<div class="flex flex-wrap gap-2">
					{#each skills as skill (skill)}
						<span class="chip preset-outlined-primary-500">{skill}</span>
					{/each}
				</div>
			</div>
		</div>

		<!-- Activity Card -->
		<div class="space-y-4 card p-6">
			<Tabs value={activityTab} onValueChange={(e) => (activityTab = e.value)}>
				<Tabs.List>
					<Tabs.Trigger value="posts">Posts</Tabs.Trigger>
					<Tabs.Trigger value="activity">Activity</Tabs.Trigger>
					<Tabs.Indicator />
				</Tabs.List>

				<Tabs.Content value="posts">
					<div class="space-y-4 pt-2">
						{#each posts as post (post.title)}
							<div class="space-y-1">
								<h4 class="cursor-pointer text-sm font-semibold hover:text-primary-500">
									{post.title}
								</h4>
								<p class="text-xs opacity-50">{post.date}</p>
								<p class="text-sm opacity-70">{post.excerpt}</p>
							</div>
							{#if post !== posts[posts.length - 1]}
								<hr class="hr" />
							{/if}
						{/each}
					</div>
				</Tabs.Content>

				<Tabs.Content value="activity">
					<div class="space-y-0 pt-2">
						{#each activityLog as entry (entry.date + entry.action)}
							<div class="flex gap-3 border-l-2 border-surface-300-700 py-3 pl-4">
								<span class="text-xs font-semibold whitespace-nowrap opacity-50">{entry.date}</span>
								<p class="text-sm">{entry.action}</p>
							</div>
						{/each}
					</div>
				</Tabs.Content>
			</Tabs>
		</div>
	</div>

	<!-- Connections Card -->
	<div class="space-y-4 card p-6">
		<h3 class="h4">Connections</h3>
		<div class="flex gap-4 overflow-x-auto pb-2">
			{#each connections as person (person.name)}
				<Tooltip>
					<Tooltip.Trigger>
						<div class="flex min-w-[120px] flex-col items-center gap-2 text-center">
							<Avatar class="size-14">
								<Avatar.Image src={person.avatar} />
								<Avatar.Fallback>{person.name.charAt(0)}</Avatar.Fallback>
							</Avatar>
							<p class="text-sm font-semibold">{person.name.split(' ')[0]}</p>
							<span class="badge {person.rolePreset} text-xs">{person.role}</span>
						</div>
					</Tooltip.Trigger>
					<Tooltip.Positioner>
						<Tooltip.Content class="z-50 card preset-filled-surface-100-900 px-3 py-2 text-sm">
							<Tooltip.Arrow>
								<Tooltip.ArrowTip />
							</Tooltip.Arrow>
							{person.name} — {person.role}
						</Tooltip.Content>
					</Tooltip.Positioner>
				</Tooltip>
			{/each}
		</div>
	</div>
</div>
