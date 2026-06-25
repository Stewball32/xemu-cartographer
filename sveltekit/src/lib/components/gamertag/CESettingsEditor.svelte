<script lang="ts">
	import { ConstructionIcon } from '@lucide/svelte';

	// Halo: CE DOES have player profiles on Xbox — a full profile parallel to H2.
	// The exact profile location + field set is being re-investigated, so this is
	// a scaffold: `settings` is the pluggable bag the confirmed CE field set will
	// fill, and saving stores the record (generation deferred until the format
	// lands). The gamertag above is the profile's in-game name.
	let {
		settings = $bindable()
	}: {
		settings: Record<string, unknown>;
	} = $props();

	// Touch settings so the binding is considered "used" until the real field
	// set lands (keeps lint happy without an eslint-disable).
	$effect(() => {
		void settings;
	});
</script>

<div class="space-y-3">
	<div class="flex items-start gap-2 card preset-tonal-warning p-3 text-xs">
		<ConstructionIcon class="mt-0.5 size-4 shrink-0" />
		<div>
			<p class="font-medium">Full CE profile — field set coming</p>
			<p class="mt-1 opacity-90">
				Halo: CE has its own player profiles. The exact profile location and field set are being
				re-investigated; once confirmed, the full customization (name + appearance + controls) drops
				in here and saving will generate the signed CE profile. For now your gamertag is recorded
				and the profile is scaffolded.
			</p>
		</div>
	</div>
	<p class="text-xs text-surface-600-400">
		Separately, your gamertag is also available as your Xbox <strong>console name</strong>
		(<code class="font-mono">NICKNAME.XBN</code> — dashboard / system-link identity), which is not the
		CE profile.
	</p>
</div>
