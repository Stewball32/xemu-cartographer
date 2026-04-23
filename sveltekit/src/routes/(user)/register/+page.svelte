<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { OAUTH_PROVIDERS } from '$lib/config/app';
	import { buildLoginUrl } from '$lib/utils/redirect';
	import pb from '$lib/pocketbase';
	import { UserPlusIcon, MailIcon, LockIcon } from '@lucide/svelte';
	import { onMount } from 'svelte';

	let { data } = $props();

	let email = $state('');
	let password = $state('');
	let passwordConfirm = $state('');
	let error = $state('');
	let loading = $state(false);
	let enabledProviders = $state<string[]>([]);

	const visibleProviders = $derived(
		enabledProviders
			.filter((name) => name in OAUTH_PROVIDERS)
			.map((name) => ({ name, meta: OAUTH_PROVIDERS[name] }))
	);

	const layout = $derived(
		visibleProviders.length <= 2 ? 'single' : visibleProviders.length <= 6 ? 'double' : 'compact'
	);

	const isOdd = $derived(visibleProviders.length % 2 !== 0);

	onMount(async () => {
		try {
			const methods = await pb.collection('users').listAuthMethods();
			enabledProviders = methods.oauth2?.providers?.map((p) => p.name) ?? [];
		} catch {
			enabledProviders = [];
		}
	});

	async function handleRegister(e: SubmitEvent) {
		e.preventDefault();
		error = '';

		if (password !== passwordConfirm) {
			error = 'Passwords do not match.';
			return;
		}

		loading = true;
		try {
			await auth.register(email, password, passwordConfirm);
			// data.redirectTo is runtime-validated in +page.ts via safeRedirectTarget
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(data.redirectTo);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Registration failed. Please try again.';
		} finally {
			loading = false;
		}
	}

	async function handleOAuth(provider: string) {
		error = '';
		loading = true;
		try {
			await auth.loginWithOAuth(provider);
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(data.redirectTo);
		} catch (err) {
			error = err instanceof Error ? err.message : 'OAuth sign-in failed. Please try again.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex min-h-[70vh] items-center justify-center">
	<div class="w-full max-w-md space-y-6">
		<div class="space-y-6 card p-8">
			<!-- Header -->
			<div class="space-y-2 text-center">
				<UserPlusIcon class="mx-auto size-10 text-primary-500" />
				<h1 class="h3">Create Account</h1>
				<p class="text-sm opacity-70">Sign up to get started</p>
			</div>

			<!-- Error Alert -->
			{#if error}
				<aside class="alert preset-filled-error-500">
					<p>{error}</p>
				</aside>
			{/if}

			<!-- Form -->
			<form class="space-y-4" onsubmit={handleRegister}>
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

				<label class="label">
					<span>Password</span>
					<div class="input-group grid-cols-[auto_1fr_auto]">
						<div class="ig-cell"><LockIcon class="size-4" /></div>
						<input
							type="password"
							class="ig-input"
							placeholder="••••••••"
							bind:value={password}
							required
							minlength="8"
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
							required
							minlength="8"
						/>
					</div>
				</label>

				<button type="submit" class="btn w-full preset-filled" disabled={loading}>
					{loading ? 'Creating account...' : 'Create Account'}
				</button>
			</form>

			{#if visibleProviders.length > 0}
				<!-- Divider -->
				<div class="flex items-center gap-4">
					<hr class="hr flex-1" />
					<span class="text-xs opacity-50">or continue with</span>
					<hr class="hr flex-1" />
				</div>

				<!-- Social Login -->
				<div
					class="grid gap-3"
					class:grid-cols-1={layout === 'single'}
					class:grid-cols-2={layout === 'double'}
					class:grid-cols-4={layout === 'compact'}
				>
					{#each visibleProviders as { name, meta }, i (name)}
						<button
							type="button"
							class="btn w-full preset-tonal"
							class:col-span-2={layout === 'double' && isOdd && i === visibleProviders.length - 1}
							disabled={loading}
							title={meta.label}
							onclick={() => handleOAuth(name)}
						>
							<img
								src={meta.icon}
								alt={meta.label}
								class="shrink-0"
								class:size-8={layout === 'single'}
								class:size-6={layout !== 'single'}
							/>
							{#if layout !== 'compact'}
								<span class="flex-1 text-center">{meta.label}</span>
								<span
									class="shrink-0"
									class:size-8={layout === 'single'}
									class:size-6={layout !== 'single'}
									aria-hidden="true"
								></span>
							{/if}
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<p class="text-center text-sm opacity-70">
			Already have an account?
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={buildLoginUrl(data.redirectTo)} class="text-primary-500 hover:underline">Sign in</a>
		</p>
	</div>
</div>
