<script lang="ts">
	// Settings — the unified, tabbed player-identity page (settings redesign).
	// One page owns account profile (General), the per-game Halo: CE / Halo 2
	// profile config (WIP tabs), the stream nameplate + gamertags (Stream), and
	// OAuth connections (Accounts). CONSOLIDATES the old /gamertag/ page (which
	// now redirects here) and the previous settings layout; the old Teams
	// section is PARKED until the Teams tab gets its own design pass.
	//
	// Persistence is the pre-redesign logic behind the new UI: profile upserts
	// regenerate signed saves server-side, the default gamertag (Stream) is the
	// name both game profiles carry (a users hook syncs users.gamertag + regen),
	// and motto + nameplate feed the overlays via /api/public/profiles.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { GamepadIcon, LinkIcon, SwordsIcon, TvIcon, UserIcon } from '@lucide/svelte';
	import { ClientResponseError } from 'pocketbase';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import WipChip from '$lib/components/settings/WipChip.svelte';
	import GeneralTab from '$lib/components/settings/GeneralTab.svelte';
	import CETab from '$lib/components/settings/CETab.svelte';
	import H2Tab from '$lib/components/settings/H2Tab.svelte';
	import StreamTab from '$lib/components/settings/StreamTab.svelte';
	import AccountsTab from '$lib/components/settings/AccountsTab.svelte';
	import { lanMeta } from '$lib/utils/lansaves';
	import { fetchIdentity, type MeGamertag } from '$lib/utils/identity';
	import type { CEField } from '$lib/types/lansaves';
	import type { CeProfileRecord, CeProfileSettings, H2ProfileRecord } from '$lib/types/gamertag';
	import type { Appearance } from '$lib/utils/emblem';

	type TabKey = 'general' | 'ce' | 'h2' | 'stream' | 'accounts';
	const TABS: { key: TabKey; icon: typeof UserIcon; label: string; wip: boolean }[] = [
		{ key: 'general', icon: UserIcon, label: 'General', wip: false },
		{ key: 'ce', icon: GamepadIcon, label: 'Halo: CE', wip: true },
		{ key: 'h2', icon: SwordsIcon, label: 'Halo 2', wip: true },
		{ key: 'stream', icon: TvIcon, label: 'Stream', wip: false },
		{ key: 'accounts', icon: LinkIcon, label: 'Accounts', wip: false }
	];
	const isTab = (v: string | null): v is TabKey =>
		v === 'general' || v === 'ce' || v === 'h2' || v === 'stream' || v === 'accounts';

	const tab = $derived.by<TabKey>(() => {
		const q = page.url.searchParams.get('tab');
		return isTab(q) ? q : 'general';
	});
	function switchTab(k: TabKey) {
		const url = new URL(page.url);
		if (k === 'general') url.searchParams.delete('tab');
		else url.searchParams.set('tab', k);
		// Same-page query-param update — resolve() has nothing to resolve here.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		void goto(url, { replaceState: true, noScroll: true, keepFocus: true });
	}

	let innerWidth = $state(1280);
	const narrow = $derived(innerWidth < 900);

	// ── Shared identity + profile state (loaded once) ───────────────────────
	let loading = $state(true);
	let gamertags = $state<MeGamertag[]>([]);
	let defaultGamertagId = $state<string | null>(null);
	let defaultTag = $state('');

	let ceFields = $state<CEField[]>([]);
	let ceRecord = $state<CeProfileRecord | null>(null);
	let ceSettings = $state<CeProfileSettings>({});
	let ceBaseline = $state('{}');
	let h2Record = $state<H2ProfileRecord | null>(null);
	let h2Appearance = $state<Appearance>({});
	let h2Baseline = $state('{}');
	let gameBusy = $state(false);

	let motto = $state('');
	let nameplateId = $state('');
	let savedMotto = $state('');
	let savedNameplateId = $state('');
	let streamBusy = $state(false);

	const ceDirty = $derived(JSON.stringify(ceSettings) !== ceBaseline);
	const h2Dirty = $derived(JSON.stringify(h2Appearance) !== h2Baseline);

	async function firstOrNull<T>(collection: string, filter: string): Promise<T | null> {
		try {
			return await pb.collection(collection).getFirstListItem<T>(filter);
		} catch (err) {
			if (err instanceof ClientResponseError && err.status === 404) return null;
			throw err;
		}
	}

	async function reloadIdentity() {
		const id = await fetchIdentity();
		gamertags = id?.gamertags ?? [];
		defaultGamertagId = id?.default_gamertag?.id ?? null;
		defaultTag = id?.default_gamertag?.tag ?? '';
	}

	async function load() {
		const uid = auth.user?.id;
		if (!uid) return;
		try {
			loading = true;
			const [meta, user] = await Promise.all([
				lanMeta(),
				pb.collection('users').getOne(uid),
				reloadIdentity()
			]);
			ceFields = meta.ce_profile_fields ?? [];
			const u = user as unknown as Record<string, unknown>;
			savedMotto = motto = String(u.motto ?? '');
			savedNameplateId = nameplateId = String(u.nameplate ?? '');

			ceRecord = await firstOrNull<CeProfileRecord>('ce_profiles', `user = "${uid}"`);
			h2Record = await firstOrNull<H2ProfileRecord>('h2_profiles', `user = "${uid}"`);
			ceSettings = { ...(ceRecord?.settings ?? {}) };
			ceBaseline = JSON.stringify(ceSettings);
			h2Appearance = { ...(h2Record?.appearance ?? {}) };
			h2Baseline = JSON.stringify(h2Appearance);
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function cleanAppearance(): Record<string, number> {
		const out: Record<string, number> = {};
		for (const [k, v] of Object.entries(h2Appearance)) {
			if (typeof v === 'number' && Number.isFinite(v)) out[k] = v;
		}
		return out;
	}

	async function saveCE() {
		const uid = auth.user?.id;
		if (!uid) return;
		gameBusy = true;
		try {
			const rec = await toastPromise(
				ceRecord
					? pb
							.collection('ce_profiles')
							.update<CeProfileRecord>(ceRecord.id, { settings: ceSettings })
					: pb
							.collection('ce_profiles')
							.create<CeProfileRecord>({ user: uid, settings: ceSettings }),
				{
					loading: { title: 'Saving', description: 'Regenerating blam.sav' },
					success: { title: 'Saved', description: 'Halo: CE profile regenerated.' },
					errorTitle: 'Save failed'
				}
			);
			ceRecord = rec;
			ceSettings = { ...(rec.settings ?? {}) };
			ceBaseline = JSON.stringify(ceSettings);
		} catch {
			/* toast shown */
		} finally {
			gameBusy = false;
		}
	}

	async function saveH2() {
		const uid = auth.user?.id;
		if (!uid) return;
		gameBusy = true;
		try {
			const ap = cleanAppearance();
			const rec = await toastPromise(
				h2Record
					? pb.collection('h2_profiles').update<H2ProfileRecord>(h2Record.id, { appearance: ap })
					: pb.collection('h2_profiles').create<H2ProfileRecord>({ user: uid, appearance: ap }),
				{
					loading: { title: 'Saving', description: 'Regenerating the H2 profile' },
					success: { title: 'Saved', description: 'Halo 2 profile regenerated.' },
					errorTitle: 'Save failed'
				}
			);
			h2Record = rec;
			h2Appearance = { ...(rec.appearance ?? {}) };
			h2Baseline = JSON.stringify(h2Appearance);
		} catch {
			/* toast shown */
		} finally {
			gameBusy = false;
		}
	}

	async function saveStream() {
		const uid = auth.user?.id;
		if (!uid) return;
		streamBusy = true;
		try {
			await toastPromise(
				pb.collection('users').update(uid, { motto: motto.trim(), nameplate: nameplateId }),
				{
					loading: { title: 'Saving' },
					success: { title: 'Saved', description: 'Your nameplate is live on the overlays.' },
					errorTitle: 'Save failed'
				}
			);
			savedMotto = motto = motto.trim();
			savedNameplateId = nameplateId;
		} catch {
			/* toast shown */
		} finally {
			streamBusy = false;
		}
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to edit your settings.' });
			return;
		}
		void load();
	});
</script>

<svelte:window bind:innerWidth />

<div class="mx-auto flex w-full max-w-290 flex-col gap-4.5 p-4 sm:p-6">
	<PageHeader
		title="Settings"
		description="Your account, your identity, your stream presence — all in one place."
	/>

	<!-- tab row -->
	<div class="flex flex-wrap gap-1.5 border-b border-surface-200-800 pb-3">
		{#each TABS as t (t.key)}
			<button
				type="button"
				class="inline-flex items-center gap-2 rounded-lg border px-3.5 py-2 text-[13px] font-semibold transition-colors
					{tab === t.key
					? 'border-primary-500/40 bg-primary-500/15 text-primary-600-400'
					: 'border-transparent text-surface-600-400 hover:text-surface-950-50'}"
				aria-current={tab === t.key ? 'page' : undefined}
				onclick={() => switchTab(t.key)}
			>
				<t.icon class="size-4" />
				<span>{t.label}</span>
				{#if t.wip}<WipChip />{/if}
			</button>
		{/each}
	</div>

	{#if loading}
		<p class="p-4 text-sm text-surface-500">Loading…</p>
	{:else}
		<GeneralTab active={tab === 'general'} {h2Appearance} />
		<CETab
			active={tab === 'ce'}
			{narrow}
			gamertag={defaultTag}
			fields={ceFields}
			bind:settings={ceSettings}
			record={ceRecord}
			dirty={ceDirty}
			busy={gameBusy}
			onsave={saveCE}
		/>
		<H2Tab
			active={tab === 'h2'}
			{narrow}
			gamertag={defaultTag}
			bind:appearance={h2Appearance}
			record={h2Record}
			dirty={h2Dirty}
			busy={gameBusy}
			onsave={saveH2}
		/>
		<StreamTab
			active={tab === 'stream'}
			{gamertags}
			{defaultGamertagId}
			{defaultTag}
			bind:motto
			bind:nameplateId
			{savedMotto}
			{savedNameplateId}
			busy={streamBusy}
			onsave={saveStream}
			onreloadIdentity={reloadIdentity}
		/>
		<AccountsTab active={tab === 'accounts'} />
	{/if}
</div>
