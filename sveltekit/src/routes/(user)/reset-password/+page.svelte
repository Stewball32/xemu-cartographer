<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth.svelte';
	import { toaster } from '$lib/stores/toaster';
	import { LockIcon, ArrowLeftIcon, KeyRoundIcon } from '@lucide/svelte';

	let token = $state($page.data.token as string);
	let password = $state('');
	let passwordConfirm = $state('');
	let error = $state('');
	let loading = $state(false);

	const mismatch = $derived(passwordConfirm !== '' && password !== passwordConfirm);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (password !== passwordConfirm) {
			error = 'Passwords do not match.';
			return;
		}
		error = '';
		loading = true;
		try {
			await auth.confirmPasswordReset(token, password, passwordConfirm);
			toaster.success({
				title: 'Password Reset',
				description: 'You can now sign in with your new password.'
			});
			goto(resolve('/login/'));
		} catch (err) {
			error =
				err instanceof Error ? err.message : 'Failed to reset password. The link may have expired.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-[70vh] items-center justify-center">
	<div class="w-full max-w-md space-y-6">
		<div class="space-y-6 card p-8">
			<div class="space-y-2 text-center">
				<KeyRoundIcon class="mx-auto size-10 text-primary-500" />
				<h1 class="h3">Set New Password</h1>
				<p class="text-sm opacity-70">Enter your new password below.</p>
			</div>

			{#if !token}
				<aside class="alert preset-filled-error-500">
					<p>Missing reset token. Please use the link from your email.</p>
				</aside>
			{:else}
				{#if error}
					<aside class="alert preset-filled-error-500">
						<p>{error}</p>
					</aside>
				{/if}

				<form class="space-y-4" onsubmit={handleSubmit}>
					<label class="label">
						<span>New Password</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><LockIcon class="size-4" /></div>
							<input
								type="password"
								class="ig-input"
								placeholder="••••••••"
								bind:value={password}
								minlength="8"
								required
							/>
						</div>
					</label>

					<label class="label">
						<span>Confirm Password</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><LockIcon class="size-4" /></div>
							<input
								type="password"
								class="ig-input"
								placeholder="••••••••"
								bind:value={passwordConfirm}
								minlength="8"
								required
							/>
						</div>
						{#if mismatch}
							<p class="text-sm text-error-500">Passwords do not match.</p>
						{/if}
					</label>

					<button type="submit" class="btn w-full preset-filled" disabled={loading || mismatch}>
						{loading ? 'Resetting...' : 'Reset Password'}
					</button>
				</form>
			{/if}

			<div class="text-center">
				<a
					href={resolve('/login/')}
					class="inline-flex items-center gap-1 text-sm text-primary-500 hover:underline"
				>
					<ArrowLeftIcon class="size-3" />
					Back to Sign In
				</a>
			</div>
		</div>
	</div>
</div>
