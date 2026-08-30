<script lang="ts">
	// Settings → Halo: CE (WIP chip per the handoff). Armor color + the two
	// in-game control presets as cycler rows + the nine Advanced Controls in a
	// collapsible — ALL schema-driven from the backend (ce_profile_fields, the
	// live-verified byte map), so the surface never drifts from what the signed
	// blam.sav actually holds. Save & regenerate upserts ce_profiles; the server
	// hook reads the default gamertag off the user and re-signs the save.
	import { ChevronDownIcon, DownloadIcon, LoaderIcon } from '@lucide/svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CharacterPreview from '$lib/components/gamertag/CharacterPreview.svelte';
	import SchemaField from '$lib/components/creator/SchemaField.svelte';
	import ActionBar from '$lib/components/settings/ActionBar.svelte';
	import CyclerRow from '$lib/components/settings/CyclerRow.svelte';
	import SwatchGrid from '$lib/components/settings/SwatchGrid.svelte';
	import WipChip from '$lib/components/settings/WipChip.svelte';
	import { CE_COLORS, colorName } from '$lib/utils/emblem';
	import { downloadRecordFile, formatBytes } from '$lib/utils/gamertag';
	import { isDeferred } from '$lib/types/gamertag';
	import type { CeProfileRecord, CeProfileSettings } from '$lib/types/gamertag';
	import type { CEField } from '$lib/types/lansaves';

	let {
		active,
		narrow,
		gamertag,
		fields,
		settings = $bindable(),
		record,
		dirty,
		busy,
		onsave
	}: {
		active: boolean;
		narrow: boolean;
		gamertag: string;
		fields: CEField[];
		settings: CeProfileSettings;
		record: CeProfileRecord | null;
		dirty: boolean;
		busy: boolean;
		onsave: () => void;
	} = $props();

	let advOpen = $state(false);

	const tagUpper = $derived((gamertag || 'unnamed').toUpperCase());
	const colorIdx = $derived(Number(settings.color ?? 0));
	const ceMeta = $derived(`Halo: CE · ${colorName(CE_COLORS, colorIdx)}`);

	// Cycler rows come from the schema's controller section so the option lists
	// stay in lockstep with the byte map (the mock's from-memory lists differ).
	const controllerFields = $derived(fields.filter((f) => f.section === 'controller'));
	const advancedFields = $derived(fields.filter((f) => f.section === 'advanced'));
	const s = $derived(settings as Record<string, boolean | number | undefined>);

	function optionLabels(f: CEField): string[] {
		return (f.options ?? []).map((o) => o.label);
	}
	function optionIndex(f: CEField): number {
		const v = Number(s[f.key] ?? 0);
		const idx = (f.options ?? []).findIndex((o) => o.value === v);
		return idx >= 0 ? idx : 0;
	}
	function pickOption(f: CEField, i: number) {
		const opt = (f.options ?? [])[i];
		if (opt) s[f.key] = opt.value;
	}

	const saveNote = $derived.by(() => {
		const info = record?.save_info;
		if (!info || isDeferred(info)) return 'No generated save yet — Save & regenerate builds it.';
		const f = info.files?.find((x) => x.name.endsWith('.sav')) ?? info.files?.[0];
		const when = record?.updated ? new Date(record.updated.replace(' ', 'T')).toLocaleString() : '';
		return `Generated save · ${f?.name ?? 'blam.sav'} · ${formatBytes(info.total_bytes)} · ${info.digest?.resolved ? 'signed' : 'unsigned'} ${when}`;
	});

	async function download() {
		if (!record) return;
		await downloadRecordFile(record, 'save_bundle', `${gamertag || 'gamertag'}-ce-profile.tar`);
	}
</script>

{#if active}
	<div class="flex flex-col gap-5">
		{#if narrow}
			<!-- sticky mini-preview: floats over the swatch grids while scrolling -->
			<div
				class="sticky top-2 z-4 flex items-center gap-3.5 rounded-xl border border-surface-500/25 bg-surface-50-950/95 px-3.5 py-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_8px_24px_rgba(0,0,0,0.45)] backdrop-blur-sm"
			>
				<CharacterPreview game="ce" colorIndex={colorIdx} size={78} />
				<span class="min-w-0 flex-1 text-[11px] opacity-60">{ceMeta} armor</span>
			</div>
		{/if}

		<!-- header -->
		<Card class="flex flex-wrap items-center gap-3.5">
			<div class="flex min-w-55 flex-1 flex-col gap-0.5">
				<span class="inline-flex items-center gap-2">
					<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
						Halo: CE profile
					</span>
					<WipChip />
				</span>
				<span class="font-[Orbitron] text-xl font-extrabold tracking-[0.06em]">{tagUpper}</span>
			</div>
			<span class="max-w-85 text-[11.5px] opacity-60">
				Carries your default gamertag — pick it under Stream. Blank fields keep the fresh-profile
				factory defaults.
			</span>
		</Card>

		<div class="grid grid-cols-[repeat(auto-fit,minmax(min(420px,100%),1fr))] items-start gap-5">
			<!-- Appearance -->
			<Card class="flex flex-col gap-3.5">
				<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
					Appearance
				</span>
				{#if !narrow}
					<div class="flex justify-center rounded-[10px] bg-black/20 p-4">
						<CharacterPreview game="ce" colorIndex={colorIdx} gamertag={tagUpper} size={170} />
					</div>
				{/if}
				<SwatchGrid
					label="Armor color"
					colors={CE_COLORS}
					selected={colorIdx}
					onpick={(i) => (settings.color = i)}
				/>
			</Card>

			<!-- Controls -->
			<Card class="flex flex-col gap-3.5">
				<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
					Controls
				</span>
				<div class="flex flex-col gap-2">
					{#each controllerFields as f (f.key)}
						<CyclerRow
							label={f.label}
							options={optionLabels(f)}
							value={optionIndex(f)}
							onchange={(i) => pickOption(f, i)}
						/>
					{/each}
				</div>
				<button
					type="button"
					class="flex w-full items-center justify-between gap-2 border-t border-surface-200-800 pt-3 text-left text-xs font-semibold text-surface-600-400"
					onclick={() => (advOpen = !advOpen)}
				>
					<span>Advanced controls — sensitivity, deadzones, response ({advancedFields.length})</span
					>
					<span
						class="inline-flex flex-none transition-transform duration-150 {advOpen
							? 'rotate-180'
							: ''}"
					>
						<ChevronDownIcon class="size-3.5" />
					</span>
				</button>
				{#if advOpen}
					<div class="grid grid-cols-[repeat(auto-fit,minmax(140px,1fr))] gap-3">
						{#each advancedFields as f (f.key)}
							<SchemaField field={f} bind:value={s[f.key]} disabled={busy} />
						{/each}
					</div>
				{/if}
				<p class="text-[10.5px] leading-relaxed opacity-50">
					Presets and the nine advanced controls map 1:1 onto the signed profile bytes —
					live-verified against real saves.
				</p>
			</Card>
		</div>

		<ActionBar note={saveNote} mono dot={!!record?.save_bundle}>
			<button class="btn preset-tonal" onclick={download} disabled={busy || !record?.save_bundle}>
				<DownloadIcon class="size-4" /><span>Download</span>
			</button>
			<button class="btn preset-filled" onclick={onsave} disabled={busy || !dirty}>
				{#if busy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Save &amp; regenerate</span>
			</button>
		</ActionBar>
	</div>
{/if}
