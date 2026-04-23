<script lang="ts">
	import { Avatar } from '@skeletonlabs/skeleton-svelte';
	import { SearchIcon, MailIcon, MessageSquareIcon, UserPlusIcon } from '@lucide/svelte';

	interface Member {
		id: number;
		name: string;
		role: string;
		department: 'Engineering' | 'Design' | 'Product' | 'Operations';
		email: string;
		avatar: string;
		status: 'online' | 'away' | 'offline';
	}

	const members: Member[] = [
		{
			id: 1,
			name: 'Alex Chen',
			role: 'Senior Engineer',
			department: 'Engineering',
			email: 'alex@example.com',
			avatar: 'https://i.pravatar.cc/96?img=1',
			status: 'online'
		},
		{
			id: 2,
			name: 'Sarah Kim',
			role: 'Product Designer',
			department: 'Design',
			email: 'sarah@example.com',
			avatar: 'https://i.pravatar.cc/96?img=2',
			status: 'online'
		},
		{
			id: 3,
			name: 'Marcus Johnson',
			role: 'DevOps Lead',
			department: 'Engineering',
			email: 'marcus@example.com',
			avatar: 'https://i.pravatar.cc/96?img=3',
			status: 'away'
		},
		{
			id: 4,
			name: 'Emily Reeves',
			role: 'Engineering Manager',
			department: 'Engineering',
			email: 'emily@example.com',
			avatar: 'https://i.pravatar.cc/96?img=4',
			status: 'online'
		},
		{
			id: 5,
			name: 'James Park',
			role: 'Backend Engineer',
			department: 'Engineering',
			email: 'james@example.com',
			avatar: 'https://i.pravatar.cc/96?img=8',
			status: 'offline'
		},
		{
			id: 6,
			name: 'Olivia Garcia',
			role: 'Product Manager',
			department: 'Product',
			email: 'olivia@example.com',
			avatar: 'https://i.pravatar.cc/96?img=5',
			status: 'online'
		},
		{
			id: 7,
			name: 'Noah Patel',
			role: 'UX Researcher',
			department: 'Design',
			email: 'noah@example.com',
			avatar: 'https://i.pravatar.cc/96?img=6',
			status: 'away'
		},
		{
			id: 8,
			name: 'Ava Nguyen',
			role: 'Operations Lead',
			department: 'Operations',
			email: 'ava@example.com',
			avatar: 'https://i.pravatar.cc/96?img=7',
			status: 'offline'
		},
		{
			id: 9,
			name: 'Liam Brown',
			role: 'Frontend Engineer',
			department: 'Engineering',
			email: 'liam@example.com',
			avatar: 'https://i.pravatar.cc/96?img=11',
			status: 'online'
		}
	];

	const departments: Array<Member['department'] | 'All'> = [
		'All',
		'Engineering',
		'Design',
		'Product',
		'Operations'
	];
	const statusColors: Record<Member['status'], string> = {
		online: 'bg-success-500',
		away: 'bg-warning-500',
		offline: 'bg-surface-400-600'
	};

	let query = $state('');
	let activeDept = $state<(typeof departments)[number]>('All');

	let filtered = $derived(
		members.filter((m) => {
			const matchesDept = activeDept === 'All' || m.department === activeDept;
			const q = query.trim().toLowerCase();
			const matchesQuery =
				!q || m.name.toLowerCase().includes(q) || m.role.toLowerCase().includes(q);
			return matchesDept && matchesQuery;
		})
	);
</script>

<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<div>
		<h1 class="h2">Team</h1>
		<p class="text-sm opacity-70">{filtered.length} of {members.length} members</p>
	</div>
	<button class="btn preset-filled">
		<UserPlusIcon class="size-4" />
		<span>Invite Member</span>
	</button>
</div>

<div class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center">
	<div class="input-group max-w-sm grid-cols-[auto_1fr]">
		<div class="input-group-cell">
			<SearchIcon class="size-4" />
		</div>
		<input type="search" placeholder="Search by name or role..." bind:value={query} />
	</div>
	<div class="flex flex-wrap gap-2">
		{#each departments as dept (dept)}
			<button
				class="chip {activeDept === dept
					? 'preset-filled-primary-500'
					: 'preset-outlined-surface-200-800'}"
				onclick={() => (activeDept = dept)}
			>
				{dept}
			</button>
		{/each}
	</div>
</div>

<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
	{#each filtered as member (member.id)}
		<div class="space-y-4 card p-6 card-hover">
			<div class="flex items-start gap-4">
				<div class="relative">
					<Avatar class="size-14">
						<Avatar.Image src={member.avatar} />
						<Avatar.Fallback>{member.name.charAt(0)}</Avatar.Fallback>
					</Avatar>
					<span
						class="absolute right-0 bottom-0 size-3 rounded-full ring-2 ring-surface-50-950 {statusColors[
							member.status
						]}"
					></span>
				</div>
				<div class="min-w-0 flex-1">
					<h3 class="font-semibold">{member.name}</h3>
					<p class="text-sm opacity-70">{member.role}</p>
					<span class="mt-2 badge inline-block preset-tonal-primary text-xs"
						>{member.department}</span
					>
				</div>
			</div>

			<div class="flex gap-2">
				<button class="btn flex-1 preset-tonal btn-sm">
					<MessageSquareIcon class="size-4" />
					<span>Message</span>
				</button>
				<button class="btn-icon btn-icon-sm preset-tonal" aria-label="Email {member.email}">
					<MailIcon class="size-4" />
				</button>
			</div>
		</div>
	{/each}
	{#if filtered.length === 0}
		<div class="col-span-full py-12 text-center opacity-60">
			<p class="text-sm">No members match your filters.</p>
		</div>
	{/if}
</div>
