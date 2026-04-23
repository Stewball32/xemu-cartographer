<script lang="ts">
	import { Avatar } from '@skeletonlabs/skeleton-svelte';
	import {
		SearchIcon,
		SendIcon,
		PaperclipIcon,
		SmileIcon,
		PhoneIcon,
		VideoIcon
	} from '@lucide/svelte';

	interface Conversation {
		id: number;
		name: string;
		avatar: string;
		lastMessage: string;
		time: string;
		unread: number;
		online: boolean;
	}

	interface Message {
		id: number;
		fromMe: boolean;
		text: string;
		time: string;
	}

	const conversations: Conversation[] = [
		{
			id: 1,
			name: 'Alex Chen',
			avatar: 'https://i.pravatar.cc/64?img=1',
			lastMessage: 'Pushed the fix — CI should be green in a few min',
			time: '2m',
			unread: 2,
			online: true
		},
		{
			id: 2,
			name: 'Sarah Kim',
			avatar: 'https://i.pravatar.cc/64?img=2',
			lastMessage: 'Can you review the Figma before EOD?',
			time: '14m',
			unread: 0,
			online: true
		},
		{
			id: 3,
			name: 'Marcus Johnson',
			avatar: 'https://i.pravatar.cc/64?img=3',
			lastMessage: 'Deploy scheduled for tonight, 10pm PT',
			time: '1h',
			unread: 0,
			online: false
		},
		{
			id: 4,
			name: 'Emily Reeves',
			avatar: 'https://i.pravatar.cc/64?img=4',
			lastMessage: 'Thanks! 🎉',
			time: '3h',
			unread: 0,
			online: false
		},
		{
			id: 5,
			name: 'James Park',
			avatar: 'https://i.pravatar.cc/64?img=8',
			lastMessage: 'Let me know when you get a chance',
			time: 'Yesterday',
			unread: 1,
			online: false
		}
	];

	let activeId = $state(1);
	let draft = $state('');
	let active = $derived(conversations.find((c) => c.id === activeId) ?? conversations[0]);

	const messages: Message[] = [
		{ id: 1, fromMe: false, text: 'Hey, got a sec?', time: '9:42 AM' },
		{ id: 2, fromMe: true, text: 'Yep, what’s up?', time: '9:42 AM' },
		{
			id: 3,
			fromMe: false,
			text: 'The rate limiter is rejecting the retry batch. Any idea why?',
			time: '9:43 AM'
		},
		{
			id: 4,
			fromMe: true,
			text: 'Probably the token bucket — we cap retries at 5/min per IP.',
			time: '9:44 AM'
		},
		{
			id: 5,
			fromMe: false,
			text: 'Ah that would do it. Can we bump it for the job?',
			time: '9:45 AM'
		},
		{
			id: 6,
			fromMe: true,
			text: 'Yeah, I’ll add a bypass flag for internal jobs. PR in 10.',
			time: '9:45 AM'
		},
		{
			id: 7,
			fromMe: false,
			text: 'Pushed the fix — CI should be green in a few min',
			time: '9:52 AM'
		}
	];

	function send() {
		if (!draft.trim()) return;
		draft = '';
	}
</script>

<h1 class="mb-6 h2">Chat</h1>

<div
	class="grid h-[calc(100vh-14rem)] grid-cols-1 gap-4 overflow-hidden card md:grid-cols-[20rem_1fr]"
>
	<!-- Conversation list -->
	<aside class="flex flex-col border-r border-surface-200-800">
		<div class="border-b border-surface-200-800 p-4">
			<div class="input-group grid-cols-[auto_1fr]">
				<div class="input-group-cell">
					<SearchIcon class="size-4" />
				</div>
				<input type="search" placeholder="Search..." />
			</div>
		</div>
		<div class="flex-1 overflow-y-auto">
			{#each conversations as conv (conv.id)}
				<button
					class="flex w-full items-start gap-3 border-b border-surface-200-800 p-4 text-left transition hover:preset-tonal-primary {activeId ===
					conv.id
						? 'preset-tonal-primary'
						: ''}"
					onclick={() => (activeId = conv.id)}
				>
					<div class="relative shrink-0">
						<Avatar class="size-10">
							<Avatar.Image src={conv.avatar} />
							<Avatar.Fallback>{conv.name.charAt(0)}</Avatar.Fallback>
						</Avatar>
						{#if conv.online}
							<span
								class="absolute right-0 bottom-0 size-2.5 rounded-full bg-success-500 ring-2 ring-surface-50-950"
							></span>
						{/if}
					</div>
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between gap-2">
							<p class="truncate text-sm font-semibold">{conv.name}</p>
							<span class="text-xs opacity-60">{conv.time}</span>
						</div>
						<p class="truncate text-xs opacity-70">{conv.lastMessage}</p>
					</div>
					{#if conv.unread > 0}
						<span class="badge preset-filled-primary-500 text-xs">{conv.unread}</span>
					{/if}
				</button>
			{/each}
		</div>
	</aside>

	<!-- Thread -->
	<section class="flex flex-col">
		<header class="flex items-center justify-between border-b border-surface-200-800 p-4">
			<div class="flex items-center gap-3">
				<Avatar class="size-10">
					<Avatar.Image src={active.avatar} />
					<Avatar.Fallback>{active.name.charAt(0)}</Avatar.Fallback>
				</Avatar>
				<div>
					<p class="text-sm font-semibold">{active.name}</p>
					<p class="text-xs opacity-60">{active.online ? 'Online' : 'Offline'}</p>
				</div>
			</div>
			<div class="flex items-center gap-1">
				<button class="btn-icon preset-tonal" aria-label="Call"><PhoneIcon class="size-4" /></button
				>
				<button class="btn-icon preset-tonal" aria-label="Video"
					><VideoIcon class="size-4" /></button
				>
			</div>
		</header>

		<div class="flex-1 space-y-4 overflow-y-auto p-4">
			{#each messages as msg (msg.id)}
				<div class="flex {msg.fromMe ? 'justify-end' : 'justify-start'}">
					<div
						class="max-w-[75%] space-y-1 rounded-2xl px-4 py-2 {msg.fromMe
							? 'preset-filled-primary-500'
							: 'preset-tonal-surface'}"
					>
						<p class="text-sm">{msg.text}</p>
						<p class="text-[10px] opacity-60">{msg.time}</p>
					</div>
				</div>
			{/each}
		</div>

		<footer class="border-t border-surface-200-800 p-4">
			<div class="input-group grid-cols-[auto_1fr_auto_auto]">
				<button class="input-group-cell" aria-label="Attach">
					<PaperclipIcon class="size-4" />
				</button>
				<input
					type="text"
					placeholder="Type a message..."
					bind:value={draft}
					onkeydown={(e) => e.key === 'Enter' && send()}
				/>
				<button class="input-group-cell" aria-label="Emoji">
					<SmileIcon class="size-4" />
				</button>
				<button class="btn preset-filled" onclick={send} aria-label="Send">
					<SendIcon class="size-4" />
				</button>
			</div>
		</footer>
	</section>
</div>
