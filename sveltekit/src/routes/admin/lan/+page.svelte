<script lang="ts">
	import { onMount } from 'svelte';
	import { Tabs } from '@skeletonlabs/skeleton-svelte';
	import { InfoIcon } from '@lucide/svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import CEGametypeEditor from '$lib/components/lan/CEGametypeEditor.svelte';
	import H2ProfileEditor from '$lib/components/lan/H2ProfileEditor.svelte';
	import H2GametypeEditor from '$lib/components/lan/H2GametypeEditor.svelte';
	import { lanMeta } from '$lib/utils/lansaves';
	import { toaster } from '$lib/stores/toaster';
	import type { LanMeta } from '$lib/types/lansaves';

	let meta = $state<LanMeta | null>(null);
	let loadError = $state<string | null>(null);
	let activeTab = $state('ce-gametype');

	onMount(async () => {
		try {
			meta = await lanMeta();
		} catch (e) {
			loadError = e instanceof Error ? e.message : String(e);
			toaster.error({ title: 'Load failed', description: loadError });
		}
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-6">
	<PageHeader
		title="LAN save generator"
		description="Generate Xbox-acceptable Halo: CE / Halo 2 gametype & profile saves by template-patching real samples, then serve them to the nxdk LAN client."
	/>

	{#if meta && !meta.digest_resolved}
		<div class="flex items-start gap-2 card preset-tonal-warning p-4 text-sm">
			<InfoIcon class="mt-0.5 size-4 shrink-0" />
			<div>
				<p class="font-medium">Template-patch mode — the 20-byte save digest is not recomputed.</p>
				<p class="mt-1 text-xs opacity-90">{meta.digest_note}</p>
				<p class="mt-1 text-xs opacity-90">
					Unchanged clones are byte-identical to real saves. Edited-settings files carry the
					template's (stale) digest; whether Halo re-verifies it on load is unverified. The signing
					hook is wired and ready to plug in once cracked.
				</p>
			</div>
		</div>
	{/if}

	{#if loadError}
		<div class="card preset-tonal-error p-4 text-sm">
			Could not load generator metadata: {loadError}
		</div>
	{:else if !meta}
		<div class="card preset-tonal p-6 text-sm text-surface-600-400">Loading…</div>
	{:else}
		<Tabs value={activeTab} onValueChange={(e) => (activeTab = e.value)}>
			<Tabs.List>
				<Tabs.Trigger value="ce-gametype">Halo: CE gametype</Tabs.Trigger>
				<Tabs.Trigger value="h2-profile">Halo 2 profile</Tabs.Trigger>
				<Tabs.Trigger value="h2-gametype">Halo 2 gametype</Tabs.Trigger>
				<Tabs.Indicator />
			</Tabs.List>
			<Tabs.Content value="ce-gametype">
				<CEGametypeEditor engines={meta.ce_engines} />
			</Tabs.Content>
			<Tabs.Content value="h2-profile">
				<H2ProfileEditor fields={meta.h2_appearance} />
			</Tabs.Content>
			<Tabs.Content value="h2-gametype">
				<H2GametypeEditor mode={meta.h2_gametype_mode} />
			</Tabs.Content>
		</Tabs>

		<p class="text-xs text-surface-500">
			Halo: CE has no standalone multiplayer player-profile save — only gametype variants are
			editable. The appearance/controls editor is Halo 2 only.
		</p>
	{/if}
</div>
