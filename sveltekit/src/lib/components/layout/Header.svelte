<script lang="ts">
	import { AppBar, Popover } from '@skeletonlabs/skeleton-svelte';
	import { resolve } from '$app/paths';
	import NavToggle from './NavToggle.svelte';
	import ModeToggle from './ModeToggle.svelte';
	import HeaderBell from './HeaderBell.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { LogInIcon, LogOutIcon, UserIcon } from '@lucide/svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';

	let { onToggle }: { onToggle: () => void } = $props();
</script>

<AppBar class="h-16 p-4">
	<AppBar.Toolbar class="grid-cols-[1fr_auto]">
		<AppBar.Lead class="min-w-0">
			<NavToggle onclick={onToggle} />
		</AppBar.Lead>
		<AppBar.Trail class="flex items-center gap-1">
			<ModeToggle />
			{#if auth.isLoggedIn && auth.user}
				<HeaderBell />
				<Popover positioning={{ placement: 'bottom-end' }}>
					<Popover.Trigger
						class="flex size-9 items-center justify-center rounded-full hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2"
						aria-label="Open user menu"
						title={auth.user?.email}
					>
						<UserAvatar user={auth.user} size="size-9" />
					</Popover.Trigger>
					<Popover.Positioner class="z-50!">
						<Popover.Content
							class="max-w-72 min-w-56 card preset-filled-surface-100-900 p-2 shadow-xl"
						>
							<div class="min-w-0 border-b border-surface-200-800 px-3 pb-2">
								<p class="truncate text-sm font-medium">
									{auth.user?.name || auth.user?.username}
								</p>
								{#if auth.user?.name}
									<p class="truncate text-xs opacity-60">{auth.user?.username}</p>
								{/if}
							</div>
							<a
								href={resolve('/profile/')}
								class="rounded-token mt-1 flex items-center gap-2 px-3 py-2 hover:preset-tonal"
							>
								<UserIcon class="size-4" />
								<span class="text-sm">Profile</span>
							</a>
							<button
								type="button"
								class="rounded-token flex w-full items-center gap-2 px-3 py-2 text-left hover:preset-tonal"
								onclick={() => auth.logout()}
							>
								<LogOutIcon class="size-4" />
								<span class="text-sm">Sign out</span>
							</button>
						</Popover.Content>
					</Popover.Positioner>
				</Popover>
			{:else}
				<a
					href={resolve('/login/')}
					class="flex size-9 items-center justify-center rounded-full preset-tonal hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2"
					aria-label="Sign in"
					title="Sign in"
				>
					<LogInIcon class="size-4" />
				</a>
			{/if}
		</AppBar.Trail>
	</AppBar.Toolbar>
</AppBar>
