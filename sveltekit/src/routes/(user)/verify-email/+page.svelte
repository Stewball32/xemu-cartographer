<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth.svelte';
	import { page } from '$app/stores';
	import { MailCheckIcon, ArrowLeftIcon, LoaderIcon } from '@lucide/svelte';

	let token = $page.data.token as string;
	let status = $state<'loading' | 'success' | 'error' | 'missing'>(token ? 'loading' : 'missing');
	let error = $state('');

	onMount(async () => {
		if (!token) return;
		try {
			await auth.confirmVerification(token);
			status = 'success';
		} catch (err) {
			status = 'error';
			error =
				err instanceof Error ? err.message : 'Verification failed. The link may have expired.';
		}
	});
</script>

<div class="flex min-h-[70vh] items-center justify-center">
	<div class="w-full max-w-md space-y-6">
		<div class="space-y-6 card p-8">
			<div class="space-y-2 text-center">
				<MailCheckIcon class="mx-auto size-10 text-primary-500" />
				<h1 class="h3">Email Verification</h1>
			</div>

			{#if status === 'loading'}
				<div class="flex flex-col items-center gap-3 py-4">
					<LoaderIcon class="size-8 animate-spin text-primary-500" />
					<p class="text-sm opacity-70">Verifying your email...</p>
				</div>
			{:else if status === 'success'}
				<aside class="alert preset-filled-success-500">
					<p>Your email has been verified successfully!</p>
				</aside>
			{:else if status === 'error'}
				<aside class="alert preset-filled-error-500">
					<p>{error}</p>
				</aside>
			{:else}
				<aside class="alert preset-filled-error-500">
					<p>Missing verification token. Please use the link from your email.</p>
				</aside>
			{/if}

			<div class="text-center">
				{#if auth.isLoggedIn}
					<a
						href={resolve('/settings/')}
						class="inline-flex items-center gap-1 text-sm text-primary-500 hover:underline"
					>
						<ArrowLeftIcon class="size-3" />
						Back to Settings
					</a>
				{:else}
					<a
						href={resolve('/login/')}
						class="inline-flex items-center gap-1 text-sm text-primary-500 hover:underline"
					>
						<ArrowLeftIcon class="size-3" />
						Go to Sign In
					</a>
				{/if}
			</div>
		</div>
	</div>
</div>
