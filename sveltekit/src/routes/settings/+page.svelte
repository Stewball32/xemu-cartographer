<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Tabs, FileUpload } from '@skeletonlabs/skeleton-svelte';
	import {
		UserIcon,
		MailIcon,
		Trash2Icon,
		UploadIcon,
		LockIcon,
		ShieldCheckIcon,
		ShieldAlertIcon,
		MapPinIcon,
		LinkIcon,
		UnlinkIcon,
		LoaderIcon
	} from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import pb from '$lib/pocketbase';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { toastPromise } from '$lib/stores/toaster';
	import { OAUTH_PROVIDERS } from '$lib/config/app';

	let activeTab = $state('general');

	// General tab state — load() guarantees auth.user is non-null here
	const _user = auth.user!;
	let displayName = $state(_user?.name ?? '');
	let email = $state(_user?.email ?? '');
	let bio = $state(_user?.bio ?? '');
	let location = $state(_user?.location ?? '');
	let pendingPreviewUrl = $state<string | null>(null);
	let pendingAvatarFile = $state<File | null>(null);
	let saving = $state(false);
	let deleting = $state(false);
	let sendingVerification = $state(false);
	let changingPassword = $state(false);
	let oldPassword = $state('');
	let newPassword = $state('');
	let newPasswordConfirm = $state('');
	const passwordMismatch = $derived(
		newPasswordConfirm !== '' && newPassword !== newPasswordConfirm
	);
	const previewInitials = $derived.by<string | undefined>(() => {
		const name = displayName.trim();
		if (!name) return undefined;
		return name
			.split(/\s+/)
			.map((p) => p[0])
			.join('')
			.toUpperCase()
			.slice(0, 2);
	});

	// Connected Accounts tab state
	let linkedAuths = $state<Array<Record<string, string>>>([]);
	let enabledProviders = $state<string[]>([]);
	let linkingProvider = $state<string | null>(null);
	let unlinkingProvider = $state<string | null>(null);
	const linkedProviderNames = $derived(new Set(linkedAuths.map((a) => a.provider)));

	const visibleProviders = $derived(enabledProviders.filter((name) => name in OAUTH_PROVIDERS));

	onMount(async () => {
		try {
			const methods = await pb.collection('users').listAuthMethods();
			enabledProviders = methods.oauth2?.providers?.map((p) => p.name) ?? [];
		} catch {
			enabledProviders = [];
		}
		await loadLinkedAuths();
	});

	async function loadLinkedAuths() {
		if (!auth.user) return;
		try {
			linkedAuths = await auth.listExternalAuths(auth.user.id);
		} catch {
			linkedAuths = [];
		}
	}

	function loadUserData() {
		const user = auth.user!;
		displayName = user.name ?? '';
		email = user.email ?? '';
		bio = user.bio ?? '';
		location = user.location ?? '';
		pendingPreviewUrl = null;
	}

	function handleAvatarAccept(details: { files: File[] }) {
		const file = details.files[0];
		if (!file) return;
		pendingAvatarFile = file;
		pendingPreviewUrl = URL.createObjectURL(file);
	}

	async function saveGeneral() {
		if (!auth.user) return;
		saving = true;
		try {
			const data: Record<string, unknown> = { name: displayName, email, bio, location };
			if (pendingAvatarFile) data.avatar = pendingAvatarFile;
			await toastPromise(pb.collection('users').update(auth.user.id, data), {
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
			// toast already shown
		} finally {
			saving = false;
		}
	}

	function resetGeneral() {
		pendingAvatarFile = null;
		if (pendingPreviewUrl) {
			URL.revokeObjectURL(pendingPreviewUrl);
		}
		loadUserData();
	}

	async function resendVerification() {
		if (!auth.user) return;
		sendingVerification = true;
		try {
			await toastPromise(auth.requestVerification(auth.user.email), {
				loading: { title: 'Sending' },
				success: { title: 'Sent', description: 'Verification email sent. Check your inbox.' },
				errorTitle: 'Send failed'
			});
		} catch {
			// toast already shown
		} finally {
			sendingVerification = false;
		}
	}

	async function changePassword() {
		if (!auth.user || newPassword !== newPasswordConfirm) return;
		changingPassword = true;
		try {
			await toastPromise(
				pb.collection('users').update(auth.user.id, {
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
			// toast already shown
		} finally {
			changingPassword = false;
		}
	}

	async function deleteAccount() {
		if (!auth.user) return;
		const confirmed = confirm(
			'Permanently delete your account and all associated data? This cannot be undone.'
		);
		if (!confirmed) return;
		deleting = true;
		try {
			await toastPromise(pb.collection('users').delete(auth.user.id), {
				loading: { title: 'Deleting account' },
				success: { title: 'Deleted', description: 'Your account has been removed.' },
				errorTitle: 'Delete failed'
			});
			auth.logout();
			goto(resolve('/login/'));
		} catch {
			// toast already shown
			deleting = false;
		}
	}

	async function linkProvider(provider: string) {
		if (!auth.user) return;
		linkingProvider = provider;
		const label = OAUTH_PROVIDERS[provider]?.label ?? provider;
		try {
			await toastPromise(auth.linkOAuth(provider), {
				loading: { title: 'Connecting', description: label },
				success: { title: 'Connected', description: `${label} account linked.` },
				errorTitle: 'Connect failed',
				errorDescription: (err) => {
					const message = err instanceof Error ? err.message : 'Failed to link account.';
					return message.includes('already') || message.includes('unique')
						? 'This account is already linked to another user.'
						: message;
				}
			});
			await loadLinkedAuths();
		} catch {
			// toast already shown
		} finally {
			linkingProvider = null;
		}
	}

	async function unlinkProvider(provider: string) {
		if (!auth.user) return;
		unlinkingProvider = provider;
		const label = OAUTH_PROVIDERS[provider]?.label ?? provider;
		try {
			await toastPromise(auth.unlinkOAuth(auth.user.id, provider), {
				loading: { title: 'Disconnecting', description: label },
				success: { title: 'Disconnected', description: `${label} account unlinked.` },
				errorTitle: 'Disconnect failed'
			});
			await loadLinkedAuths();
		} catch {
			// toast already shown
		} finally {
			unlinkingProvider = null;
		}
	}
</script>

<h1 class="mb-6 h2">Settings</h1>

<div class="max-w-2xl">
	<Tabs value={activeTab} onValueChange={(e) => (activeTab = e.value)}>
		<Tabs.List class="mb-6">
			<Tabs.Trigger value="general">General</Tabs.Trigger>
			<Tabs.Trigger value="accounts">Connected Accounts</Tabs.Trigger>
			<Tabs.Indicator />
		</Tabs.List>

		<!-- General Tab -->
		<Tabs.Content value="general">
			<div class="space-y-6">
				<!-- Account Info -->
				<div class="space-y-6 card p-6">
					<h2 class="h4">Account</h2>

					<!-- Avatar Upload -->
					<div class="flex items-center gap-6">
						<UserAvatar
							user={_user}
							overrideSrc={pendingPreviewUrl}
							fallback={previewInitials}
							size="size-20"
						/>
						<FileUpload maxFiles={1} accept="image/*" onFileAccept={handleAvatarAccept}>
							<FileUpload.Dropzone class="card preset-outlined-surface-200-800 p-4">
								<div class="flex flex-col items-center gap-2 text-center">
									<UploadIcon class="size-8 text-surface-400-600" />
									<p class="text-sm">
										<span class="font-semibold text-primary-500">Click to upload</span> or drag and drop
									</p>
									<p class="text-xs opacity-50">PNG, JPG up to 2MB</p>
								</div>
							</FileUpload.Dropzone>
							<FileUpload.HiddenInput />
						</FileUpload>
					</div>

					<hr class="hr" />

					<!-- Input Groups -->
					<label class="label">
						<span>Display Name</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><UserIcon class="size-4" /></div>
							<input type="text" class="ig-input" bind:value={displayName} />
						</div>
					</label>

					<label class="label">
						<span>Email</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><MailIcon class="size-4" /></div>
							<input type="email" class="ig-input" bind:value={email} />
						</div>
					</label>

					<label class="label">
						<span>Bio</span>
						<textarea
							class="textarea"
							rows="3"
							maxlength="500"
							bind:value={bio}
							placeholder="Tell us about yourself..."
						></textarea>
					</label>

					<label class="label">
						<span>Location</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><MapPinIcon class="size-4" /></div>
							<input
								type="text"
								class="ig-input"
								bind:value={location}
								maxlength="100"
								placeholder="City, Country"
							/>
						</div>
					</label>

					{#if auth.user?.verified}
						<div class="flex items-center gap-2 text-sm text-success-500">
							<ShieldCheckIcon class="size-4" />
							<span>Email verified</span>
						</div>
					{:else}
						<div
							class="flex items-center justify-between rounded-md border border-warning-500/30 bg-warning-500/10 p-3"
						>
							<div class="flex items-center gap-2 text-sm text-warning-500">
								<ShieldAlertIcon class="size-4" />
								<span>Email not verified</span>
							</div>
							<button
								class="btn preset-tonal btn-sm"
								onclick={resendVerification}
								disabled={sendingVerification}
							>
								{#if sendingVerification}<LoaderIcon class="size-4 animate-spin" />{/if}
								<span>Resend</span>
							</button>
						</div>
					{/if}
				</div>

				<!-- Change Password -->
				<div class="space-y-4 card p-6">
					<h2 class="h4">Change Password</h2>

					<label class="label">
						<span>Current Password</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><LockIcon class="size-4" /></div>
							<input
								type="password"
								class="ig-input"
								placeholder="••••••••"
								bind:value={oldPassword}
								required
							/>
						</div>
					</label>

					<label class="label">
						<span>New Password</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><LockIcon class="size-4" /></div>
							<input
								type="password"
								class="ig-input"
								placeholder="••••••••"
								bind:value={newPassword}
								minlength="8"
								required
							/>
						</div>
					</label>

					<label class="label">
						<span>Confirm New Password</span>
						<div class="input-group grid-cols-[auto_1fr_auto]">
							<div class="ig-cell"><LockIcon class="size-4" /></div>
							<input
								type="password"
								class="ig-input"
								placeholder="••••••••"
								bind:value={newPasswordConfirm}
								minlength="8"
								required
							/>
						</div>
						{#if passwordMismatch}
							<p class="text-sm text-error-500">Passwords do not match.</p>
						{/if}
					</label>

					<button
						class="btn preset-filled"
						onclick={changePassword}
						disabled={changingPassword ||
							passwordMismatch ||
							!oldPassword ||
							!newPassword ||
							!newPasswordConfirm}
					>
						{#if changingPassword}<LoaderIcon class="size-4 animate-spin" />{/if}
						<span>Update Password</span>
					</button>
				</div>

				<!-- Danger Zone -->
				<div class="space-y-3 card preset-outlined-error-500 p-6">
					<h2 class="h4 text-error-500">Danger Zone</h2>
					<p class="text-sm opacity-70">
						Permanently delete your account and all associated data. This action cannot be undone.
					</p>
					<button class="btn preset-filled-error-500" onclick={deleteAccount} disabled={deleting}>
						{#if deleting}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<Trash2Icon class="size-4" />
						{/if}
						<span>Delete Account</span>
					</button>
				</div>

				<!-- Save -->
				<div class="flex gap-3">
					<button class="btn preset-filled" onclick={saveGeneral} disabled={saving}>
						{#if saving}<LoaderIcon class="size-4 animate-spin" />{/if}
						<span>Save Changes</span>
					</button>
					<button class="btn preset-tonal" onclick={resetGeneral} disabled={saving}>Reset</button>
				</div>
			</div>
		</Tabs.Content>

		<!-- Connected Accounts Tab -->
		<Tabs.Content value="accounts">
			<div class="space-y-6">
				<div class="space-y-4 card p-6">
					<h2 class="h4">Connected Accounts</h2>
					<p class="text-sm opacity-70">Link or unlink your external sign-in providers.</p>

					{#if visibleProviders.length === 0}
						<p class="text-sm opacity-50">No OAuth providers are configured.</p>
					{:else}
						<div class="space-y-3">
							{#each visibleProviders as provider (provider)}
								{@const meta = OAUTH_PROVIDERS[provider]}
								{@const isLinked = linkedProviderNames.has(provider)}
								{@const isLinking = linkingProvider === provider}
								{@const isUnlinking = unlinkingProvider === provider}
								<div
									class="flex items-center justify-between rounded-md border border-surface-300-700 p-3"
								>
									<div class="flex items-center gap-3">
										<img src={meta.icon} alt={meta.label} class="size-6" />
										<div>
											<p class="font-semibold">{meta.label}</p>
											{#if isLinked}
												<p class="text-xs text-success-500">Connected</p>
											{:else}
												<p class="text-xs opacity-50">Not connected</p>
											{/if}
										</div>
									</div>
									{#if isLinked}
										<button
											class="btn preset-tonal-error btn-sm"
											onclick={() => unlinkProvider(provider)}
											disabled={isUnlinking}
										>
											{#if isUnlinking}
												<LoaderIcon class="size-4 animate-spin" />
											{:else}
												<UnlinkIcon class="size-4" />
											{/if}
											<span>Disconnect</span>
										</button>
									{:else}
										<button
											class="btn preset-tonal btn-sm"
											onclick={() => linkProvider(provider)}
											disabled={isLinking}
										>
											{#if isLinking}
												<LoaderIcon class="size-4 animate-spin" />
											{:else}
												<LinkIcon class="size-4" />
											{/if}
											<span>Connect</span>
										</button>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		</Tabs.Content>
	</Tabs>
</div>
