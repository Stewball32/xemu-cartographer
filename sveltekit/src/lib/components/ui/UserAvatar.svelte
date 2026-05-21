<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Avatar } from '@skeletonlabs/skeleton-svelte';
	import { UserIcon } from '@lucide/svelte';
	import { getFileURL } from '$lib/utils/files';

	type AvatarUser = {
		id: string;
		collectionId: string;
		collectionName: string;
		[key: string]: unknown;
	};

	let {
		user = null,
		overrideSrc = null,
		fallback,
		size = 'size-10',
		class: extraClass = '',
		filter = null,
		alt = '',
		thumb = '160x160',
		badge
	}: {
		user?: AvatarUser | null;
		overrideSrc?: string | null;
		fallback?: string;
		size?: string;
		class?: string;
		filter?: string | null;
		alt?: string;
		thumb?: string;
		badge?: Snippet;
	} = $props();

	const resolvedSrc = $derived(
		overrideSrc ?? (user ? getFileURL(user, 'avatar', { thumb }) : null)
	);

	const derivedFallback = $derived.by<string | null>(() => {
		if (fallback !== undefined) return fallback;
		const name = (user?.name as string | undefined)?.trim();
		if (name) {
			return name
				.split(/\s+/)
				.map((part) => part[0])
				.join('')
				.toUpperCase()
				.slice(0, 2);
		}
		const username = (user?.username as string | undefined)?.trim();
		if (username) return username.slice(0, 2).toUpperCase();
		return null;
	});

	const imageClass = $derived(
		filter ? `size-full object-cover filter-[url(#${filter})]` : 'size-full object-cover'
	);
</script>

<Avatar
	class="inline-flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-200-800 font-semibold {badge
		? 'relative'
		: ''} {size} {extraClass}"
>
	{#if resolvedSrc}
		<Avatar.Image src={resolvedSrc} {alt} class={imageClass} />
	{/if}
	<Avatar.Fallback>
		{#if derivedFallback === null}
			<UserIcon class="size-1/2" />
		{:else}
			{derivedFallback}
		{/if}
	</Avatar.Fallback>
	{#if badge}
		<span class="absolute -top-0 -right-0 z-10">{@render badge()}</span>
	{/if}
</Avatar>
