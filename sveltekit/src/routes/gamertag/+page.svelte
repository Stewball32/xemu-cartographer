<script lang="ts">
	import { onMount } from 'svelte';
	import { LoaderIcon, SaveIcon, IdCardIcon } from '@lucide/svelte';
	import { ClientResponseError } from 'pocketbase';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { toaster, describeAsyncError } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import H2AppearanceEditor from '$lib/components/gamertag/H2AppearanceEditor.svelte';
	import CESettingsEditor from '$lib/components/gamertag/CESettingsEditor.svelte';
	import SaveResultCard from '$lib/components/gamertag/SaveResultCard.svelte';
	import { lanMeta } from '$lib/utils/lansaves';
	import { fetchDefaultGamertag, GAMERTAG_MAX_LEN } from '$lib/utils/gamertag';
	import type { H2AppearanceField } from '$lib/types/lansaves';
	import type { CeProfileRecord, CeProfileSettings, H2ProfileRecord } from '$lib/types/gamertag';

	let loading = $state(true);
	let saving = $state(false);
	let loadError = $state<string | null>(null);

	let appearanceFields = $state<H2AppearanceField[]>([]);
	let gamertag = $state('');
	let appearance = $state<Record<string, number | undefined>>({});
	let ceSettings = $state<CeProfileSettings>({});

	let h2Record = $state<H2ProfileRecord | null>(null);
	let ceRecord = $state<CeProfileRecord | null>(null);

	async function firstOrNull<T>(collection: string, filter: string): Promise<T | null> {
		try {
			return await pb.collection(collection).getFirstListItem<T>(filter);
		} catch (err) {
			if (err instanceof ClientResponseError && err.status === 404) return null;
			throw err;
		}
	}

	async function load() {
		const uid = auth.user?.id;
		if (!uid) {
			loadError = 'Not authenticated.';
			loading = false;
			return;
		}
		try {
			loading = true;
			const meta = await lanMeta();
			appearanceFields = meta.h2_appearance;

			// users.gamertag is the single source of truth — fetch a fresh user
			// record. (Cast: the generated PB types lag the new field until typegen
			// re-runs against a live server.)
			let userGamertag = '';
			try {
				const u = await pb.collection('users').getOne(uid);
				userGamertag = (((u as Record<string, unknown>).gamertag as string) ?? '').trim();
			} catch {
				/* fall back below */
			}

			h2Record = await firstOrNull<H2ProfileRecord>('h2_profiles', `user = "${uid}"`);
			ceRecord = await firstOrNull<CeProfileRecord>('ce_profiles', `user = "${uid}"`);

			// First visit (no gamertag yet): suggest the M07 default gamertag or username.
			const fallback = (await fetchDefaultGamertag()) ?? auth.user?.username ?? '';
			gamertag = userGamertag || fallback;
			appearance = { ...(h2Record?.appearance ?? {}) };
			ceSettings = { ...(ceRecord?.settings ?? {}) };
		} catch (err) {
			loadError = describeAsyncError(err);
			toaster.error({ title: 'Load failed', description: loadError });
		} finally {
			loading = false;
		}
	}

	function cleanAppearance(): Record<string, number> {
		const out: Record<string, number> = {};
		for (const [k, v] of Object.entries(appearance)) {
			if (typeof v === 'number' && Number.isFinite(v)) out[k] = v;
		}
		return out;
	}

	async function save() {
		const tag = gamertag.trim();
		if (!tag) {
			toaster.error({ title: 'Invalid', description: 'A gamertag is required.' });
			return;
		}
		const uid = auth.user?.id;
		if (!uid) return;

		saving = true;
		try {
			const ap = cleanAppearance();
			// 1. The gamertag lives on the user record (single source of truth).
			//    Updating it cascades a server-side regen to any existing profiles.
			await pb.collection('users').update(uid, { gamertag: tag });

			// 2. Upsert the H2 profile (appearance only — the hook reads the
			//    gamertag from the user and regenerates the signed save).
			if (h2Record) {
				h2Record = await pb
					.collection('h2_profiles')
					.update<H2ProfileRecord>(h2Record.id, { appearance: ap });
			} else {
				h2Record = await pb
					.collection('h2_profiles')
					.create<H2ProfileRecord>({ user: uid, appearance: ap });
			}

			// 3. Upsert the CE profile (full profile; generation deferred until the
			//    CE format research lands — the hook stamps a deferred marker).
			if (ceRecord) {
				ceRecord = await pb
					.collection('ce_profiles')
					.update<CeProfileRecord>(ceRecord.id, { settings: ceSettings });
			} else {
				ceRecord = await pb
					.collection('ce_profiles')
					.create<CeProfileRecord>({ user: uid, settings: ceSettings });
			}

			toaster.success({ title: 'Saved', description: `Gamertag "${tag}" — files regenerated.` });
		} catch (err) {
			toaster.error({ title: 'Save failed', description: describeAsyncError(err) });
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-6">
	<PageHeader
		title="Your gamertag"
		description="One gamertag, two Halo profiles. Edit your Halo: CE and Halo 2 player profiles side by side — both carry this gamertag and are stored separately. Saving regenerates the actual Xbox save files, ready for your box to download."
	/>

	{#if loadError}
		<div class="card preset-tonal-error p-4 text-sm">{loadError}</div>
	{:else if loading}
		<div class="card preset-tonal p-6 text-sm text-surface-600-400">Loading…</div>
	{:else}
		<Card class="space-y-3">
			<label class="label">
				<span class="flex items-center gap-2 font-medium">
					<IdCardIcon class="size-4" /> Gamertag (in-game name)
				</span>
				<input
					type="text"
					class="input max-w-sm"
					bind:value={gamertag}
					maxlength={GAMERTAG_MAX_LEN}
					placeholder="e.g. CARTOG"
				/>
			</label>
			<p class="text-xs text-surface-600-400">
				Your in-game name (separate from your account username), used by both Halo profiles. Capped
				at {GAMERTAG_MAX_LEN} printable characters to fit both games.
			</p>
			<div>
				<button class="btn preset-filled" onclick={save} disabled={saving}>
					{#if saving}<LoaderIcon class="size-4 animate-spin" />{:else}<SaveIcon
							class="size-4"
						/>{/if}
					<span>Save &amp; regenerate</span>
				</button>
			</div>
		</Card>

		<div class="grid gap-6 lg:grid-cols-2">
			<div class="flex flex-col gap-4">
				<Card class="space-y-4">
					<header>
						<h3 class="h4">Halo 2 profile</h3>
						<p class="text-sm text-surface-600-400">
							Full appearance &amp; controls — generates a 500-byte <code class="font-mono"
								>profile</code
							> save.
						</p>
					</header>
					<H2AppearanceEditor fields={appearanceFields} bind:appearance />
				</Card>
				<SaveResultCard
					record={h2Record}
					info={h2Record?.save_info ?? null}
					downloadName="{gamertag || 'gamertag'}-h2-profile.tar"
				/>
			</div>

			<div class="flex flex-col gap-4">
				<Card class="space-y-4">
					<header>
						<h3 class="h4">Halo: CE profile</h3>
						<p class="text-sm text-surface-600-400">
							Name + armor color + control presets — generates a signed
							<code class="font-mono">blam.sav</code>.
						</p>
					</header>
					<CESettingsEditor bind:settings={ceSettings} />
				</Card>
				<SaveResultCard
					record={ceRecord}
					info={ceRecord?.save_info ?? null}
					downloadName="{gamertag || 'gamertag'}-ce-profile.tar"
				/>
			</div>
		</div>
	{/if}
</div>
