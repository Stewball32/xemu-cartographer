<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import {
		TvIcon,
		CopyIcon,
		ExternalLinkIcon,
		LoaderIcon,
		Trash2Icon,
		LockIcon
	} from '@lucide/svelte';
	import { toaster } from '$lib/stores/toaster';
	import {
		mintOverlayToken,
		listOverlayTokens,
		revokeOverlayToken,
		canManageOverlays,
		type OverlayToken,
		type MintResult
	} from '$lib/utils/overlay-api';
	import {
		STREAM_ASSETS,
		liveURL,
		mockPath,
		mockURL,
		type StreamAsset
	} from '$lib/config/stream-assets';
	import AssetPreview from '$lib/components/studio/AssetPreview.svelte';

	const authorized = canManageOverlays();
	const origin = $derived(browser ? window.location.origin : '');

	let instance = $state('');
	let label = $state('');
	let minting = $state(false);
	let lastMinted = $state<MintResult | null>(null);

	let tokens = $state<OverlayToken[]>([]);
	let loading = $state(true);

	// Instance the current token is bound to (drives every card's live URL).
	const mintedInstance = $derived(lastMinted ? lastMinted.room.replace(/^host:/, '') : '');

	async function refresh() {
		loading = true;
		try {
			tokens = await listOverlayTokens();
		} catch (e) {
			toaster.error({ title: 'Failed to load tokens', description: String(e) });
		} finally {
			loading = false;
		}
	}

	async function mint() {
		const inst = instance.trim();
		if (!inst) {
			toaster.error({
				title: 'Instance required',
				description: 'Enter the container/instance name.'
			});
			return;
		}
		minting = true;
		try {
			lastMinted = await mintOverlayToken(`host:${inst}`, label.trim());
			toaster.success({ title: 'Overlay token minted', description: `host:${inst}` });
			label = '';
			await refresh();
		} catch (e) {
			toaster.error({ title: 'Mint failed', description: String(e) });
		} finally {
			minting = false;
		}
	}

	async function copy(url: string) {
		try {
			await navigator.clipboard.writeText(url);
			toaster.success({ title: 'Copied', description: 'URL on the clipboard.' });
		} catch {
			toaster.error({ title: 'Copy failed', description: url });
		}
	}

	async function revoke(t: OverlayToken) {
		try {
			await revokeOverlayToken(t.kid);
			toaster.info({ title: 'Revoked', description: t.room });
			if (lastMinted && lastMinted.kid === t.kid) lastMinted = null;
			await refresh();
		} catch (e) {
			toaster.error({ title: 'Revoke failed', description: String(e) });
		}
	}

	// A token-scoped asset can only produce a live OBS URL once a token is minted.
	function canCopyLive(a: StreamAsset): boolean {
		if (a.tokenScoped === false) return true;
		return authorized && lastMinted !== null;
	}

	function copyLive(a: StreamAsset) {
		if (!canCopyLive(a)) return;
		const inst = a.tokenScoped === false ? mintedInstance || 'default' : mintedInstance;
		copy(liveURL(origin, a, inst, lastMinted?.token ?? ''));
	}

	onMount(() => {
		if (authorized) refresh();
	});
</script>

<div class="flex flex-col gap-5">
	<header class="flex items-center gap-2">
		<TvIcon class="size-6" />
		<div>
			<h1 class="h3">Stream Studio</h1>
			<p class="text-sm text-surface-600-400">
				Every browser-source stream asset in one place — preview each live, then copy its OBS
				Browser Source URL.
			</p>
		</div>
	</header>

	<!-- Mint panel -->
	<section class="flex flex-col gap-3 card preset-tonal p-4">
		<div class="flex items-center gap-2">
			<LockIcon class="size-4 opacity-70" />
			<h2 class="h6">OBS link · scoped overlay token</h2>
		</div>

		{#if !authorized}
			<p class="text-sm text-surface-600-400">
				Previews work for anyone signed in. To copy a <em>live</em> OBS link you need the
				<code>overlay_manager</code> role (or admin) — it mints a scoped, read-only, revocable token bound
				to one container. Ask an admin.
			</p>
		{:else}
			<p class="max-w-prose text-sm text-surface-600-400">
				Mint one token for a container; it scopes every overlay + visualizer below to that
				container's room. Each card's <strong>Copy OBS URL</strong> then carries this token. Revocation
				is the safety net — tokens are long-lived.
			</p>
			<div class="flex flex-col gap-3 sm:flex-row sm:items-end">
				<label class="label flex-1">
					<span class="label-text">Container / instance</span>
					<input class="input" placeholder="e.g. pod-a" bind:value={instance} />
				</label>
				<label class="label flex-1">
					<span class="label-text">Label (optional)</span>
					<input class="input" placeholder="e.g. Main stream" bind:value={label} />
				</label>
				<button class="preset-filled-primary btn" disabled={minting} onclick={mint}>
					{#if minting}<LoaderIcon class="size-4 animate-spin" />{/if}
					<span>Mint token</span>
				</button>
			</div>
			{#if lastMinted}
				<div class="card preset-tonal-success p-3 text-sm">
					Token active for <code>{lastMinted.room}</code>. The <strong>Copy OBS URL</strong> buttons below
					now produce live, ready-to-paste links. The token isn't shown again — copy what you need, or
					re-mint.
				</div>
			{/if}
		{/if}
	</section>

	<!-- Gallery -->
	<section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
		{#each STREAM_ASSETS as asset (asset.id)}
			{@const Icon = asset.icon}
			<article class="flex flex-col gap-3 overflow-hidden card preset-tonal p-4">
				{#if asset.preview}
					<AssetPreview
						src={mockPath(asset)}
						aspect={asset.aspect}
						kind={asset.kind}
						title="{asset.name} preview"
					/>
				{:else}
					<div
						class="grid aspect-video place-items-center rounded-sm bg-surface-200-800 text-xs opacity-60"
					>
						No preview
					</div>
				{/if}

				<div class="flex items-start gap-2">
					<Icon class="mt-0.5 size-5 shrink-0 opacity-80" />
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-2">
							<h3 class="truncate h6">{asset.name}</h3>
							<span
								class="chip preset-tonal-{asset.kind === 'overlay'
									? 'primary'
									: 'secondary'} text-[0.6rem] uppercase"
							>
								{asset.kind}
							</span>
						</div>
						<div class="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-surface-600-400">
							<span>{asset.aspect.label}</span>
							<span>{asset.obsHint}</span>
						</div>
					</div>
				</div>

				<p class="text-sm text-surface-700-300">{asset.description}</p>
				{#if asset.note}
					<p class="text-xs text-surface-500 italic">{asset.note}</p>
				{/if}

				<div class="mt-auto flex flex-wrap items-center gap-2 pt-1">
					<button
						class="preset-filled-primary btn btn-sm"
						disabled={!canCopyLive(asset)}
						title={canCopyLive(asset) ? 'Copy the OBS browser-source URL' : 'Mint a token first'}
						onclick={() => copyLive(asset)}
					>
						<CopyIcon class="size-4" />
						<span>Copy OBS URL</span>
					</button>
					<button class="btn preset-tonal btn-sm" onclick={() => copy(mockURL(origin, asset))}>
						<CopyIcon class="size-4" />
						<span>Mock URL</span>
					</button>
					<!-- Runtime-computed absolute URL opened in a new tab, not a static route. -->
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a class="btn preset-tonal btn-sm" href={mockPath(asset)} target="_blank" rel="noopener">
						<ExternalLinkIcon class="size-4" />
						<span>Open</span>
					</a>
				</div>
			</article>
		{/each}
	</section>

	<!-- Active tokens -->
	{#if authorized}
		<section class="card preset-tonal p-4">
			<h2 class="mb-2 h6">Active tokens</h2>
			{#if loading}
				<div class="flex items-center gap-2 text-sm">
					<LoaderIcon class="size-4 animate-spin" /> Loading…
				</div>
			{:else if tokens.length === 0}
				<p class="text-sm text-surface-600-400">No active overlay tokens.</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="table">
						<thead>
							<tr>
								<th>Room</th>
								<th>Label</th>
								<th>Expires</th>
								<th class="text-right">Revoke</th>
							</tr>
						</thead>
						<tbody>
							{#each tokens as t (t.kid)}
								<tr>
									<td class="font-mono text-xs">{t.room}</td>
									<td>{t.label || '—'}</td>
									<td class="text-xs">{t.expires_at}</td>
									<td class="text-right">
										<button
											class="btn-icon preset-tonal-error"
											onclick={() => revoke(t)}
											aria-label="Revoke"
										>
											<Trash2Icon class="size-4" />
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
	{/if}
</div>
