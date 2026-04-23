<script lang="ts">
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth.svelte';
	import { MailIcon, ArrowLeftIcon, KeyRoundIcon } from '@lucide/svelte';

	let email = $state('');
	let error = $state('');
	let loading = $state(false);
	let sent = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.requestPasswordReset(email);
			sent = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to send reset email.';
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
				<h1 class="h3">Reset Password</h1>
				<p class="text-sm opacity-70">
					{sent
						? 'Check your email for a password reset link.'
						: "Enter your email and we'll send you a reset link."}
				</p>
			</div>

			{#if error}
				<aside class="alert preset-filled-error-500">
					<p>{error}</p>
				</aside>
			{/if}

			{#if sent}
				<aside class="alert preset-filled-success-500">
					<p>If an account with that email exists, a reset link has been sent.</p>
				</aside>
			{:else}
				<form class="space-y-4" onsubmit={handleSubmit}>
					<label class="label">
						<span>Email</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><MailIcon class="size-4" /></div>
							<input
								type="email"
								class="ig-input"
								placeholder="you@example.com"
								bind:value={email}
								required
							/>
						</div>
					</label>

					<button type="submit" class="btn w-full preset-filled" disabled={loading}>
						{loading ? 'Sending...' : 'Send Reset Link'}
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
