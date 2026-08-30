<script lang="ts">
	// Settings → Accounts: OAuth provider link/unlink. Same logic as the old
	// Connected Accounts section, one row per enabled provider. No action bar
	// on this tab — connects/disconnects apply immediately.
	import { onMount } from 'svelte';
	import { LinkIcon, LoaderIcon, UnlinkIcon } from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { toastPromise } from '$lib/stores/toaster';
	import { OAUTH_PROVIDERS } from '$lib/config/app';
	import Card from '$lib/components/ui/Card.svelte';

	let { active }: { active: boolean } = $props();

	let linkedAuths = $state<Array<Record<string, string>>>([]);
	let enabledProviders = $state<string[]>([]);
	let linkingProvider = $state<string | null>(null);
	let unlinkingProvider = $state<string | null>(null);
	const linkedProviderNames = $derived(new Set(linkedAuths.map((a) => a.provider)));
	const visibleProviders = $derived(enabledProviders.filter((name) => name in OAUTH_PROVIDERS));

	onMount(async () => {
		try {
			const methods = await pb.collection('users').listAuthMethods();
			enabledProviders = methods.oauth2?.providers?.map((p) => p.name) ?? [];
		} catch {
			enabledProviders = [];
		}
		await loadLinkedAuths();
	});

	async function loadLinkedAuths() {
		if (!auth.user) return;
		try {
			linkedAuths = await auth.listExternalAuths(auth.user.id);
		} catch {
			linkedAuths = [];
		}
	}

	async function linkProvider(provider: string) {
		if (!auth.user) return;
		linkingProvider = provider;
		const label = OAUTH_PROVIDERS[provider]?.label ?? provider;
		try {
			await toastPromise(auth.linkOAuth(provider), {
				loading: { title: 'Connecting', description: label },
				success: { title: 'Connected', description: `${label} account linked.` },
				errorTitle: 'Connect failed',
				errorDescription: (err) => {
					const message = err instanceof Error ? err.message : 'Failed to link account.';
					return message.includes('already') || message.includes('unique')
						? 'This account is already linked to another user.'
						: message;
				}
			});
			await loadLinkedAuths();
		} catch {
			/* toast shown */
		} finally {
			linkingProvider = null;
		}
	}

	async function unlinkProvider(provider: string) {
		if (!auth.user) return;
		unlinkingProvider = provider;
		const label = OAUTH_PROVIDERS[provider]?.label ?? provider;
		try {
			await toastPromise(auth.unlinkOAuth(auth.user.id, provider), {
				loading: { title: 'Disconnecting', description: label },
				success: { title: 'Disconnected', description: `${label} account unlinked.` },
				errorTitle: 'Disconnect failed'
			});
			await loadLinkedAuths();
		} catch {
			/* toast shown */
		} finally {
			unlinkingProvider = null;
		}
	}
</script>

{#if active}
	<Card class="flex flex-col gap-3">
		<div class="flex flex-wrap items-center justify-between gap-2.5">
			<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
				Sign-in
			</span>
			<span class="text-[11.5px] opacity-60">
				Linking Discord keeps your LAN check-ins synced to the server.
			</span>
		</div>

		{#if visibleProviders.length === 0}
			<p class="text-sm opacity-50">No OAuth providers are configured.</p>
		{:else}
			{#each visibleProviders as provider (provider)}
				{@const meta = OAUTH_PROVIDERS[provider]}
				{@const isLinked = linkedProviderNames.has(provider)}
				{@const isLinking = linkingProvider === provider}
				{@const isUnlinking = unlinkingProvider === provider}
				<div
					class="flex flex-wrap items-center gap-3.5 rounded-[9px] border border-surface-500/20 px-3.5 py-2.5"
				>
					<img src={meta.icon} alt={meta.label} class="size-5.5 flex-none" />
					<div class="flex min-w-30 flex-1 flex-col gap-px">
						<b class="text-[13px]">{meta.label}</b>
						<span class="text-[11.5px] {isLinked ? 'text-success-600-400' : 'opacity-50'}">
							{isLinked ? 'Connected' : 'Not connected'}
						</span>
					</div>
					{#if isLinked}
						<button
							class="btn preset-tonal-error btn-sm"
							onclick={() => unlinkProvider(provider)}
							disabled={isUnlinking}
						>
							{#if isUnlinking}<LoaderIcon class="size-4 animate-spin" />{:else}<UnlinkIcon
									class="size-4"
								/>{/if}
							<span>Disconnect</span>
						</button>
					{:else}
						<button
							class="btn preset-tonal btn-sm"
							onclick={() => linkProvider(provider)}
							disabled={isLinking}
						>
							{#if isLinking}<LoaderIcon class="size-4 animate-spin" />{:else}<LinkIcon
									class="size-4"
								/>{/if}
							<span>Connect</span>
						</button>
					{/if}
				</div>
			{/each}
		{/if}
	</Card>
{/if}
