<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { auth } from '$lib/stores/auth.svelte';
	import pb from '$lib/pocketbase';
	import { OAUTH_PROVIDERS } from '$lib/config/app';
	import { buildRegisterUrl } from '$lib/utils/redirect';
	import { LogInIcon, MailIcon, LockIcon } from '@lucide/svelte';

	let { data } = $props();

	let email = $state('');
	let password = $state('');
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

	async function handleLogin(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await auth.login(email, password);
			// data.redirectTo is runtime-validated in +page.ts via safeRedirectTarget
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(data.redirectTo);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed. Please try again.';
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
				<LogInIcon class="mx-auto size-10 text-primary-500" />
				<h1 class="h3">Sign In</h1>
				<p class="text-sm opacity-70">Enter your credentials to continue</p>
			</div>

			<!-- Error Alert -->
			{#if error}
				<aside class="alert preset-filled-error-500">
					<p>{error}</p>
				</aside>
			{/if}

			<!-- Form -->
			<form class="space-y-4" onsubmit={handleLogin}>
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
						/>
					</div>
				</label>

				<div class="flex justify-end">
					<a href={resolve('/forgot-password/')} class="text-sm text-primary-500 hover:underline"
						>Forgot password?</a
					>
				</div>

				<button type="submit" class="btn w-full preset-filled" disabled={loading}>
					{loading ? 'Signing in...' : 'Sign In'}
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
			Don't have an account?
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={buildRegisterUrl(data.redirectTo)} class="text-primary-500 hover:underline"
				>Create one</a
			>
		</p>
	</div>
</div>
