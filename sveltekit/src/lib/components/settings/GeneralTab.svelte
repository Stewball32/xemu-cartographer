<script lang="ts">
	// Settings → General: profile (avatar / display name / email / location /
	// bio / immutable username), security (password change), danger strip.
	// The save logic is the pre-redesign page's, verbatim behind the new layout:
	// email changes go through PB's requestEmailChange (opt-in button, never on
	// Save), delete = tombstone (is_deleted) + logout.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { LoaderIcon, LockIcon, Trash2Icon, UploadIcon } from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, toastPromise } from '$lib/stores/toaster';
	import Card from '$lib/components/ui/Card.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import EmblemPreview from '$lib/components/gamertag/EmblemPreview.svelte';
	import ActionBar from '$lib/components/settings/ActionBar.svelte';
	import { colorHex, H2_COLORS, type Appearance } from '$lib/utils/emblem';

	let {
		active,
		h2Appearance = {}
	}: {
		active: boolean;
		/** The H2 appearance map, for the avatar ring (armor primary) + the
		 * emblem fallback when no avatar is uploaded. */
		h2Appearance?: Appearance;
	} = $props();

	const user = $derived(auth.user);

	let displayName = $state(auth.user?.name ?? '');
	let email = $state(auth.user?.email ?? '');
	let bio = $state(auth.user?.bio ?? '');
	let location = $state(auth.user?.location ?? '');
	let pendingAvatarFile = $state<File | null>(null);
	let pendingPreviewUrl = $state<string | null>(null);
	let avatarInput = $state<HTMLInputElement | null>(null);
	let saving = $state(false);
	let deleting = $state(false);
	let changingEmail = $state(false);
	let sendingVerification = $state(false);

	let oldPassword = $state('');
	let newPassword = $state('');
	let newPasswordConfirm = $state('');
	let changingPassword = $state(false);
	const passwordMismatch = $derived(
		newPasswordConfirm !== '' && newPassword !== newPasswordConfirm
	);

	const armorHex = $derived(colorHex(H2_COLORS, Number(h2Appearance?.armor_primary ?? 11)));
	const dirty = $derived(
		displayName !== (user?.name ?? '') ||
			bio !== (user?.bio ?? '') ||
			location !== (user?.location ?? '') ||
			pendingAvatarFile !== null
	);
	const emailDirty = $derived(email !== (user?.email ?? ''));

	function acceptAvatar(f: File | undefined | null) {
		if (!f) return;
		pendingAvatarFile = f;
		if (pendingPreviewUrl) URL.revokeObjectURL(pendingPreviewUrl);
		pendingPreviewUrl = URL.createObjectURL(f);
	}

	function reset() {
		displayName = user?.name ?? '';
		email = user?.email ?? '';
		bio = user?.bio ?? '';
		location = user?.location ?? '';
		pendingAvatarFile = null;
		if (pendingPreviewUrl) {
			URL.revokeObjectURL(pendingPreviewUrl);
			pendingPreviewUrl = null;
		}
	}

	async function save() {
		if (!user) return;
		saving = true;
		try {
			const data: Record<string, unknown> = { name: displayName, bio, location };
			if (pendingAvatarFile) data.avatar = pendingAvatarFile;
			await toastPromise(pb.collection('users').update(user.id, data), {
				loading: { title: 'Saving' },
				success: { title: 'Saved', description: 'Your profile has been updated.' },
				errorTitle: 'Save failed'
			});
			pendingAvatarFile = null;
			if (pendingPreviewUrl) {
				URL.revokeObjectURL(pendingPreviewUrl);
				pendingPreviewUrl = null;
			}
		} catch {
			/* toast shown */
		} finally {
			saving = false;
		}
	}

	async function requestEmailChangeFlow() {
		if (!user || !emailDirty) return;
		changingEmail = true;
		try {
			await toastPromise(pb.collection('users').requestEmailChange(email), {
				loading: { title: 'Sending confirmation', description: email },
				success: {
					title: 'Check your inbox',
					description: `We sent a confirmation link to ${email}. Until you click it, your account email stays ${user.email}.`
				},
				errorTitle: 'Email change failed'
			});
		} catch {
			/* toast shown */
		} finally {
			changingEmail = false;
		}
	}

	async function resendVerification() {
		if (!user) return;
		sendingVerification = true;
		try {
			await toastPromise(auth.requestVerification(user.email), {
				loading: { title: 'Sending' },
				success: { title: 'Sent', description: 'Verification email sent. Check your inbox.' },
				errorTitle: 'Send failed'
			});
		} catch {
			/* toast shown */
		} finally {
			sendingVerification = false;
		}
	}

	async function changePassword() {
		if (!user || newPassword !== newPasswordConfirm) return;
		changingPassword = true;
		try {
			await toastPromise(
				pb.collection('users').update(user.id, {
					oldPassword,
					password: newPassword,
					passwordConfirm: newPasswordConfirm
				}),
				{
					loading: { title: 'Updating password' },
					success: { title: 'Updated', description: 'Your password has been changed.' },
					errorTitle: 'Password change failed'
				}
			);
			oldPassword = '';
			newPassword = '';
			newPasswordConfirm = '';
		} catch {
			/* toast shown */
		} finally {
			changingPassword = false;
		}
	}

	async function deleteAccount() {
		if (!user) return;
		const ok = await confirmToast({
			title: 'Delete account',
			description:
				'Tombstone the account? Games stay on the record as [deleted user]; your email, display name, bio, and avatar are blanked and you cannot log back in. No undo.',
			confirmLabel: 'Delete account',
			type: 'warning'
		});
		if (!ok) return;
		deleting = true;
		try {
			await toastPromise(pb.collection('users').update(user.id, { is_deleted: true }), {
				loading: { title: 'Deleting account' },
				success: { title: 'Deleted', description: 'Your account has been tombstoned.' },
				errorTitle: 'Delete failed'
			});
			auth.logout();
			goto(resolve('/login/'));
		} catch {
			deleting = false;
		}
	}
</script>

{#if active && user}
	<div class="flex flex-col gap-5">
		<div class="grid grid-cols-[repeat(auto-fit,minmax(340px,1fr))] items-start gap-5">
			<!-- Profile -->
			<Card class="flex flex-col gap-4">
				<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
					Profile
				</span>
				<div class="flex flex-wrap items-center gap-3.5">
					<div
						class="relative size-13 flex-none overflow-hidden rounded-full"
						style="box-shadow: 0 0 0 2px {armorHex}"
					>
						{#if pendingPreviewUrl}
							<img src={pendingPreviewUrl} alt="" class="size-full object-cover" />
						{:else if user.avatar}
							<UserAvatar {user} size="size-13" />
						{:else}
							<EmblemPreview appearance={h2Appearance} size={52} />
						{/if}
					</div>
					<button class="btn preset-tonal btn-sm" onclick={() => avatarInput?.click()}>
						<UploadIcon class="size-4" /><span>Upload avatar</span>
					</button>
					<span class="text-[11px] opacity-60"> PNG or JPG up to 2 MB — or rep your emblem. </span>
					<input
						bind:this={avatarInput}
						type="file"
						accept="image/*"
						class="hidden"
						onchange={(e) => acceptAvatar((e.currentTarget as HTMLInputElement).files?.[0])}
					/>
				</div>

				<label class="label">
					<span class="label-text">Display name</span>
					<input class="input font-semibold" maxlength={32} bind:value={displayName} />
					<span class="text-[11px] opacity-60">
						Your name around the site. It's not a gamertag — longer is fine.
					</span>
				</label>

				<label class="label">
					<span class="flex items-center justify-between gap-2">
						<span class="label-text">Email</span>
						{#if user.verified}
							<span class="badge preset-tonal-success text-[10px]">Verified</span>
						{:else}
							<button
								class="badge preset-tonal-warning text-[10px]"
								onclick={resendVerification}
								disabled={sendingVerification}
								title="Resend the verification email"
							>
								{sendingVerification ? 'Sending…' : 'Not verified — resend'}
							</button>
						{/if}
					</span>
					<div class="flex gap-2">
						<input type="email" class="input min-w-0 flex-1" bind:value={email} />
						<button
							class="btn flex-none preset-tonal btn-sm"
							onclick={requestEmailChangeFlow}
							disabled={changingEmail || !emailDirty || !email.trim()}
							title="Send a confirmation link to the new address"
						>
							{#if changingEmail}<LoaderIcon class="size-4 animate-spin" />{/if}
							<span>Change email</span>
						</button>
					</div>
					<span class="text-[11px] opacity-60">
						Changing it sends a confirmation link first — nothing moves until you click it.
					</span>
				</label>

				<div class="grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-3.5">
					<label class="label">
						<span class="label-text">Location</span>
						<input
							class="input"
							maxlength={100}
							placeholder="City, Country"
							bind:value={location}
						/>
					</label>
					<label class="label">
						<span class="label-text">Bio</span>
						<input
							class="input"
							maxlength={500}
							placeholder="Tell the lobby who you are…"
							bind:value={bio}
						/>
					</label>
				</div>

				<div class="flex flex-wrap items-center gap-2.5 border-t border-surface-200-800 pt-3.5">
					<LockIcon class="size-3.5 opacity-50" />
					<span class="font-mono text-[13px] opacity-70">{user.username}</span>
					<span class="text-[11px] opacity-60">
						Usernames never change — that's your login, not your name.
					</span>
				</div>
			</Card>

			<!-- Security -->
			<Card class="flex flex-col gap-3">
				<span class="text-[10px] font-bold tracking-[0.2em] text-surface-500 uppercase">
					Security
				</span>
				<label class="label">
					<span class="label-text">Current password</span>
					<input type="password" class="input" placeholder="••••••••" bind:value={oldPassword} />
				</label>
				<div class="grid grid-cols-[repeat(auto-fit,minmax(160px,1fr))] gap-3">
					<label class="label">
						<span class="label-text">New password</span>
						<input
							type="password"
							class="input"
							placeholder="••••••••"
							minlength={8}
							bind:value={newPassword}
						/>
					</label>
					<label class="label">
						<span class="label-text">Confirm</span>
						<input
							type="password"
							class="input"
							placeholder="••••••••"
							minlength={8}
							bind:value={newPasswordConfirm}
						/>
					</label>
				</div>
				{#if passwordMismatch}
					<p class="text-sm text-error-500">Passwords do not match.</p>
				{/if}
				<div>
					<button
						class="btn preset-tonal btn-sm"
						onclick={changePassword}
						disabled={changingPassword ||
							passwordMismatch ||
							!oldPassword ||
							!newPassword ||
							!newPasswordConfirm}
					>
						{#if changingPassword}<LoaderIcon class="size-4 animate-spin" />{/if}
						<span>Update password</span>
					</button>
				</div>
			</Card>
		</div>

		<!-- Danger zone -->
		<div
			class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-error-500/30 bg-error-500/5 px-4.5 py-3"
		>
			<span class="text-xs opacity-80">
				<b class="text-error-500">Danger zone</b> — tombstone the account; games stay on the record
				as <span class="font-mono">[deleted user]</span>. No undo.
			</span>
			<button class="btn preset-tonal-error btn-sm" onclick={deleteAccount} disabled={deleting}>
				{#if deleting}<LoaderIcon class="size-4 animate-spin" />{:else}<Trash2Icon
						class="size-4"
					/>{/if}
				<span>Delete account</span>
			</button>
		</div>

		<ActionBar note="Game saves follow your default gamertag under Stream.">
			<button class="btn preset-tonal" onclick={reset} disabled={!dirty || saving}>Reset</button>
			<button class="btn preset-filled" onclick={save} disabled={!dirty || saving}>
				{#if saving}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Save changes</span>
			</button>
		</ActionBar>
	</div>
{/if}
