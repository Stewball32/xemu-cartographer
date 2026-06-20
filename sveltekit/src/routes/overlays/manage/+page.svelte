<script lang="ts">
	import { onMount } from 'svelte';
	import { MonitorIcon, CopyIcon, Trash2Icon, LoaderIcon } from '@lucide/svelte';
	import { toaster } from '$lib/stores/toaster';
	import {
		mintOverlayToken,
		listOverlayTokens,
		revokeOverlayToken,
		canManageOverlays,
		type OverlayToken,
		type MintResult
	} from '$lib/utils/overlay-api';

	const authorized = canManageOverlays();

	let instance = $state('');
	let label = $state('');
	let minting = $state(false);
	let lastMinted = $state<MintResult | null>(null);

	let tokens = $state<OverlayToken[]>([]);
	let loading = $state(true);

	// m.url is "/overlays/<instance>/?token=<jwt>". Split it once so we can offer
	// every overlay view bound to the same scoped token (the token is scoped to
	// the instance, not the view) plus a token-free mock preview.
	function parts(m: MintResult): { origin: string; base: string; query: string } {
		const origin = window.location.origin;
		const q = m.url.indexOf('?');
		const base = q === -1 ? m.url : m.url.slice(0, q); // "/overlays/<instance>/"
		const query = q === -1 ? '' : m.url.slice(q); // "?token=<jwt>"
		return { origin, base, query };
	}

	function scoreboardURL(m: MintResult): string {
		const { origin, base, query } = parts(m);
		return `${origin}${base}${query}`;
	}

	function statusURL(m: MintResult): string {
		const { origin, base, query } = parts(m);
		return `${origin}${base}status/${query}`;
	}

	function mockURL(m: MintResult): string {
		const { origin, base } = parts(m);
		return `${origin}${base}?mock=1`;
	}

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
			toaster.success({ title: 'Copied', description: 'Overlay URL on the clipboard.' });
		} catch {
			toaster.error({ title: 'Copy failed', description: url });
		}
	}

	async function revoke(t: OverlayToken) {
		try {
			await revokeOverlayToken(t.kid);
			toaster.info({ title: 'Revoked', description: t.room });
			await refresh();
		} catch (e) {
			toaster.error({ title: 'Revoke failed', description: String(e) });
		}
	}

	onMount(() => {
		if (authorized) refresh();
	});
</script>

<div class="flex flex-col gap-4">
	<header class="flex items-center gap-2">
		<MonitorIcon class="size-5" />
		<h1 class="h4">Overlay tokens</h1>
	</header>

	{#if !authorized}
		<div class="card preset-tonal p-6 text-sm">
			You don't have permission to manage overlay tokens. Ask an admin for the
			<code>overlay_manager</code> role.
		</div>
	{:else}
		<p class="max-w-prose text-sm text-surface-600-400">
			Mint a scoped, read-only token for an OBS browser-source overlay. The token is bound to one
			container's room and can be revoked anytime. Tokens are long-lived by default — revocation is
			the safety net.
		</p>

		<!-- Mint form -->
		<div class="flex flex-col gap-3 card preset-tonal p-4">
			<div class="flex flex-col gap-3 sm:flex-row sm:items-end">
				<label class="label flex-1">
					<span class="label-text">Container / instance</span>
					<input class="input" placeholder="e.g. pod-a" bind:value={instance} />
				</label>
				<label class="label flex-1">
					<span class="label-text">Label (optional)</span>
					<input class="input" placeholder="e.g. Scoreboard — main stream" bind:value={label} />
				</label>
				<button class="preset-filled-primary btn" disabled={minting} onclick={mint}>
					{#if minting}<LoaderIcon class="size-4 animate-spin" />{/if}
					<span>Mint token</span>
				</button>
			</div>

			{#if lastMinted}
				<div class="flex flex-col gap-3 card preset-tonal-success p-3 text-sm">
					<span class="font-medium">
						Overlay URLs for <code>{lastMinted.room}</code> — add each as an OBS Browser Source (1920×1080,
						transparent).
					</span>

					<label class="flex flex-col gap-1">
						<span class="text-xs font-medium">Scoreboard / roster (primary)</span>
						<div class="flex items-center gap-2">
							<input
								class="input flex-1 font-mono text-xs"
								readonly
								value={scoreboardURL(lastMinted)}
							/>
							<button
								class="btn-icon preset-tonal"
								onclick={() => copy(scoreboardURL(lastMinted!))}
							>
								<CopyIcon class="size-4" />
							</button>
						</div>
					</label>

					<label class="flex flex-col gap-1">
						<span class="text-xs font-medium">Match-status strip</span>
						<div class="flex items-center gap-2">
							<input
								class="input flex-1 font-mono text-xs"
								readonly
								value={statusURL(lastMinted)}
							/>
							<button class="btn-icon preset-tonal" onclick={() => copy(statusURL(lastMinted!))}>
								<CopyIcon class="size-4" />
							</button>
						</div>
					</label>

					<span class="text-xs text-surface-600-400">
						Copy these now — the token isn't shown again. No live game?
						<!-- mockURL is a runtime-computed absolute URL opened in a new tab, not a static route -->
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a class="anchor" href={mockURL(lastMinted)} target="_blank" rel="noopener">
							Preview with mock data
						</a>
						(append <code>?mock=1</code> to any overlay URL — no token needed). Add
						<code>&amp;scale=1.5</code> to enlarge.
					</span>
				</div>
			{/if}
		</div>

		<!-- Active tokens -->
		<div class="card preset-tonal p-4">
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
		</div>
	{/if}
</div>
