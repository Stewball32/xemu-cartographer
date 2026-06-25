<script lang="ts">
	import { groupAppearanceFields } from '$lib/utils/gamertag';
	import type { H2AppearanceField } from '$lib/types/lansaves';

	let {
		fields,
		appearance = $bindable()
	}: {
		fields: H2AppearanceField[];
		// key -> byte (0..255); undefined = keep the template's value.
		appearance: Record<string, number | undefined>;
	} = $props();

	const groups = $derived(groupAppearanceFields(fields));
</script>

<div class="space-y-4">
	<p class="text-xs text-surface-600-400">
		Each setting is a single byte (0–255). Blank keeps the template's value (a clean, renamed
		clone). Labels are <strong>provisional</strong> — derived from 2 reverse-engineered sample profiles.
	</p>
	{#each groups as group (group.label)}
		<fieldset class="space-y-2">
			<legend class="text-xs font-semibold tracking-wide uppercase opacity-70">{group.label}</legend
			>
			<div class="grid gap-3 sm:grid-cols-2">
				{#each group.fields as f (f.key)}
					<label class="label">
						<span class="text-xs">
							{f.label}
							<span class="font-mono text-surface-500">@0x{f.offset.toString(16)}</span>
						</span>
						<input
							type="number"
							class="input"
							min="0"
							max="255"
							bind:value={appearance[f.key]}
							placeholder="—"
						/>
					</label>
				{/each}
			</div>
		</fieldset>
	{/each}
</div>
