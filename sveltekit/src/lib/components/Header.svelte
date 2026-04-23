<script lang="ts">
	import { AppBar, Avatar } from '@skeletonlabs/skeleton-svelte';
	import { resolve } from '$app/paths';
	import NavToggle from '$lib/components/NavToggle.svelte';
	import ModeToggle from '$lib/components/ModeToggle.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { LogInIcon, LogOutIcon } from '@lucide/svelte';
	import { getFileURL } from '$lib/utils/files';
	import type { RecordModel } from 'pocketbase';

	let { onToggle }: { onToggle: () => void } = $props();

	const initials = $derived(
		auth.user?.name
			?.split(' ')
			.map((n: string) => n[0])
			.join('')
			.toUpperCase()
			.slice(0, 2) ??
			auth.user?.email?.charAt(0).toUpperCase() ??
			'?'
	);
</script>

<AppBar class="h-16 p-4">
	<AppBar.Toolbar class="grid-cols-[1fr_auto]">
		<AppBar.Lead>
			<NavToggle onclick={onToggle} />
		</AppBar.Lead>
		<AppBar.Trail>
			<ModeToggle />
			{#if auth.isLoggedIn && auth.user}
				<div class="flex items-center gap-2">
					<a
						href={resolve('/profile/')}
						class="rounded-token flex items-center gap-2 px-2 py-1 hover:preset-tonal"
						title={auth.user?.email}
					>
						<Avatar class="size-8">
							<Avatar.Fallback>{initials}</Avatar.Fallback>
							<Avatar.Image src={getFileURL(auth.user as RecordModel, 'avatar')} />
						</Avatar>
						<span class="hidden text-sm font-medium sm:block">
							{auth.user?.name ?? auth.user?.email}
						</span>
					</a>
					<button
						class="btn-icon preset-tonal btn-sm"
						onclick={() => auth.logout()}
						title="Sign out"
					>
						<LogOutIcon class="size-4" />
					</button>
				</div>
			{:else}
				<a href={resolve('/login/')} class="btn preset-tonal btn-sm">
					<LogInIcon class="size-4" />
					<span>Login</span>
				</a>
			{/if}
		</AppBar.Trail>
	</AppBar.Toolbar>
</AppBar>
