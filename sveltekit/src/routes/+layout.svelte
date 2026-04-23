<script lang="ts">
	import { onMount } from 'svelte';
	import { Toast } from '@skeletonlabs/skeleton-svelte';
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import Header from '$lib/components/Header.svelte';
	import NavPanel from '$lib/components/NavPanel.svelte';
	import NavBar from '$lib/components/NavBar.svelte';
	import { toaster } from '$lib/stores/toaster';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { isLayoutHidden } from '$lib/config/layout';
	import { buildLoginUrl } from '$lib/utils/redirect';
	import { APP_NAME } from '$lib/config/app';

	let { children } = $props();

	const hideLayout = $derived(isLayoutHidden(page.url.pathname));

	$effect(() => {
		if (page.data.requiresAuth && !auth.isLoggedIn) {
			// buildLoginUrl returns a runtime-validated path — not a static route literal
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			goto(buildLoginUrl(page.url.pathname + page.url.search));
		}
	});
	let navOpen = $state(false);
	let isTablet = $state(false);
	let isDesktop = $state(false);

	onMount(() => {
		const mqlSm = window.matchMedia('(min-width: 640px)');
		const mqlLg = window.matchMedia('(min-width: 1024px)');

		isDesktop = mqlLg.matches;
		isTablet = mqlSm.matches && !mqlLg.matches;
		if (mqlLg.matches) navOpen = true;

		const handlerSm = (e: MediaQueryListEvent) => {
			isTablet = e.matches && !mqlLg.matches;
			if (!e.matches) navOpen = false;
		};
		const handlerLg = (e: MediaQueryListEvent) => {
			isDesktop = e.matches;
			isTablet = mqlSm.matches && !e.matches;
			navOpen = e.matches;
		};

		mqlSm.addEventListener('change', handlerSm);
		mqlLg.addEventListener('change', handlerLg);
		return () => {
			mqlSm.removeEventListener('change', handlerSm);
			mqlLg.removeEventListener('change', handlerLg);
		};
	});

	function handleToggle() {
		navOpen = !navOpen;
	}
</script>

<svelte:head>
	<title>{APP_NAME}</title>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if hideLayout}
	<main class="min-h-screen">
		{@render children()}
	</main>
{:else}
	<div class="flex h-screen flex-col">
		<Header onToggle={handleToggle} />
		<div class="relative flex flex-1 overflow-hidden">
			<div class="hidden w-25 shrink-0 sm:flex lg:hidden" aria-hidden="true"></div>
			<NavPanel bind:open={navOpen} {isDesktop} {isTablet} currentPath={page.url.pathname} />
			<main class="flex-1 overflow-y-auto p-4 pb-20 sm:pb-4 lg:p-8 lg:pb-8">
				{@render children()}
			</main>
		</div>
		<NavBar currentPath={page.url.pathname} />
	</div>
{/if}

<Toast.Group {toaster}>
	{#snippet children(toast)}
		<Toast {toast}>
			<Toast.Message>
				<Toast.Title>{toast.title}</Toast.Title>
				<Toast.Description>{toast.description}</Toast.Description>
			</Toast.Message>
			<Toast.CloseTrigger />
		</Toast>
	{/snippet}
</Toast.Group>
