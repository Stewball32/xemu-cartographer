<script lang="ts">
	import SavePreview from './SavePreview.svelte';
	import type { BuildRequest, H2AppearanceField } from '$lib/types/lansaves';

	let { fields }: { fields: H2AppearanceField[] } = $props();

	let name = $state('CARTOG');
	// Appearance/controller bytes — blank = keep the template's value.
	let appearance = $state<Record<string, number | undefined>>({});

	function num(v: number | undefined): number | undefined {
		return v === undefined || v === null || Number.isNaN(v) ? undefined : v;
	}

	const req = $derived.by((): BuildRequest => {
		const ap: Record<string, number> = {};
		for (const f of fields) {
			const v = num(appearance[f.key]);
			if (v !== undefined) ap[f.key] = v;
		}
		const r: BuildRequest = { title: 'h2', kind: 'profile', name };
		if (Object.keys(ap).length) r.appearance = ap;
		return r;
	});
</script>

<div class="grid gap-6 lg:grid-cols-2">
	<div class="flex flex-col gap-4">
		<div class="space-y-4 card p-6">
			<h3 class="h4">Halo 2 player profile</h3>
			<p class="text-sm text-surface-600-400">
				Generates a 500-byte <code class="font-mono">profile</code> + its
				<code class="font-mono">SaveMeta.xbx</code> (display name
				<span class="font-mono">Profile: …</span>).
			</p>
			<label class="label">
				<span>Player name</span>
				<input type="text" class="input" bind:value={name} placeholder="CARTOG" />
			</label>
		</div>

		<div class="space-y-3 card p-6">
			<div>
				<h4 class="text-sm font-medium">Appearance &amp; controls</h4>
				<p class="text-xs text-surface-600-400">
					Each is a single byte (0–255). Labels are <strong>provisional</strong> (derived from 2 sample
					profiles). Blank = keep template value.
				</p>
			</div>
			<div class="grid gap-3 sm:grid-cols-2">
				{#each fields as f (f.key)}
					<label class="label">
						<span class="text-xs">
							{f.label}
							<span class="font-mono text-surface-500">@0x{f.offset.toString(16)}</span>
						</span>
						<input type="number" class="input" min="0" max="255" bind:value={appearance[f.key]} />
					</label>
				{/each}
			</div>
		</div>
	</div>

	<SavePreview {req} />
</div>
