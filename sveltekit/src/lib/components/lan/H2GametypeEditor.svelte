<script lang="ts">
	import SavePreview from './SavePreview.svelte';
	import type { BuildRequest } from '$lib/types/lansaves';

	let { mode }: { mode: string } = $props();

	let name = $state('team brs');
	let scoreLimit = $state<number | undefined>(50);

	function num(v: number | undefined): number | undefined {
		return v === undefined || v === null || Number.isNaN(v) ? undefined : v;
	}

	const req = $derived.by((): BuildRequest => {
		const r: BuildRequest = { title: 'h2', kind: 'gametype', name };
		r.score_limit = num(scoreLimit);
		return r;
	});
</script>

<div class="grid gap-6 lg:grid-cols-2">
	<div class="flex flex-col gap-4">
		<div class="space-y-4 card p-6">
			<h3 class="h4">Halo 2 gametype</h3>
			<p class="text-sm text-surface-600-400">
				Generates a <code class="font-mono">{mode}</code> gametype payload + its
				<code class="font-mono">SaveMeta.xbx</code>. The field map is <strong>partial</strong> — name
				and score limit are mapped from a single sample; other settings are preserved from the template.
			</p>

			<label class="label">
				<span>Mode</span>
				<input type="text" class="input" value={mode} disabled />
			</label>
			<div class="grid gap-4 sm:grid-cols-2">
				<label class="label">
					<span>Name</span>
					<input type="text" class="input" bind:value={name} placeholder="team brs" />
				</label>
				<label class="label">
					<span>Score limit</span>
					<input type="number" class="input" min="0" bind:value={scoreLimit} />
				</label>
			</div>
		</div>
	</div>

	<SavePreview {req} />
</div>
