<script lang="ts">
	// Organizer gametype editor form (used inside the dialog on
	// /organizer/gametypes/). Mirrors the in-game variant settings: CE exposes
	// engine / teams / radar / score / time; H2 maps name + score limit (the
	// fields reverse-engineered so far). The page owns the bindable `form`.

	export interface GametypeFormState {
		id: string; // '' for a new gametype
		title: 'ce' | 'h2';
		engine: string;
		name: string;
		teams: boolean;
		radar: boolean;
		scoreLimit?: number;
		timeMinutes?: number;
	}

	let {
		form = $bindable(),
		ceEngines,
		disabled = false
	}: {
		form: GametypeFormState;
		ceEngines: string[];
		disabled?: boolean;
	} = $props();

	// H2 only has the `slayer` mode mapped; force it when title flips to h2.
	function onTitleChange() {
		if (form.title === 'h2') form.engine = 'slayer';
		else if (!ceEngines.includes(form.engine)) form.engine = ceEngines[0] ?? 'slayer';
	}
</script>

<div class="flex flex-col gap-3">
	<div class="grid grid-cols-2 gap-3">
		<label class="label">
			<span class="label-text">Title</span>
			<select class="select" bind:value={form.title} onchange={onTitleChange} {disabled}>
				<option value="ce">Halo: CE</option>
				<option value="h2">Halo 2</option>
			</select>
		</label>
		<label class="label">
			<span class="label-text">{form.title === 'h2' ? 'Mode' : 'Engine'}</span>
			{#if form.title === 'h2'}
				<input class="input" value="slayer" disabled />
			{:else}
				<select class="select" bind:value={form.engine} {disabled}>
					{#each ceEngines as e (e)}
						<option value={e}>{e}</option>
					{/each}
				</select>
			{/if}
		</label>
	</div>

	<label class="label">
		<span class="label-text">Variant name</span>
		<input
			type="text"
			class="input"
			bind:value={form.name}
			maxlength="64"
			placeholder="e.g. Team Slayer 50"
			{disabled}
		/>
	</label>

	<div class="grid grid-cols-2 gap-3">
		<label class="label">
			<span class="label-text">Score limit</span>
			<input
				type="number"
				class="input"
				min="0"
				bind:value={form.scoreLimit}
				placeholder="—"
				{disabled}
			/>
		</label>
		{#if form.title === 'ce'}
			<label class="label">
				<span class="label-text">Time limit (min)</span>
				<input
					type="number"
					class="input"
					min="0"
					step="0.5"
					bind:value={form.timeMinutes}
					placeholder="—"
					{disabled}
				/>
			</label>
		{/if}
	</div>

	{#if form.title === 'ce'}
		<div class="flex flex-wrap gap-4">
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" class="checkbox" bind:checked={form.teams} {disabled} />
				<span>Team game</span>
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" class="checkbox" bind:checked={form.radar} {disabled} />
				<span>Radar ("R")</span>
			</label>
		</div>
	{/if}

	<p class="text-xs text-surface-600-400">
		Blank score/time keeps the template's value. Halo 2 variant settings beyond name + score limit
		are preserved from the template (only those are mapped so far).
	</p>
</div>
