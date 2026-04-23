<script lang="ts">
	import { Tabs, Avatar } from '@skeletonlabs/skeleton-svelte';
	import {
		MessageSquareIcon,
		GitPullRequestIcon,
		AlertCircleIcon,
		UserPlusIcon,
		CheckCheckIcon,
		TrashIcon,
		BellIcon
	} from '@lucide/svelte';
	import type { Component } from 'svelte';

	type NotifKind = 'mention' | 'pr' | 'alert' | 'invite';

	interface Notification {
		id: number;
		kind: NotifKind;
		actor: string;
		avatar: string;
		title: string;
		body: string;
		time: string;
		read: boolean;
	}

	const kindMeta: Record<
		NotifKind,
		{ icon: Component<{ class?: string }>; preset: string; label: string }
	> = {
		mention: { icon: MessageSquareIcon, preset: 'preset-tonal-primary', label: 'Mention' },
		pr: { icon: GitPullRequestIcon, preset: 'preset-tonal-secondary', label: 'PR' },
		alert: { icon: AlertCircleIcon, preset: 'preset-tonal-warning', label: 'Alert' },
		invite: { icon: UserPlusIcon, preset: 'preset-tonal-success', label: 'Invite' }
	};

	let notifications = $state<Notification[]>([
		{
			id: 1,
			kind: 'mention',
			actor: 'Alex Chen',
			avatar: 'https://i.pravatar.cc/64?img=1',
			title: 'mentioned you in #engineering',
			body: '@you — can you take a look at the retry logic before we ship?',
			time: '2m ago',
			read: false
		},
		{
			id: 2,
			kind: 'pr',
			actor: 'Sarah Kim',
			avatar: 'https://i.pravatar.cc/64?img=2',
			title: 'requested a review on PR #142',
			body: 'Refactor auth middleware to use JWT rotation',
			time: '18m ago',
			read: false
		},
		{
			id: 3,
			kind: 'alert',
			actor: 'Monitoring Bot',
			avatar: 'https://i.pravatar.cc/64?img=12',
			title: 'triggered an alert on api-prod-3',
			body: 'p95 latency > 800ms for 5 minutes',
			time: '1h ago',
			read: false
		},
		{
			id: 4,
			kind: 'invite',
			actor: 'Emily Reeves',
			avatar: 'https://i.pravatar.cc/64?img=4',
			title: 'invited you to Workspace Design System',
			body: 'Join the workspace to collaborate on shared components',
			time: '3h ago',
			read: true
		},
		{
			id: 5,
			kind: 'mention',
			actor: 'Marcus Johnson',
			avatar: 'https://i.pravatar.cc/64?img=3',
			title: 'replied to your comment',
			body: 'Good catch — I’ll update the migration to handle that edge case.',
			time: 'Yesterday',
			read: true
		},
		{
			id: 6,
			kind: 'pr',
			actor: 'James Park',
			avatar: 'https://i.pravatar.cc/64?img=8',
			title: 'merged PR #138',
			body: 'Add WebSocket ping/pong heartbeat',
			time: 'Yesterday',
			read: true
		}
	]);

	let tab = $state('all');
	let filtered = $derived(tab === 'unread' ? notifications.filter((n) => !n.read) : notifications);
	let unreadCount = $derived(notifications.filter((n) => !n.read).length);

	function markAllRead() {
		notifications = notifications.map((n) => ({ ...n, read: true }));
	}

	function toggleRead(id: number) {
		notifications = notifications.map((n) => (n.id === id ? { ...n, read: !n.read } : n));
	}

	function remove(id: number) {
		notifications = notifications.filter((n) => n.id !== id);
	}
</script>

<div class="mx-auto max-w-3xl">
	<div class="mb-6 flex items-center justify-between">
		<div class="flex items-center gap-3">
			<BellIcon class="size-6" />
			<div>
				<h1 class="h2">Inbox</h1>
				<p class="text-sm opacity-70">{unreadCount} unread notifications</p>
			</div>
		</div>
		<button class="btn preset-tonal btn-sm" onclick={markAllRead}>
			<CheckCheckIcon class="size-4" />
			<span>Mark all read</span>
		</button>
	</div>

	<Tabs value={tab} onValueChange={(e) => (tab = e.value)}>
		<Tabs.List>
			<Tabs.Trigger value="all">All ({notifications.length})</Tabs.Trigger>
			<Tabs.Trigger value="unread">Unread ({unreadCount})</Tabs.Trigger>
			<Tabs.Indicator />
		</Tabs.List>

		<Tabs.Content value={tab}>
			<div class="mt-4 space-y-2">
				{#each filtered as notif (notif.id)}
					{@const meta = kindMeta[notif.kind]}
					{@const Icon = meta.icon}
					<div
						class="group flex items-start gap-4 card p-4 card-hover {notif.read
							? 'opacity-70'
							: ''}"
					>
						<div class="relative shrink-0">
							<Avatar class="size-10">
								<Avatar.Image src={notif.avatar} />
								<Avatar.Fallback>{notif.actor.charAt(0)}</Avatar.Fallback>
							</Avatar>
							<span
								class="absolute -right-1 -bottom-1 flex size-5 items-center justify-center rounded-full {meta.preset}"
							>
								<Icon class="size-3" />
							</span>
						</div>

						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<p class="text-sm">
									<span class="font-semibold">{notif.actor}</span>
									<span class="opacity-80">{notif.title}</span>
								</p>
								{#if !notif.read}
									<span class="size-2 shrink-0 rounded-full bg-primary-500"></span>
								{/if}
							</div>
							<p class="mt-1 text-sm opacity-70">{notif.body}</p>
							<p class="mt-1 text-xs opacity-50">{notif.time}</p>
						</div>

						<div class="flex items-center gap-1 opacity-0 transition group-hover:opacity-100">
							<button
								class="btn-icon btn-icon-sm preset-tonal"
								onclick={() => toggleRead(notif.id)}
								aria-label="Toggle read"
							>
								<CheckCheckIcon class="size-4" />
							</button>
							<button
								class="btn-icon btn-icon-sm preset-tonal"
								onclick={() => remove(notif.id)}
								aria-label="Delete"
							>
								<TrashIcon class="size-4" />
							</button>
						</div>
					</div>
				{/each}
				{#if filtered.length === 0}
					<div class="py-12 text-center opacity-60">
						<p class="text-sm">You’re all caught up.</p>
					</div>
				{/if}
			</div>
		</Tabs.Content>
	</Tabs>
</div>
