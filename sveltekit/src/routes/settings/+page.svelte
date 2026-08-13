<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { FileUpload } from '@skeletonlabs/skeleton-svelte';
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
		LoaderIcon,
		TagIcon,
		PlusIcon,
		StarIcon,
		UsersIcon,
		CrownIcon,
		BriefcaseIcon
	} from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import pb from '$lib/pocketbase';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import PendingRequests from '$lib/components/membership/PendingRequests.svelte';
	import { toastPromise } from '$lib/stores/toaster';
	import { OAUTH_PROVIDERS } from '$lib/config/app';
	import { apiBaseURL } from '$lib/utils/api-base';

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

	// Identity tab state
	interface MeGamertag {
		id: string;
		tag: string;
		status: string;
	}
	interface MeTeamMembership {
		gamertag_id: string;
		is_owner: boolean;
		is_manager: boolean;
		joined_at: string;
		left_at: string | null;
	}
	interface MeTeam {
		id: string;
		name: string;
		slug: string;
		status: string;
		membership: MeTeamMembership;
	}
	interface MeIdentity {
		default_gamertag: MeGamertag | null;
		gamertags: MeGamertag[];
		teams: MeTeam[];
	}

	let myGamertags = $state<MeGamertag[]>([]);
	let myTeams = $state<MeTeam[]>([]);
	let myDefaultGamertagId = $state<string | null>(null);
	let identityLoading = $state(false);
	let newTagInput = $state('');
	let addingTag = $state(false);
	let removingTagId = $state<string | null>(null);
	let settingDefaultId = $state<string | null>(null);
	let teamCreateOpen = $state(false);
	let teamCreateName = $state('');
	let teamCreateSlug = $state('');
	let teamSlugTouched = $state(false);
	let creatingTeam = $state(false);

	const identityBaseURL = apiBaseURL();

	function slugify(value: string): string {
		return value
			.toLowerCase()
			.trim()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');
	}

	$effect(() => {
		// Auto-derive the slug from the name until the user touches the slug
		// field directly. After that, leave their typed value alone.
		if (!teamSlugTouched) teamCreateSlug = slugify(teamCreateName);
	});

	async function loadIdentity() {
		if (!auth.token) return;
		identityLoading = true;
		try {
			const res = await fetch(`${identityBaseURL}/api/me`, {
				headers: { Authorization: auth.token }
			});
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const data = (await res.json()) as MeIdentity;
			myGamertags = data.gamertags ?? [];
			myTeams = data.teams ?? [];
			myDefaultGamertagId = data.default_gamertag?.id ?? null;
		} catch {
			// Don't toast — the tab just stays empty. Network blip is fine.
			myGamertags = [];
			myTeams = [];
			myDefaultGamertagId = null;
		} finally {
			identityLoading = false;
		}
	}

	async function addGamertag() {
		if (!auth.user || !newTagInput.trim()) return;
		const tag = newTagInput.trim();
		addingTag = true;
		try {
			await toastPromise(pb.collection('gamertags').create({ user: auth.user.id, tag }), {
				loading: { title: 'Adding gamertag' },
				success: { title: 'Added', description: tag },
				errorTitle: 'Add failed',
				errorDescription: (err) => {
					const m = err instanceof Error ? err.message : 'Failed';
					return m.toLowerCase().includes('unique') ? 'You already own that gamertag.' : m;
				}
			});
			newTagInput = '';
			await loadIdentity();
		} catch {
			// toast already shown
		} finally {
			addingTag = false;
		}
	}

	async function removeGamertag(id: string) {
		if (!auth.user) return;
		removingTagId = id;
		try {
			await toastPromise(pb.collection('gamertags').delete(id), {
				loading: { title: 'Removing' },
				success: { title: 'Removed' },
				errorTitle: 'Remove failed'
			});
			await loadIdentity();
		} catch {
			// toast already shown
		} finally {
			removingTagId = null;
		}
	}

	async function setDefaultGamertag(id: string) {
		if (!auth.user) return;
		settingDefaultId = id;
		try {
			await toastPromise(pb.collection('users').update(auth.user.id, { default_gamertag: id }), {
				loading: { title: 'Setting default' },
				success: { title: 'Default updated' },
				errorTitle: 'Update failed'
			});
			await loadIdentity();
		} catch {
			// toast already shown
		} finally {
			settingDefaultId = null;
		}
	}

	function openTeamCreate() {
		teamCreateOpen = true;
		teamCreateName = '';
		teamCreateSlug = '';
		teamSlugTouched = false;
	}

	function closeTeamCreate() {
		teamCreateOpen = false;
	}

	async function createTeam() {
		if (!auth.user || !myDefaultGamertagId) return;
		const name = teamCreateName.trim();
		const slug = teamCreateSlug.trim();
		if (!name || !slug) return;
		creatingTeam = true;
		try {
			const team = await toastPromise(
				pb.collection('teams').create({ name, slug, created_by: auth.user.id }),
				{
					loading: { title: 'Creating team' },
					success: { title: 'Team created', description: name },
					errorTitle: 'Create failed',
					errorDescription: (err) => {
						const m = err instanceof Error ? err.message : 'Failed';
						return m.toLowerCase().includes('unique') ? 'Slug is already taken.' : m;
					}
				}
			);
			// Auto-roster the creator as owner + manager so they can manage
			// the team without an admin intervention. PB rules permit this
			// because the acting user is the team's created_by.
			await pb.collection('rosters').create({
				team: team.id,
				gamertag: myDefaultGamertagId,
				is_owner: true,
				is_manager: true,
				joined_at: new Date().toISOString().replace('T', ' ').replace(/\..+$/, '.000Z')
			});
			closeTeamCreate();
			await loadIdentity();
		} catch {
			// toast already shown
		} finally {
			creatingTeam = false;
		}
	}

	onMount(async () => {
		try {
			const methods = await pb.collection('users').listAuthMethods();
			enabledProviders = methods.oauth2?.providers?.map((p) => p.name) ?? [];
		} catch {
			enabledProviders = [];
		}
		await loadLinkedAuths();
		await loadIdentity();
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
			const data: Record<string, unknown> = { name: displayName, bio, location };
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

	// Email change goes through PocketBase's built-in requestEmailChange flow:
	// PB sends a confirmation token to the NEW address, and on confirm it flips
	// `email`, sets `verified=false`, and re-fires verification. We don't hold
	// the address change ourselves; the user clicks the link in their inbox to
	// commit. Old-email notification + current-password re-auth gate are
	// deferred — PB doesn't surface them and we don't have a mailer outside
	// of PB's templated path yet (M7 log notes this).
	let changingEmail = $state(false);
	const emailDirty = $derived(email !== (auth.user?.email ?? ''));

	async function requestEmailChangeFlow() {
		if (!auth.user || !emailDirty) return;
		changingEmail = true;
		try {
			await toastPromise(pb.collection('users').requestEmailChange(email), {
				loading: { title: 'Sending confirmation', description: email },
				success: {
					title: 'Check your inbox',
					description: `We sent a confirmation link to ${email}. Until you click it, your account email stays ${auth.user.email}.`
				},
				errorTitle: 'Email change failed'
			});
		} catch {
			// toast already shown
		} finally {
			changingEmail = false;
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
			'Mark your account as deleted? Your gamertag, roster, and team history stay attached to "[deleted user]" so past games remain readable, but your email, display name, bio, and avatar are blanked and you cannot log back in.'
		);
		if (!confirmed) return;
		deleting = true;
		try {
			await toastPromise(pb.collection('users').update(auth.user.id, { is_deleted: true }), {
				loading: { title: 'Deleting account' },
				success: { title: 'Deleted', description: 'Your account has been tombstoned.' },
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
	<!-- General -->
	<section class="mb-10">
		<h2 class="mb-4 border-b border-surface-300-700 pb-2 h3">General</h2>
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

				<!-- Username — immutable. Locked at the DB layer via the
					     users_username_immutable hook; new gamertags are the
					     mechanism for changing how others see you. -->
				<label class="label">
					<span>Username</span>
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<LockIcon class="size-4" />
						</div>
						<input
							type="text"
							class="input opacity-70"
							value={_user?.username ?? ''}
							readonly
							title="Usernames cannot be changed. Add a gamertag to use a different displayed handle."
						/>
					</div>
				</label>

				<!-- Input Groups -->
				<label class="label">
					<span>Display Name</span>
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<UserIcon class="size-4" />
						</div>
						<input type="text" class="input" bind:value={displayName} />
					</div>
				</label>

				<!-- Email change goes through PB's requestEmailChange. The save
					     button below intentionally does NOT send email so the user
					     has to opt in to the verification flow. -->
				<label class="label">
					<span>Email</span>
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<MailIcon class="size-4" />
						</div>
						<input type="email" class="input" bind:value={email} />
						<button
							type="button"
							class="btn preset-tonal btn-sm"
							onclick={requestEmailChangeFlow}
							disabled={changingEmail || !emailDirty || !email.trim()}
							title="Send a confirmation link to the new address"
						>
							{#if changingEmail}<LoaderIcon class="size-4 animate-spin" />{/if}
							<span>Change email</span>
						</button>
					</div>
					{#if emailDirty}
						<p class="text-xs opacity-70">
							Clicking Save updates everything except email. Click "Change email" to send a
							confirmation link to the new address — your email won't change until you click that
							link.
						</p>
					{/if}
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
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<MapPinIcon class="size-4" />
						</div>
						<input
							type="text"
							class="input"
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
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<LockIcon class="size-4" />
						</div>
						<input
							type="password"
							class="input"
							placeholder="••••••••"
							bind:value={oldPassword}
							required
						/>
					</div>
				</label>

				<label class="label">
					<span>New Password</span>
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<LockIcon class="size-4" />
						</div>
						<input
							type="password"
							class="input"
							placeholder="••••••••"
							bind:value={newPassword}
							minlength="8"
							required
						/>
					</div>
				</label>

				<label class="label">
					<span>Confirm New Password</span>
					<div class="field-group grid-cols-[auto_1fr_auto]">
						<div class="flex items-center justify-center preset-tonal px-3">
							<LockIcon class="size-4" />
						</div>
						<input
							type="password"
							class="input"
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
	</section>

	<!-- Gamertags & Teams -->
	<section class="mb-10">
		<h2 class="mb-4 border-b border-surface-300-700 pb-2 h3">Gamertags &amp; Teams</h2>
		<div class="space-y-6">
			<!-- M23d: pending invites + sent requests, inline with the rest
			     of the team management surface. The component handles its
			     own load + actions. -->
			<PendingRequests />
			<!-- My Gamertags -->
			<div class="space-y-4 card p-6">
				<div class="flex items-center justify-between gap-3">
					<h2 class="h4">My Gamertags</h2>
					{#if identityLoading}
						<LoaderIcon class="size-4 animate-spin opacity-50" />
					{/if}
				</div>
				<p class="text-sm opacity-70">
					Add the handles you play under. Your default is what other players see by default.
				</p>

				{#if myGamertags.length === 0}
					<p class="text-sm opacity-50">No gamertags yet.</p>
				{:else}
					<div class="space-y-2">
						{#each myGamertags as gt (gt.id)}
							{@const isDefault = gt.id === myDefaultGamertagId}
							{@const isRemoving = removingTagId === gt.id}
							{@const isSettingDefault = settingDefaultId === gt.id}
							<div
								class="flex items-center justify-between rounded-md border border-surface-300-700 p-3"
							>
								<div class="flex items-center gap-3">
									<TagIcon class="size-4 opacity-50" />
									<span class="font-mono">{gt.tag}</span>
									{#if isDefault}
										<span class="badge preset-tonal-primary text-xs">
											<StarIcon class="size-3" />
											Default
										</span>
									{/if}
									{#if gt.status === 'blocked'}
										<span class="badge preset-tonal-error text-xs" title="Blocked by an admin">
											<ShieldAlertIcon class="size-3" />
											Blocked
										</span>
									{:else if gt.status === 'pending'}
										<span class="badge preset-tonal-warning text-xs" title="Pending admin review">
											Pending
										</span>
									{:else if gt.status === 'approved'}
										<span class="badge preset-tonal-success text-xs" title="Approved by an admin">
											Approved
										</span>
									{/if}
								</div>
								<div class="flex items-center gap-2">
									{#if !isDefault && gt.status !== 'blocked'}
										<button
											class="btn preset-tonal btn-sm"
											onclick={() => setDefaultGamertag(gt.id)}
											disabled={isSettingDefault}
										>
											{#if isSettingDefault}
												<LoaderIcon class="size-4 animate-spin" />
											{:else}
												<StarIcon class="size-4" />
											{/if}
											<span>Set default</span>
										</button>
									{/if}
									{#if gt.status !== 'blocked'}
										<button
											class="btn preset-tonal-error btn-sm"
											onclick={() => removeGamertag(gt.id)}
											disabled={isRemoving}
											title={isDefault
												? 'Removing your default will clear the default selection'
												: 'Remove gamertag'}
										>
											{#if isRemoving}
												<LoaderIcon class="size-4 animate-spin" />
											{:else}
												<Trash2Icon class="size-4" />
											{/if}
										</button>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}

				<form
					class="flex items-end gap-2"
					onsubmit={(e) => {
						e.preventDefault();
						addGamertag();
					}}
				>
					<label class="label flex-1">
						<span>Add a gamertag</span>
						<div class="field-group grid-cols-[auto_1fr_auto]">
							<div class="flex items-center justify-center preset-tonal px-3">
								<TagIcon class="size-4" />
							</div>
							<input
								type="text"
								class="input"
								placeholder="e.g. Stewball32"
								maxlength="32"
								bind:value={newTagInput}
								disabled={addingTag}
							/>
						</div>
					</label>
					<button
						type="submit"
						class="btn preset-filled"
						disabled={addingTag || !newTagInput.trim()}
					>
						{#if addingTag}
							<LoaderIcon class="size-4 animate-spin" />
						{:else}
							<PlusIcon class="size-4" />
						{/if}
						<span>Add</span>
					</button>
				</form>
			</div>

			<!-- My Teams -->
			<div class="space-y-4 card p-6">
				<div class="flex items-center justify-between gap-3">
					<h2 class="h4">My Teams</h2>
					<button
						class="btn preset-filled btn-sm"
						onclick={openTeamCreate}
						disabled={!myDefaultGamertagId}
						title={myDefaultGamertagId ? 'Create a new team' : 'Pick a default gamertag first'}
					>
						<PlusIcon class="size-4" />
						<span>Create team</span>
					</button>
				</div>
				<p class="text-sm opacity-70">Teams you are currently rostered on.</p>

				{#if myTeams.length === 0}
					<p class="text-sm opacity-50">Not on any teams yet.</p>
				{:else}
					<div class="space-y-2">
						{#each myTeams as team (team.id)}
							<div
								class="flex items-center justify-between rounded-md border border-surface-300-700 p-3"
							>
								<div class="flex items-center gap-3">
									<UsersIcon class="size-4 opacity-50" />
									<div>
										<div class="flex items-center gap-2">
											<p class="font-semibold">{team.name}</p>
											{#if team.status === 'blocked'}
												<span class="badge preset-tonal-error text-xs" title="Blocked by an admin">
													<ShieldAlertIcon class="size-3" />
													Blocked
												</span>
											{:else if team.status === 'pending'}
												<span
													class="badge preset-tonal-warning text-xs"
													title="Pending admin review"
												>
													Pending
												</span>
											{:else if team.status === 'approved'}
												<span
													class="badge preset-tonal-success text-xs"
													title="Approved by an admin"
												>
													Approved
												</span>
											{/if}
										</div>
										<p class="text-xs opacity-50">{team.slug}</p>
									</div>
								</div>
								<div class="flex items-center gap-2">
									{#if team.membership.is_owner}
										<span class="badge preset-tonal-warning text-xs">
											<CrownIcon class="size-3" />
											Owner
										</span>
									{/if}
									{#if team.membership.is_manager}
										<span class="badge preset-tonal-primary text-xs">
											<BriefcaseIcon class="size-3" />
											Manager
										</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Team Create Modal (inline; Skeleton's Modal needs portal setup, dialog is simpler) -->
			{#if teamCreateOpen}
				<div
					class="fixed inset-0 z-50 flex items-center justify-center bg-surface-950/50 p-4"
					role="dialog"
					aria-modal="true"
				>
					<div class="w-full max-w-md space-y-4 card p-6">
						<h3 class="h4">Create team</h3>
						<label class="label">
							<span>Name</span>
							<div class="field-group grid-cols-[auto_1fr_auto]">
								<div class="flex items-center justify-center preset-tonal px-3">
									<UsersIcon class="size-4" />
								</div>
								<input
									type="text"
									class="input"
									maxlength="60"
									bind:value={teamCreateName}
									placeholder="NorCal Halo"
								/>
							</div>
						</label>
						<label class="label">
							<span>Slug</span>
							<div class="field-group grid-cols-[auto_1fr_auto]">
								<div
									class="flex items-center justify-center preset-tonal px-3 font-mono text-xs opacity-50"
								>
									/teams/
								</div>
								<input
									type="text"
									class="input"
									maxlength="60"
									bind:value={teamCreateSlug}
									onkeydown={() => (teamSlugTouched = true)}
									placeholder="norcal-halo"
								/>
							</div>
							<p class="text-xs opacity-50">Lowercase letters, numbers, and dashes only.</p>
						</label>
						<div class="flex justify-end gap-2">
							<button class="btn preset-tonal" onclick={closeTeamCreate} disabled={creatingTeam}>
								Cancel
							</button>
							<button
								class="btn preset-filled"
								onclick={createTeam}
								disabled={creatingTeam || !teamCreateName.trim() || !teamCreateSlug.trim()}
							>
								{#if creatingTeam}<LoaderIcon class="size-4 animate-spin" />{/if}
								<span>Create</span>
							</button>
						</div>
					</div>
				</div>
			{/if}
		</div>
	</section>

	<!-- Connected Accounts -->
	<section class="mb-10">
		<h2 class="mb-4 border-b border-surface-300-700 pb-2 h3">Connected Accounts</h2>
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
	</section>
</div>
