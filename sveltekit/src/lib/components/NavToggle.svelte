<script lang="ts">
	import { resolve } from '$app/paths';
	import { APP_NAME } from '$lib/config/app';
	import logoUrl from '$lib/assets/logo.svg';
	import { MenuIcon } from '@lucide/svelte';

	let { onclick }: { onclick: () => void } = $props();
</script>

<div class="flex flex-row items-center gap-3">
	<button class="btn-icon hover:preset-tonal" aria-label="Toggle navigation" {onclick}>
		<MenuIcon class="size-10" />
	</button>
	<a href={resolve('/')} class="flex items-center gap-2" aria-label="{APP_NAME} home">
		<img src={logoUrl} alt="" class="size-8" />
		<span class="text-xl font-bold">{APP_NAME}</span>
	</a>
</div>
