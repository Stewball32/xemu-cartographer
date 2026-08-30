<script lang="ts">
	// Independent toggle chips (never either/or) — the organizer pages' shared
	// filter treatment. Each chip flips its own key in the bound `active` map;
	// all-on is the default the pages seed.
	export interface Chip {
		key: string;
		label: string;
	}

	let {
		chips,
		active = $bindable()
	}: {
		chips: Chip[];
		active: Record<string, boolean>;
	} = $props();
</script>

{#each chips as c (c.key)}
	<button
		type="button"
		class="rounded-md border px-2 py-1 text-[10px] font-bold tracking-widest uppercase transition-colors
			{active[c.key]
			? 'border-primary-500/45 bg-primary-500/15 text-primary-600-400'
			: 'border-surface-500/30 bg-transparent text-surface-600-400 hover:text-surface-950-50'}"
		aria-pressed={active[c.key]}
		onclick={() => (active = { ...active, [c.key]: !active[c.key] })}
	>
		{c.label}
	</button>
{/each}
