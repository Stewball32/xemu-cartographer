<script lang="ts">
	import SavePreview from './SavePreview.svelte';
	import type { BuildRequest } from '$lib/types/lansaves';

	let { engines }: { engines: string[] } = $props();

	// Friendly, well-understood fields (the de-risk's high-confidence map).
	let engine = $state('slayer');
	let name = $state('TS 25');
	let scoreLimit = $state<number | undefined>(25);
	let timeMinutes = $state<number | undefined>(7);
	let teamGame = $state(true);
	let radar = $state(false);

	// Advanced raw fields — left blank means "keep the template's value".
	let optionsHex = $state('');
	let scoringSubtype = $state<number | undefined>(undefined);
	let timeLimitRaw = $state<number | undefined>(undefined);
	let timeLimit2 = $state<number | undefined>(undefined);
	let option2 = $state<number | undefined>(undefined);
	let respawn = $state<number | undefined>(undefined);
	let engineUnion = $state<number | undefined>(undefined);

	function num(v: number | undefined): number | undefined {
		return v === undefined || v === null || Number.isNaN(v) ? undefined : v;
	}

	const req = $derived.by((): BuildRequest => {
		const r: BuildRequest = { title: 'ce', kind: 'gametype', name, engine };
		r.teams = teamGame;
		if (radar) r.radar = true;
		r.score_limit = num(scoreLimit);
		// raw time wins over friendly minutes (matches the Go resolution order)
		if (num(timeLimitRaw) !== undefined) r.time_limit = num(timeLimitRaw);
		else if (num(timeMinutes) !== undefined) r.time_minutes = num(timeMinutes);
		if (optionsHex.trim()) {
			const parsed = Number.parseInt(
				optionsHex.trim(),
				optionsHex.trim().startsWith('0x') ? 16 : 10
			);
			if (Number.isFinite(parsed)) r.options = parsed;
		}
		r.scoring_subtype = num(scoringSubtype);
		r.time_limit2 = num(timeLimit2);
		r.option2 = num(option2);
		r.respawn = num(respawn);
		r.engine_union = num(engineUnion);
		return r;
	});
</script>

<div class="grid gap-6 lg:grid-cols-2">
	<!-- Form -->
	<div class="flex flex-col gap-4">
		<div class="space-y-4 card p-6">
			<h3 class="h4">Halo: CE gametype</h3>
			<p class="text-sm text-surface-600-400">
				Generates a <code class="font-mono">blam.lst</code> variant + its
				<code class="font-mono">SaveMeta.xbx</code>. Switching the engine swaps the underlying real
				template (one clean sample per engine).
			</p>

			<div class="grid gap-4 sm:grid-cols-2">
				<label class="label">
					<span>Engine</span>
					<select class="select" bind:value={engine}>
						{#each engines as e (e)}
							<option value={e}>{e}</option>
						{/each}
					</select>
				</label>
				<label class="label">
					<span>Name <span class="text-surface-500">(≤ 11 chars)</span></span>
					<input type="text" class="input" maxlength="11" bind:value={name} placeholder="TS 25" />
				</label>
			</div>

			<div class="grid gap-4 sm:grid-cols-2">
				<label class="label">
					<span>Score limit</span>
					<input type="number" class="input" min="0" bind:value={scoreLimit} />
				</label>
				<label class="label">
					<span>Time limit <span class="text-surface-500">(minutes)</span></span>
					<input type="number" class="input" min="0" step="0.5" bind:value={timeMinutes} />
				</label>
			</div>

			<div class="flex flex-wrap gap-6">
				<label class="flex items-center gap-2">
					<input type="checkbox" class="checkbox" bind:checked={teamGame} />
					<span class="text-sm">Team game</span>
				</label>
				<label class="flex items-center gap-2">
					<input type="checkbox" class="checkbox" bind:checked={radar} />
					<span class="text-sm">Radar / "R" variant</span>
				</label>
			</div>
		</div>

		<details class="card p-6">
			<summary class="cursor-pointer text-sm font-medium">Advanced — raw mapped fields</summary>
			<p class="mt-2 mb-3 text-xs text-surface-600-400">
				Leave blank to keep the template's value. Raw time limit overrides the minutes field.
			</p>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="label">
					<span class="text-xs">options (bitfield, hex ok)</span>
					<input type="text" class="input" placeholder="e.g. 0x22" bind:value={optionsHex} />
				</label>
				<label class="label">
					<span class="text-xs">scoring_subtype</span>
					<input type="number" class="input" bind:value={scoringSubtype} />
				</label>
				<label class="label">
					<span class="text-xs">time_limit (raw, ×2s)</span>
					<input type="number" class="input" bind:value={timeLimitRaw} />
				</label>
				<label class="label">
					<span class="text-xs">time_limit2 (raw)</span>
					<input type="number" class="input" bind:value={timeLimit2} />
				</label>
				<label class="label">
					<span class="text-xs">option2</span>
					<input type="number" class="input" bind:value={option2} />
				</label>
				<label class="label">
					<span class="text-xs">respawn (2=normal, 0=practice)</span>
					<input type="number" class="input" bind:value={respawn} />
				</label>
				<label class="label">
					<span class="text-xs">engine_union (hex u32)</span>
					<input type="number" class="input" bind:value={engineUnion} />
				</label>
			</div>
		</details>
	</div>

	<!-- Preview -->
	<SavePreview {req} />
</div>
