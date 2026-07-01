<script lang="ts">
	// Auth-free harness for the H2 appearance studio + in-game character previews.
	// Matches the repo's /debug/* convention; renders the same components the
	// /gamertag/ editor uses, with local state instead of PocketBase.
	import AppearanceStudio from '$lib/components/gamertag/AppearanceStudio.svelte';
	import CharacterPreview from '$lib/components/gamertag/CharacterPreview.svelte';
	import EmblemPreview from '$lib/components/gamertag/EmblemPreview.svelte';
	import { H2_KEYS, type Appearance } from '$lib/utils/emblem';
	import type { H2AppearanceField } from '$lib/types/lansaves';

	// "Stew" sample: cobalt/blue armor, white Jolly Roger on horizontal split.
	let appearance = $state<Appearance>({
		[H2_KEYS.armorPrimary]: 10,
		[H2_KEYS.armorSecondary]: 11,
		[H2_KEYS.emblemPrimary]: 0,
		[H2_KEYS.emblemSecondary]: 0,
		[H2_KEYS.character]: 0,
		[H2_KEYS.foreground]: 12,
		[H2_KEYS.background]: 3
	});

	// Mock the meta-supplied raw fields the pickers don't manage.
	const fields: H2AppearanceField[] = [
		{ offset: 0x11f, key: 'emblem_flags', label: 'Emblem flags' },
		{ offset: 0x12b, key: 'ctrl_a', label: 'Controller A' },
		{ offset: 0x12f, key: 'ctrl_b', label: 'Controller B' }
	];

	// A few fixed sample emblems for the gallery.
	const samples = [
		{
			armorPrimary: 2,
			armorSecondary: 0,
			emblemPrimary: 0,
			emblemSecondary: 0,
			character: 0,
			foreground: 4,
			background: 1
		},
		{
			armorPrimary: 13,
			armorSecondary: 11,
			emblemPrimary: 14,
			emblemSecondary: 14,
			character: 0,
			foreground: 26,
			background: 22
		},
		{
			armorPrimary: 6,
			armorSecondary: 4,
			emblemPrimary: 0,
			emblemSecondary: 0,
			character: 0,
			foreground: 15,
			background: 14
		},
		{
			armorPrimary: 3,
			armorSecondary: 0,
			emblemPrimary: 0,
			emblemSecondary: 0,
			character: 0,
			foreground: 27,
			background: 16
		}
	];

	// CE armor-color gallery (CE has no emblem, single-tone tint).
	const ceColors = [2, 3, 6, 5, 8, 16];
</script>

<div class="page">
	<header>
		<h1>H2 Appearance Studio — emblem creator + in-game preview</h1>
		<p>Debug harness (no auth / no PocketBase). Same components the gamertag editor mounts.</p>
	</header>

	<section class="card">
		<h2>Halo 2 — emblem creator</h2>
		<AppearanceStudio {fields} bind:appearance gamertag="CARTOG" />
	</section>

	<section class="card">
		<h2>In-game character preview — Halo 2</h2>
		<div class="gallery">
			<CharacterPreview game="h2" appearance={{ ...appearance }} gamertag="CARTOG" size={220} />
			{#each samples as s (s.foreground)}
				<CharacterPreview
					game="h2"
					appearance={{
						[H2_KEYS.armorPrimary]: s.armorPrimary,
						[H2_KEYS.armorSecondary]: s.armorSecondary,
						[H2_KEYS.emblemPrimary]: s.emblemPrimary,
						[H2_KEYS.emblemSecondary]: s.emblemSecondary,
						[H2_KEYS.character]: s.character,
						[H2_KEYS.foreground]: s.foreground,
						[H2_KEYS.background]: s.background
					}}
					gamertag="SPARTAN"
					size={200}
				/>
			{/each}
		</div>
	</section>

	<section class="card">
		<h2>In-game character preview — Halo: CE (armor color → blam.sav 0x18)</h2>
		<div class="gallery">
			{#each ceColors as c (c)}
				<CharacterPreview game="ce" colorIndex={c} gamertag="CARTOG" size={190} />
			{/each}
		</div>
	</section>

	<section class="card">
		<h2>Emblem gallery</h2>
		<div class="emblems">
			{#each samples as s (s.foreground)}
				<EmblemPreview
					emblem={{
						armorPrimary: s.armorPrimary,
						armorSecondary: s.armorSecondary,
						emblemPrimary: s.emblemPrimary,
						emblemSecondary: s.emblemSecondary,
						character: s.character,
						foreground: s.foreground,
						background: s.background
					}}
					size={120}
					ring
				/>
			{/each}
		</div>
	</section>
</div>

<style>
	.page {
		max-width: 1100px;
		margin: 0 auto;
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}
	header h1 {
		font-size: 1.4rem;
		font-weight: 700;
	}
	header p {
		opacity: 0.6;
		font-size: 0.85rem;
	}
	.card {
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 0.75rem;
		padding: 1.25rem;
		background: color-mix(in oklab, var(--color-surface-500) 6%, transparent);
	}
	.card h2 {
		font-size: 0.95rem;
		font-weight: 700;
		margin-bottom: 1rem;
		opacity: 0.85;
	}
	.gallery {
		display: flex;
		flex-wrap: wrap;
		gap: 1.5rem;
		align-items: flex-end;
	}
	.emblems {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
	}
</style>
