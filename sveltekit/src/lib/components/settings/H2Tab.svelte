<script lang="ts">
	// Settings → Halo 2 (WIP chip per the handoff). Appearance = the emblem
	// creator (armor / emblem sub-tabs) against the CONFIRMED 0x118 appearance
	// block, compositing with the same emblem utils as everywhere else. The
	// Controls panel is a DISABLED preview: the six in-game options are designed,
	// but their byte offsets aren't mapped yet (only two provisional mystery
	// bytes exist) — the cyclers wire up when the offsets are live-verified.
	import { DownloadIcon, LoaderIcon } from '@lucide/svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CharacterPreview from '$lib/components/gamertag/CharacterPreview.svelte';
	import EmblemPreview from '$lib/components/gamertag/EmblemPreview.svelte';
	import RawSvg from '$lib/components/gamertag/RawSvg.svelte';
	import ActionBar from '$lib/components/settings/ActionBar.svelte';
	import CyclerRow from '$lib/components/settings/CyclerRow.svelte';
	import SwatchGrid from '$lib/components/settings/SwatchGrid.svelte';
	import WipChip from '$lib/components/settings/WipChip.svelte';
	import {
		DEFAULT_EMBLEM,
		FOREGROUNDS,
		H2_BACKGROUNDS,
		H2_CHARACTERS,
		H2_COLORS,
		H2_KEYS,
		colorHex,
		colorName,
		emblemUnset,
		foregroundSvg,
		readEmblem,
		seedEmblem,
		type Appearance,
		type EmblemState
	} from '$lib/utils/emblem';
	import { backgroundSvg } from '$lib/utils/emblem-backgrounds';
	import { downloadRecordFile, formatBytes } from '$lib/utils/gamertag';
	import { isDeferred } from '$lib/types/gamertag';
	import type { H2ProfileRecord } from '$lib/types/gamertag';

	let {
		active,
		narrow,
		gamertag,
		appearance = $bindable(),
		record,
		dirty,
		busy,
		onsave
	}: {
		active: boolean;
		narrow: boolean;
		gamertag: string;
		appearance: Appearance;
		record: H2ProfileRecord | null;
		dirty: boolean;
		busy: boolean;
		onsave: () => void;
	} = $props();

	let apTab = $state<'armor' | 'emblem'>('armor');

	// Seed a complete default emblem on first use so the preview is never blank.
	$effect(() => {
		if (active && emblemUnset(appearance)) seedEmblem(appearance);
	});

	const FIELD_KEY: Record<keyof EmblemState, string> = {
		armorPrimary: H2_KEYS.armorPrimary,
		armorSecondary: H2_KEYS.armorSecondary,
		emblemPrimary: H2_KEYS.emblemPrimary,
		emblemSecondary: H2_KEYS.emblemSecondary,
		character: H2_KEYS.character,
		foreground: H2_KEYS.foreground,
		background: H2_KEYS.background
	};
	const emblem = $derived(readEmblem(appearance ?? {}, DEFAULT_EMBLEM));
	function select(field: keyof EmblemState, value: number) {
		appearance[FIELD_KEY[field]] = value;
	}

	const tagUpper = $derived((gamertag || 'unnamed').toUpperCase());
	const armorA = $derived(colorHex(H2_COLORS, emblem.armorPrimary));
	const emblemDesc = $derived(
		`${FOREGROUNDS[emblem.foreground]?.label ?? ''} in ${colorName(H2_COLORS, emblem.emblemPrimary)}/${colorName(H2_COLORS, emblem.emblemSecondary)} on ${H2_BACKGROUNDS[emblem.background]?.label ?? ''} · ${colorName(H2_COLORS, emblem.armorPrimary)}/${colorName(H2_COLORS, emblem.armorSecondary)} armor`
	);

	// Tile thumbs — same inner-SVG builders as the emblem studio.
	function fgThumb(i: number): string {
		return (
			`<rect x="0" y="0" width="100" height="100" rx="14" fill="#11161c"/>` +
			`<g transform="translate(13,13) scale(0.74)">${foregroundSvg(i, '#e9eef3', '#9fb4c6', `sfg${i}`)}</g>`
		);
	}
	function bgThumb(i: number): string {
		return backgroundSvg(i, armorA, colorHex(H2_COLORS, emblem.armorSecondary), `sbg${i}`);
	}

	// The six in-game options (menu order) — DISPLAY ONLY until the H2 profile
	// control bytes are mapped. Defaults per the design (vibration on).
	const H2_CONTROL_PREVIEW: { label: string; options: string[]; value: number }[] = [
		{
			label: 'Thumbstick layout',
			options: ['Default', 'Southpaw', 'Legacy', 'Legacy Southpaw'],
			value: 0
		},
		{ label: 'Button layout', options: ['Default', 'Southpaw', 'Boxer', 'Green Thumb'], value: 0 },
		{
			label: 'Look sensitivity',
			options: ['1', '2', '3', '4', '5', '6', '7', '8', '9', '10'],
			value: 2
		},
		{ label: 'Look inversion', options: ['Disabled', 'Enabled'], value: 0 },
		{ label: 'Automatic look centering', options: ['Disabled', 'Enabled'], value: 0 },
		{ label: 'Controller vibration', options: ['Disabled', 'Enabled'], value: 1 }
	];

	const saveNote = $derived.by(() => {
		const info = record?.save_info;
		if (!info || isDeferred(info)) return 'No generated save yet — Save & regenerate builds it.';
		const f = info.files?.find((x) => x.name === 'profile') ?? info.files?.[0];
		const when = record?.updated ? new Date(record.updated.replace(' ', 'T')).toLocaleString() : '';
		return `Generated save · ${f?.name ?? 'profile'} · ${formatBytes(info.total_bytes)} · ${info.digest?.resolved ? 'signed' : 'unsigned'} ${when}`;
	});

	async function download() {
		if (!record) return;
		await downloadRecordFile(record, 'save_bundle', `${gamertag || 'gamertag'}-h2-profile.tar`);
	}
</script>

{#if active}
	<div class="flex flex-col gap-5">
		{#if narrow}
			<div
				class="sticky top-2 z-4 flex items-center gap-3.5 rounded-xl border border-surface-500/25 bg-surface-50-950/95 px-3.5 py-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.1),0_8px_24px_rgba(0,0,0,0.45)] backdrop-blur-sm"
			>
				<CharacterPreview game="h2" {appearance} size={78} />
				<div
					class="size-13.5 flex-none overflow-hidden rounded-[12%] shadow-[0_0_0_1px_rgba(255,255,255,0.18)]"
				>
					<EmblemPreview {appearance} size={54} />
				</div>
				<span class="min-w-0 flex-1 text-[11px] opacity-60">{emblemDesc}</span>
			</div>
		{/if}

		<!-- header -->
		<Card class="flex flex-wrap items-center gap-3.5">
			<div
				class="size-11 flex-none overflow-hidden rounded-[12%] shadow-[0_0_0_1px_rgba(255,255,255,0.18)]"
			>
				<EmblemPreview {appearance} size={44} />
			</div>
			<div class="flex min-w-50 flex-1 flex-col gap-0.5">
				<span class="inline-flex items-center gap-2">
					<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
						Halo 2 profile
					</span>
					<WipChip />
				</span>
				<span class="font-[Orbitron] text-xl font-extrabold tracking-[0.06em]">{tagUpper}</span>
			</div>
			<span class="max-w-85 text-[11.5px] opacity-60">
				Carries your default gamertag — pick it under Stream.
			</span>
		</Card>

		<div class="grid grid-cols-[repeat(auto-fit,minmax(min(420px,100%),1fr))] items-start gap-5">
			<!-- Appearance -->
			<Card class="flex flex-col gap-3.5">
				<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
					Appearance
				</span>
				{#if !narrow}
					<div class="flex flex-wrap items-end justify-center gap-7 rounded-[10px] bg-black/20 p-4">
						<CharacterPreview game="h2" {appearance} gamertag={tagUpper} size={175} />
						<div class="flex flex-col items-center gap-1.5">
							<div
								class="size-27.5 overflow-hidden rounded-[12%] shadow-[0_0_0_1px_rgba(255,255,255,0.18),0_0_22px_rgba(61,98,224,0.3)]"
							>
								<EmblemPreview {appearance} size={110} />
							</div>
							<span class="text-[9.5px] font-bold tracking-[0.3em] uppercase opacity-50">
								Emblem
							</span>
						</div>
					</div>
					<p class="text-center text-[11px] opacity-60">{emblemDesc}</p>
				{/if}

				<!-- Armor / Emblem segmented -->
				<div class="grid grid-cols-2 gap-2">
					{#each [['armor', 'Armor'], ['emblem', 'Emblem']] as const as [key, label] (key)}
						<button
							type="button"
							class="rounded-lg border px-1 py-2 text-center text-xs font-semibold whitespace-nowrap transition-colors
								{apTab === key
								? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
								: 'border-surface-500/20 bg-surface-100-900 text-surface-600-400'}"
							onclick={() => (apTab = key)}
						>
							{label}
						</button>
					{/each}
				</div>

				{#if apTab === 'armor'}
					<SwatchGrid
						label="Armor primary"
						colors={H2_COLORS}
						selected={emblem.armorPrimary}
						onpick={(i) => select('armorPrimary', i)}
					/>
					<SwatchGrid
						label="Armor secondary"
						colors={H2_COLORS}
						selected={emblem.armorSecondary}
						onpick={(i) => select('armorSecondary', i)}
					/>
					<div class="flex flex-col gap-1.5">
						<span class="text-xs text-surface-600-400">Character</span>
						<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
							{#each H2_CHARACTERS as ch (ch.index)}
								<button
									type="button"
									class="chip min-h-9.5 w-full justify-center {emblem.character === ch.index
										? 'preset-filled-primary-500'
										: 'preset-tonal'}"
									aria-pressed={emblem.character === ch.index}
									onclick={() => select('character', ch.index)}
								>
									{ch.label}
								</button>
							{/each}
						</div>
					</div>
				{:else}
					<div class="flex flex-col gap-1.5">
						<div class="flex justify-between gap-2 text-xs text-surface-600-400">
							<span>Foreground</span>
							<span class="font-semibold opacity-70">
								{FOREGROUNDS[emblem.foreground]?.label} · {FOREGROUNDS.length} total
							</span>
						</div>
						<div
							class="grid max-h-49 grid-cols-[repeat(auto-fill,minmax(44px,1fr))] gap-1.5 overflow-x-hidden overflow-y-auto overscroll-contain rounded-[9px] bg-black/25 p-1.5"
						>
							{#each FOREGROUNDS as f (f.index)}
								<button
									type="button"
									title={f.label}
									aria-pressed={emblem.foreground === f.index}
									class="aspect-square min-w-0 overflow-hidden rounded-[7px] bg-[#11161c] {emblem.foreground ===
									f.index
										? 'outline-2 outline-offset-1 outline-primary-500'
										: 'border border-surface-500/30'}"
									onclick={() => select('foreground', f.index)}
								>
									<RawSvg inner={fgThumb(f.index)} />
								</button>
							{/each}
						</div>
					</div>
					<div class="flex flex-col gap-1.5">
						<div class="flex justify-between gap-2 text-xs text-surface-600-400">
							<span>Background</span>
							<span class="font-semibold opacity-70"
								>{H2_BACKGROUNDS[emblem.background]?.label}</span
							>
						</div>
						<div
							class="grid max-h-27.5 grid-cols-[repeat(auto-fill,minmax(44px,1fr))] gap-1.5 overflow-x-hidden overflow-y-auto rounded-[9px] bg-black/25 p-1.5"
						>
							{#each H2_BACKGROUNDS as bg (bg.index)}
								<button
									type="button"
									title={bg.label}
									aria-pressed={emblem.background === bg.index}
									class="aspect-square min-w-0 overflow-hidden rounded-[7px] bg-[#11161c] {emblem.background ===
									bg.index
										? 'outline-2 outline-offset-1 outline-primary-500'
										: 'border border-surface-500/30'}"
									onclick={() => select('background', bg.index)}
								>
									<RawSvg inner={bgThumb(bg.index)} />
								</button>
							{/each}
						</div>
					</div>
					<SwatchGrid
						label="Emblem primary"
						colors={H2_COLORS}
						selected={emblem.emblemPrimary}
						onpick={(i) => select('emblemPrimary', i)}
					/>
					<SwatchGrid
						label="Emblem secondary"
						colors={H2_COLORS}
						selected={emblem.emblemSecondary}
						onpick={(i) => select('emblemSecondary', i)}
					/>
					<p class="text-[10.5px] leading-relaxed opacity-50">
						The real Halo 2 emblem set, extracted from your own copy (personal / LAN use). Emblem
						colors tint the foreground only — the background always wears your armor colors.
					</p>
				{/if}
			</Card>

			<!-- Controls (disabled preview until the bytes are mapped) -->
			<Card class="flex flex-col gap-3.5">
				<span class="inline-flex items-center gap-2">
					<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
						Controls
					</span>
					<WipChip />
				</span>
				<div class="flex flex-col gap-2">
					{#each H2_CONTROL_PREVIEW as r (r.label)}
						<CyclerRow
							label={r.label}
							options={r.options}
							value={r.value}
							defaultIndex={r.value}
							disabled
						/>
					{/each}
				</div>
				<p class="text-[10.5px] leading-relaxed opacity-50">
					The in-game options, in menu order — these will be written into the same signed Halo 2
					profile as your appearance once their byte offsets are live-verified. Until then the save
					keeps the template's control settings.
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
