<script lang="ts">
	// Halo: CE player profile (blam.sav) editor — schema-driven from the backend
	// (ce_profile_fields), so the mapped surface stays in lockstep with the byte
	// map: armor color, button/thumbstick presets, and the nine Advanced Controls
	// (2026-08-07 live-verified). The name is the gamertag above; the generator
	// signs the file. Binds a plain CeProfileSettings the page persists.
	import { onMount } from 'svelte';
	import SchemaField from '$lib/components/creator/SchemaField.svelte';
	import { lanMeta } from '$lib/utils/lansaves';
	import type { CEField, CESection } from '$lib/types/lansaves';
	import type { CeProfileSettings } from '$lib/types/gamertag';

	let {
		settings = $bindable()
	}: {
		settings: CeProfileSettings;
	} = $props();

	let fields = $state<CEField[]>([]);
	let sections = $state<CESection[]>([]);
	let loaded = $state(false);

	onMount(() => {
		void lanMeta()
			.then((m) => {
				fields = m.ce_profile_fields ?? [];
				sections = m.ce_profile_sections ?? [];
			})
			.catch(() => {})
			.finally(() => (loaded = true));
	});

	function fieldsFor(section: string): CEField[] {
		return fields.filter((f) => f.section === section);
	}

	// SchemaField binds boolean | number | undefined; CeProfileSettings values
	// are exactly that, keyed by the schema key. $derived so a parent reassigning
	// the whole settings object (e.g. after load) stays tracked.
	const s = $derived(settings as Record<string, boolean | number | undefined>);
</script>

<div class="space-y-5">
	<p class="text-xs text-surface-600-400">
		Your gamertag is the in-game name. Blank fields keep the fresh-profile factory default.
	</p>

	{#if !loaded}
		<p class="text-sm text-surface-500">Loading settings…</p>
	{:else if fields.length === 0}
		<p class="text-sm text-error-500">Couldn't load the profile schema.</p>
	{:else}
		{#each sections as sec (sec.id)}
			{@const secFields = fieldsFor(sec.id)}
			{#if secFields.length}
				<section class="flex flex-col gap-3">
					<h3 class="text-xs font-semibold tracking-wide text-surface-500 uppercase">
						{sec.label}
					</h3>
					<div class="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
						{#each secFields as f (f.key)}
							<SchemaField field={f} bind:value={s[f.key]} />
						{/each}
					</div>
				</section>
			{/if}
		{/each}
	{/if}
</div>
