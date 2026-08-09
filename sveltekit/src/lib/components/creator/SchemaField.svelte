<script lang="ts">
	// Renders one CE gametype setting from the backend schema, bound to a value in
	// the editor's form object. The field kind decides the control; enum/number
	// metadata (options, min/max/step, unit) all come from the schema so the UI
	// stays in lockstep with the byte map.
	import type { CEField } from '$lib/types/lansaves';

	let {
		field,
		value = $bindable(),
		disabled = false
	}: {
		field: CEField;
		value: boolean | number | undefined;
		disabled?: boolean;
	} = $props();

	// An empty number box means "keep the template's value" (undefined).
	function commitNumber(raw: string) {
		const t = raw.trim();
		value = t === '' ? undefined : Number(t);
	}
</script>

<label class="flex flex-col gap-1">
	{#if field.kind === 'bool'}
		<span class="flex items-center justify-between gap-3">
			<span class="flex flex-col">
				<span class="text-sm font-medium">{field.label}</span>
				{#if field.help}<span class="text-xs text-surface-500">{field.help}</span>{/if}
			</span>
			<input
				type="checkbox"
				class="checkbox"
				checked={value === true}
				onchange={(e) => (value = e.currentTarget.checked)}
				{disabled}
			/>
		</span>
	{:else if field.kind === 'enum'}
		<span class="text-sm font-medium">{field.label}</span>
		<select
			class="select"
			value={value ?? field.options?.[0]?.value ?? 0}
			onchange={(e) => (value = Number(e.currentTarget.value))}
			{disabled}
		>
			{#each field.options ?? [] as opt (opt.value)}
				<option value={opt.value}>{opt.label}</option>
			{/each}
		</select>
		{#if field.help}<span class="text-xs text-surface-500">{field.help}</span>{/if}
	{:else}
		<span class="flex items-center justify-between gap-2">
			<span class="text-sm font-medium">{field.label}</span>
			{#if field.unit}<span class="text-xs text-surface-500">{field.unit}</span>{/if}
		</span>
		<input
			type="number"
			class="input"
			min={field.min}
			max={field.max}
			step={field.step ?? 1}
			value={value ?? ''}
			oninput={(e) => commitNumber(e.currentTarget.value)}
			placeholder="— keep default"
			{disabled}
		/>
		{#if field.help}<span class="text-xs text-surface-500">{field.help}</span>{/if}
	{/if}
</label>
