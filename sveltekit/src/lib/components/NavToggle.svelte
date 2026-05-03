<script lang="ts">
	import { resolve } from '$app/paths';
	import { APP_NAME } from '$lib/config/app';
	import logoUrl from '$lib/assets/logo.svg';
	import { MenuIcon } from '@lucide/svelte';

	let { onclick }: { onclick: () => void } = $props();
</script>

<div class="flex min-w-0 flex-row items-center gap-3">
	<button class="btn-icon shrink-0 hover:preset-tonal" aria-label="Toggle navigation" {onclick}>
		<MenuIcon class="size-10" />
	</button>
	<a href={resolve('/')} class="flex min-w-0 items-center gap-2" aria-label="{APP_NAME} home">
		<img src={logoUrl} alt="" class="size-8 shrink-0" />
		<span class="hidden truncate text-xl font-bold sm:inline">{APP_NAME}</span>
	</a>
</div>
