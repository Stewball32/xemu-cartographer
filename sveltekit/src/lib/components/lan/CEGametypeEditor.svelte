<script lang="ts">
	// Lean admin quick-build for a CE gametype (raw-endpoint smoke test). The full
	// engine-aware editor with the complete live-verified setting surface lives at
	// /organizer/creator/ — this is just a fast preview against /api/lan/saves.
	import { resolve } from '$app/paths';
	import SavePreview from './SavePreview.svelte';
	import type { BuildRequest } from '$lib/types/lansaves';

	let { engines }: { engines: string[] } = $props();

	let engine = $state('slayer');
	let name = $state('TS 25');
	let scoreLimit = $state<number | undefined>(25);
	let respawnSeconds = $state<number | undefined>(undefined);
	let lives = $state<number | undefined>(undefined);
	let teamGame = $state(true);
	let radar = $state(false);
	let optionsHex = $state('');

	function num(v: number | undefined): number | undefined {
		return v === undefined || v === null || Number.isNaN(v) ? undefined : v;
	}

	const req = $derived.by((): BuildRequest => {
		const r: BuildRequest = { title: 'ce', kind: 'gametype', name, engine };
		r.teams = teamGame;
		if (radar) r.radar = true;
		r.score_limit = num(scoreLimit);
		r.respawn_seconds = num(respawnSeconds);
		r.lives = num(lives);
		if (optionsHex.trim()) {
			const parsed = Number.parseInt(
				optionsHex.trim(),
				optionsHex.trim().startsWith('0x') ? 16 : 10
			);
			if (Number.isFinite(parsed)) r.options = parsed;
		}
		return r;
	});
</script>

<div class="grid gap-6 lg:grid-cols-2">
	<div class="flex flex-col gap-4">
		<div class="space-y-4 card p-6">
			<h3 class="h4">Halo: CE gametype — quick build</h3>
			<p class="text-sm text-surface-600-400">
				Fast <code class="font-mono">blam.lst</code> preview. For the full editor (all engines +
				every setting), use
				<a class="anchor" href={resolve('/organizer/creator/')}>the gametype creator</a>.
			</p>

			<div class="grid gap-4 sm:grid-cols-2">
				<label class="label">
					<span>Engine</span>
					<select class="select" bind:value={engine}>
						{#each engines as e (e)}<option value={e}>{e}</option>{/each}
					</select>
				</label>
				<label class="label">
					<span>Name <span class="text-surface-500">(≤ 11 chars)</span></span>
					<input type="text" class="input" maxlength="11" bind:value={name} placeholder="TS 25" />
				</label>
				<label class="label">
					<span>Score limit</span>
					<input type="number" class="input" min="0" bind:value={scoreLimit} />
				</label>
				<label class="label">
					<span>Respawn <span class="text-surface-500">(seconds)</span></span>
					<input type="number" class="input" min="0" step="0.5" bind:value={respawnSeconds} />
				</label>
				<label class="label">
					<span>Lives <span class="text-surface-500">(0 = infinite)</span></span>
					<input type="number" class="input" min="0" bind:value={lives} />
				</label>
				<label class="label">
					<span>options <span class="text-surface-500">(hex ok)</span></span>
					<input type="text" class="input" placeholder="e.g. 0x22" bind:value={optionsHex} />
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
	</div>

	<SavePreview {req} />
</div>
