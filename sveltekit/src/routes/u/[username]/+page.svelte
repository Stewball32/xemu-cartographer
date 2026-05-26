<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import pb from '$lib/pocketbase';
	import PagePlaceholder from '$lib/components/ui/PagePlaceholder.svelte';

	// Placeholder until M23 lights up invitations + the public profile.
	// The lookup runs early to catch the soft-delete case (7d) so a
	// tombstoned account renders "[deleted user]" instead of leaking the
	// placeholder copy + a username that no longer maps to a real account.

	const username = $derived(page.params.username);

	let isDeleted = $state<boolean | null>(null);
	let lookupFailed = $state(false);

	onMount(async () => {
		try {
			const user = await pb
				.collection('users')
				.getFirstListItem<{ is_deleted?: boolean }>(`username = "${username}"`, {
					fields: 'id,is_deleted'
				});
			isDeleted = user.is_deleted === true;
		} catch {
			// 404 / non-existent username — leave isDeleted=null and show the
			// placeholder; we intentionally don't disambiguate "never existed"
			// from "deleted" in the public view.
			lookupFailed = true;
		}
	});
</script>

{#if isDeleted}
	<div class="mx-auto flex max-w-3xl flex-col gap-4">
		<a href={resolve('/')} class="inline-flex items-center gap-1 anchor text-sm">← Back home</a>
		<h1 class="h2">[deleted user]</h1>
		<div class="card preset-tonal-surface p-4 text-sm opacity-80">
			This account was deleted. Past gamertag, roster, and team membership records remain attached
			to preserve game history, but the person's profile is gone.
		</div>
	</div>
{:else if lookupFailed || isDeleted === false}
	<PagePlaceholder
		title={`@${username}`}
		milestone="M23"
		description="Public profile: display name, avatar, bio, approved gamertags, and current + past team memberships. Owners/managers see an Invite-to-team CTA. Lands with M23 (team membership workflows)."
		backHref={resolve('/')}
		backLabel="Back home"
	/>
{/if}
