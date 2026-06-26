<script lang="ts">
	import type { CeProfileSettings } from '$lib/types/gamertag';

	// Halo: CE player profile (blam.sav) — name (= the gamertag above) + armor
	// color + control presets, signed. The advanced Setup bytes (sensitivity /
	// invert / vibration) aren't individually mapped yet; this exposes the mapped
	// surface.
	let {
		settings = $bindable()
	}: {
		settings: CeProfileSettings;
	} = $props();

	// Known color enum values (the full carousel isn't enumerated yet).
	const COLORS = [
		{ value: 0, label: 'White (0)' },
		{ value: 2, label: 'Red (2)' },
		{ value: 3, label: 'Blue (3)' }
	];
	const PRESETS = [
		{ value: 0, label: 'Default' },
		{ value: 1, label: 'Southpaw' }
	];

	// Normalize undefined → defaults for the bound controls.
	const color = $derived(settings.color ?? 0);
	const thumbstick = $derived(settings.thumbstick ?? 0);
	const button = $derived(settings.button ?? 0);
</script>

<div class="space-y-4">
	<p class="text-xs text-surface-600-400">
		Your gamertag is the in-game name. Armor color is a single enum (CE has no two-tone); controls
		are the in-game presets.
	</p>

	<label class="label">
		<span class="text-xs">Armor color</span>
		<div class="grid grid-cols-2 gap-2">
			<select
				class="select"
				value={COLORS.some((c) => c.value === color) ? String(color) : 'custom'}
				onchange={(e) => {
					const v = (e.currentTarget as HTMLSelectElement).value;
					if (v !== 'custom') settings.color = Number(v);
				}}
			>
				{#each COLORS as c (c.value)}
					<option value={String(c.value)}>{c.label}</option>
				{/each}
				<option value="custom">Custom…</option>
			</select>
			<input
				type="number"
				class="input"
				min="0"
				max="255"
				value={color}
				oninput={(e) => (settings.color = Number((e.currentTarget as HTMLInputElement).value))}
				title="Color enum value"
			/>
		</div>
	</label>

	<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
		<label class="label">
			<span class="text-xs">Thumbstick preset</span>
			<select
				class="select"
				value={String(thumbstick)}
				onchange={(e) =>
					(settings.thumbstick = Number((e.currentTarget as HTMLSelectElement).value))}
			>
				{#each PRESETS as p (p.value)}
					<option value={String(p.value)}>{p.label}</option>
				{/each}
			</select>
		</label>
		<label class="label">
			<span class="text-xs">Button preset</span>
			<select
				class="select"
				value={String(button)}
				onchange={(e) => (settings.button = Number((e.currentTarget as HTMLSelectElement).value))}
			>
				{#each PRESETS as p (p.value)}
					<option value={String(p.value)}>{p.label}</option>
				{/each}
			</select>
		</label>
	</div>

	<p class="text-xs text-surface-500">
		Advanced Setup (look sensitivity / invert / vibration) is a follow-up — one more capture maps
		those bytes.
	</p>
</div>
