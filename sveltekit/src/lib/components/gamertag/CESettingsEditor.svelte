<script lang="ts">
	import { ConstructionIcon } from '@lucide/svelte';

	// Halo: CE has no standalone multiplayer player-profile save file, so the CE
	// customization field set (beyond the name) is still being researched. This
	// editor is a SCAFFOLD: it carries the gamertag now and is structured so the
	// real field set drops straight in. `settings` is the pluggable bag that
	// will hold the CE appearance/controls once mapped.
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
			<p class="font-medium">CE customization is pending research</p>
			<p class="mt-1 opacity-90">
				Halo: CE has no standalone multiplayer player-profile save file (the editable CE surface is
				the gametype). The exact CE player-name / appearance / controls source is being determined
				by a separate research task. For now this profile records your gamertag; the appearance
				&amp; controls fields plug in here once mapped, and the save-file generation follows.
			</p>
		</div>
	</div>
	<p class="text-xs text-surface-600-400">
		The gamertag above is the in-game name this profile will carry.
	</p>
</div>
