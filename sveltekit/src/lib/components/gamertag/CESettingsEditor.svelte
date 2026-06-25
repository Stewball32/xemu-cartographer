<script lang="ts">
	import { InfoIcon } from '@lucide/svelte';

	// Halo: CE is NAME-ONLY on the original Xbox. There is no MP player profile /
	// appearance / controls — the MP name is the Xbox console name
	// (E:\UDATA\NICKNAME.XBN). So this is an info card, not an editor: the
	// gamertag above is the whole of the CE "profile", and saving generates
	// NICKNAME.XBN. (The "name + armor + controls" profile people remember is
	// Halo PC, not Xbox CE.)
</script>

<div class="flex items-start gap-2 card preset-tonal p-3 text-sm">
	<InfoIcon class="mt-0.5 size-4 shrink-0" />
	<div class="space-y-1">
		<p class="font-medium">Name only</p>
		<p class="text-xs opacity-90">
			On the original Xbox, Halo: CE has no player-profile appearance or controls — your gamertag
			becomes the <strong>Xbox console name</strong>
			(<code class="font-mono">E:\UDATA\NICKNAME.XBN</code>), which is also your in-game multiplayer
			name and what shows on the dashboard / system link. Saving generates it from the gamertag
			above.
		</p>
		<p class="text-xs opacity-60">
			(Appearance, armor, and look/sensitivity are Halo PC features, or local per-controller — not
			part of Xbox CE's networked identity.)
		</p>
	</div>
</div>
