<script lang="ts">
	import { Pagination, Avatar } from '@skeletonlabs/skeleton-svelte';
	import {
		SearchIcon,
		ChevronUpIcon,
		ChevronDownIcon,
		ChevronsUpDownIcon,
		ChevronLeftIcon,
		ChevronRightIcon
	} from '@lucide/svelte';

	interface User {
		id: number;
		name: string;
		email: string;
		role: 'Admin' | 'Editor' | 'Viewer';
		status: 'Active' | 'Invited' | 'Disabled';
		joined: string;
		avatar: string;
	}

	const rolePresets: Record<User['role'], string> = {
		Admin: 'preset-filled-primary-500',
		Editor: 'preset-tonal-secondary',
		Viewer: 'preset-tonal-surface'
	};

	const statusPresets: Record<User['status'], string> = {
		Active: 'preset-filled-success-500',
		Invited: 'preset-filled-warning-500',
		Disabled: 'preset-filled-surface-400-600'
	};

	const allUsers: User[] = Array.from({ length: 47 }, (_, i) => {
		const roles: User['role'][] = ['Admin', 'Editor', 'Viewer'];
		const statuses: User['status'][] = ['Active', 'Invited', 'Disabled'];
		const firstNames = [
			'Alex',
			'Sarah',
			'Marcus',
			'Emily',
			'James',
			'Olivia',
			'Noah',
			'Ava',
			'Liam',
			'Mia'
		];
		const lastNames = [
			'Chen',
			'Kim',
			'Johnson',
			'Reeves',
			'Park',
			'Garcia',
			'Patel',
			'Nguyen',
			'Brown',
			'Silva'
		];
		const first = firstNames[i % firstNames.length];
		const last = lastNames[(i * 3) % lastNames.length];
		return {
			id: i + 1,
			name: `${first} ${last}`,
			email: `${first.toLowerCase()}.${last.toLowerCase()}@example.com`,
			role: roles[i % 3],
			status: statuses[i % 3],
			joined: `2024-${String((i % 12) + 1).padStart(2, '0')}-${String((i % 27) + 1).padStart(2, '0')}`,
			avatar: `https://i.pravatar.cc/40?img=${(i % 70) + 1}`
		};
	});

	type SortKey = 'name' | 'role' | 'status' | 'joined';
	let sortKey = $state<SortKey>('name');
	let sortDir = $state<'asc' | 'desc'>('asc');
	let query = $state('');
	let page = $state(1);
	const pageSize = 8;

	function toggleSort(key: SortKey) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'asc';
		}
	}

	let filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		const base = q
			? allUsers.filter(
					(u) => u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q)
				)
			: allUsers;
		const sorted = [...base].sort((a, b) => {
			const av = a[sortKey];
			const bv = b[sortKey];
			return sortDir === 'asc'
				? String(av).localeCompare(String(bv))
				: String(bv).localeCompare(String(av));
		});
		return sorted;
	});

	let pageRows = $derived(filtered.slice((page - 1) * pageSize, page * pageSize));
</script>

<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<div>
		<h1 class="h2">Data Table</h1>
		<p class="text-sm opacity-70">Sortable, searchable, paginated user list.</p>
	</div>
	<div class="input-group flex max-w-sm grid-cols-[auto_1fr] items-center gap-2">
		<div class="input-group-cell">
			<SearchIcon class="size-4" />
		</div>
		<input
			type="search"
			placeholder="Search users..."
			bind:value={query}
			oninput={() => (page = 1)}
		/>
	</div>
</div>

<div class="card p-0">
	<div class="table-wrap">
		<table class="table">
			<thead>
				<tr>
					<th>
						<button class="flex items-center gap-1" onclick={() => toggleSort('name')}>
							User
							{#if sortKey === 'name'}
								{#if sortDir === 'asc'}
									<ChevronUpIcon class="size-4" />
								{:else}
									<ChevronDownIcon class="size-4" />
								{/if}
							{:else}
								<ChevronsUpDownIcon class="size-4 opacity-50" />
							{/if}
						</button>
					</th>
					<th class="hidden md:table-cell">Email</th>
					<th>
						<button class="flex items-center gap-1" onclick={() => toggleSort('role')}>
							Role
							{#if sortKey === 'role'}
								{#if sortDir === 'asc'}
									<ChevronUpIcon class="size-4" />
								{:else}
									<ChevronDownIcon class="size-4" />
								{/if}
							{:else}
								<ChevronsUpDownIcon class="size-4 opacity-50" />
							{/if}
						</button>
					</th>
					<th>
						<button class="flex items-center gap-1" onclick={() => toggleSort('status')}>
							Status
							{#if sortKey === 'status'}
								{#if sortDir === 'asc'}
									<ChevronUpIcon class="size-4" />
								{:else}
									<ChevronDownIcon class="size-4" />
								{/if}
							{:else}
								<ChevronsUpDownIcon class="size-4 opacity-50" />
							{/if}
						</button>
					</th>
					<th class="hidden lg:table-cell">
						<button class="flex items-center gap-1" onclick={() => toggleSort('joined')}>
							Joined
							{#if sortKey === 'joined'}
								{#if sortDir === 'asc'}
									<ChevronUpIcon class="size-4" />
								{:else}
									<ChevronDownIcon class="size-4" />
								{/if}
							{:else}
								<ChevronsUpDownIcon class="size-4 opacity-50" />
							{/if}
						</button>
					</th>
				</tr>
			</thead>
			<tbody class="[&>tr]:hover:preset-tonal-primary">
				{#each pageRows as user (user.id)}
					<tr>
						<td>
							<div class="flex items-center gap-2">
								<Avatar class="size-8">
									<Avatar.Image src={user.avatar} />
									<Avatar.Fallback>{user.name.charAt(0)}</Avatar.Fallback>
								</Avatar>
								<span class="text-sm font-semibold">{user.name}</span>
							</div>
						</td>
						<td class="hidden text-sm opacity-70 md:table-cell">{user.email}</td>
						<td><span class="badge {rolePresets[user.role]} text-xs">{user.role}</span></td>
						<td><span class="badge {statusPresets[user.status]} text-xs">{user.status}</span></td>
						<td class="hidden text-sm opacity-70 lg:table-cell">{user.joined}</td>
					</tr>
				{/each}
				{#if pageRows.length === 0}
					<tr>
						<td colspan="5" class="text-center text-sm opacity-60">No matches found.</td>
					</tr>
				{/if}
			</tbody>
		</table>
	</div>

	<div class="flex items-center justify-between gap-4 border-t border-surface-200-800 p-4">
		<p class="text-sm opacity-70">
			Showing {filtered.length === 0 ? 0 : (page - 1) * pageSize + 1}–{Math.min(
				page * pageSize,
				filtered.length
			)} of {filtered.length}
		</p>
		<Pagination
			count={filtered.length}
			{pageSize}
			{page}
			siblingCount={1}
			onPageChange={(e) => (page = e.page)}
		>
			<Pagination.PrevTrigger class="btn-icon preset-tonal">
				<ChevronLeftIcon class="size-4" />
			</Pagination.PrevTrigger>
			<Pagination.Context>
				{#snippet children(pagination)}
					{#each pagination().pages as p, i (i)}
						{#if p.type === 'page'}
							<Pagination.Item
								{...p}
								class="btn-icon preset-tonal data-selected:preset-filled-primary-500"
							>
								{p.value}
							</Pagination.Item>
						{:else}
							<Pagination.Ellipsis index={i} class="px-1 opacity-60">…</Pagination.Ellipsis>
						{/if}
					{/each}
				{/snippet}
			</Pagination.Context>
			<Pagination.NextTrigger class="btn-icon preset-tonal">
				<ChevronRightIcon class="size-4" />
			</Pagination.NextTrigger>
		</Pagination>
	</div>
</div>
