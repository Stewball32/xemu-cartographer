<script lang="ts">
	// /admin/roles/ — M08 admin UI for managing role grants + user bans.
	// Two tabs: Roles catalog (read-only — baseline + future roles created via
	// PB admin UI) and Assignments (per-user role grant/revoke + ban controls).
	//
	// /admin/+layout.ts enforces isAdmin.

	import { onMount } from 'svelte';
	import { Tabs } from '@skeletonlabs/skeleton-svelte';
	import {
		ShieldIcon,
		UserIcon,
		BanIcon,
		ClockIcon,
		CheckIcon,
		LoaderIcon,
		PlusIcon,
		RefreshCwIcon,
		SearchIcon,
		Trash2Icon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { apiBaseURL } from '$lib/utils/api-base';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup, SortState } from '$lib/components/ui/data-table';

	const baseURL = apiBaseURL();

	interface RoleRow {
		id: string;
		slug: string;
		label: string;
		description: string;
		level: number;
		created: string;
		updated: string;
	}

	interface RoleAssignment {
		user: string;
		role_slug: string;
	}

	interface UserRow {
		id: string;
		username: string;
		email: string;
		is_banned: boolean;
		banned_until: string;
		is_deleted: boolean;
		roles: string[]; // computed
	}

	const BASELINE_SLUGS = new Set(['superuser', 'admin', 'member']);

	let activeTab = $state<'roles' | 'assignments'>('roles');
	let roles = $state<RoleRow[]>([]);
	let assignments = $state<RoleAssignment[]>([]);
	let users = $state<UserRow[]>([]);
	let loading = $state(true);
	let userFilter = $state('');
	let userSort = $state<SortState>({ key: 'username', dir: 'asc' });
	let rolesSort = $state<SortState>({ key: 'level', dir: 'desc' });

	let grantDialogOpen = $state(false);
	let grantTarget = $state<UserRow | null>(null);
	let grantSlug = $state('');
	let grantBusy = $state(false);

	let banDialogOpen = $state(false);
	let banTarget = $state<UserRow | null>(null);
	let banMode = $state<'ban' | 'timeout'>('ban');
	let banReason = $state('');
	let banExpiresAt = $state('');
	let banBusy = $state(false);

	async function authHeaders() {
		return { Authorization: pb.authStore.token };
	}

	async function getAdmin<T>(path: string): Promise<T> {
		const res = await fetch(`${baseURL}${path}`, {
			headers: { ...(await authHeaders()) }
		});
		if (!res.ok) {
			const text = await res.text();
			throw new Error(text || `HTTP ${res.status}`);
		}
		return (await res.json()) as T;
	}

	async function postAdmin(path: string, body: object) {
		const res = await fetch(`${baseURL}${path}`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', ...(await authHeaders()) },
			body: JSON.stringify(body)
		});
		if (!res.ok) {
			const text = await res.text();
			throw new Error(text || `HTTP ${res.status}`);
		}
		return res.json();
	}

	async function deleteAdmin(path: string, body?: object) {
		const res = await fetch(`${baseURL}${path}`, {
			method: 'DELETE',
			headers: { 'Content-Type': 'application/json', ...(await authHeaders()) },
			body: body ? JSON.stringify(body) : undefined
		});
		if (!res.ok) {
			const text = await res.text();
			throw new Error(text || `HTTP ${res.status}`);
		}
		return res.json();
	}

	async function load() {
		try {
			loading = true;
			const [rolesList, assignmentList, userList] = await Promise.all([
				pb.collection('roles').getFullList<RoleRow>({ sort: '-level' }),
				getAdmin<RoleAssignment[]>('/api/admin/users/role-assignments'),
				pb.collection('users').getFullList<UserRow>({ sort: 'username' })
			]);
			const userRolesByUser: Record<string, string[]> = {};
			for (const a of assignmentList) {
				(userRolesByUser[a.user] ??= []).push(a.role_slug);
			}
			roles = rolesList;
			assignments = assignmentList;
			users = userList.map((u) => ({
				...u,
				roles: userRolesByUser[u.id] ?? []
			}));
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function openGrant(user: UserRow) {
		grantTarget = user;
		grantSlug = roles[0]?.slug ?? '';
		grantDialogOpen = true;
	}

	async function grant() {
		if (!grantTarget || !grantSlug) return;
		const target = grantTarget;
		try {
			grantBusy = true;
			await toastPromise(postAdmin(`/api/admin/users/${target.id}/roles`, { slug: grantSlug }), {
				loading: { title: 'Granting', description: `${grantSlug} → ${target.username}` },
				success: { title: 'Granted', description: `${grantSlug} → ${target.username}` },
				errorTitle: 'Grant failed'
			});
			grantDialogOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			grantBusy = false;
		}
	}

	async function revoke(user: UserRow, slug: string) {
		const ok = await confirmToast({
			title: `Revoke ${slug}`,
			description: `Remove the "${slug}" role from ${user.username}? An audit_log entry will be written.`,
			confirmLabel: 'Revoke',
			type: 'warning'
		});
		if (!ok) return;
		try {
			await toastPromise(
				deleteAdmin(`/api/admin/users/${user.id}/roles/${slug}`, {
					reason: 'Revoked via admin UI'
				}),
				{
					loading: { title: 'Revoking', description: `${slug} ← ${user.username}` },
					success: { title: 'Revoked', description: `${slug} ← ${user.username}` },
					errorTitle: 'Revoke failed'
				}
			);
			await load();
		} catch {
			// toast already shown
		}
	}

	function openBan(user: UserRow, mode: 'ban' | 'timeout') {
		banTarget = user;
		banMode = mode;
		banReason = '';
		banExpiresAt = '';
		banDialogOpen = true;
	}

	async function applyBan() {
		if (!banTarget) return;
		const target = banTarget;
		try {
			banBusy = true;
			const path =
				banMode === 'ban'
					? `/api/admin/users/${target.id}/ban`
					: `/api/admin/users/${target.id}/timeout`;
			// datetime-local inputs return "YYYY-MM-DDTHH:mm" in local time
			// with no timezone — PB's date field silently rejects that shape
			// and stores nothing. Convert to a UTC ISO string before POST.
			const expiresAtISO = banExpiresAt ? new Date(banExpiresAt).toISOString() : '';
			const body =
				banMode === 'ban' ? { reason: banReason } : { reason: banReason, expires_at: expiresAtISO };
			await toastPromise(postAdmin(path, body), {
				loading: {
					title: banMode === 'ban' ? 'Banning' : 'Timing out',
					description: target.username
				},
				success: {
					title: banMode === 'ban' ? 'Banned' : 'Timed out',
					description: target.username
				},
				errorTitle: 'Action failed'
			});
			banDialogOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			banBusy = false;
		}
	}

	async function unban(user: UserRow) {
		const ok = await confirmToast({
			title: 'Unban',
			description: `Lift the ban on ${user.username}? An audit_log entry will be written.`,
			confirmLabel: 'Unban',
			type: 'warning'
		});
		if (!ok) return;
		try {
			await toastPromise(
				postAdmin(`/api/admin/users/${user.id}/unban`, { reason: 'Unbanned via admin UI' }),
				{
					loading: { title: 'Unbanning', description: user.username },
					success: { title: 'Unbanned', description: user.username },
					errorTitle: 'Unban failed'
				}
			);
			await load();
		} catch {
			// toast already shown
		}
	}

	const filteredUsers = $derived.by<UserRow[]>(() => {
		const q = userFilter.trim().toLowerCase();
		const base = !q
			? users
			: users.filter(
					(u) =>
						u.username?.toLowerCase().includes(q) ||
						u.email?.toLowerCase().includes(q) ||
						u.roles.some((r) => r.toLowerCase().includes(q))
				);
		const s = userSort;
		if (!s) return base;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...base].sort((a, b) => {
			const av = String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv = String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	const rolesWithCount = $derived.by(() => {
		const count: Record<string, number> = {};
		for (const a of assignments) {
			count[a.role_slug] = (count[a.role_slug] ?? 0) + 1;
		}
		const augmented = roles.map((r) => ({
			...r,
			memberCount: count[r.slug] ?? 0
		}));
		const s = rolesSort;
		if (!s) return augmented;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...augmented].sort((a, b) => {
			if (s.key === 'level' || s.key === 'memberCount') {
				return (
					(((a as unknown as Record<string, number>)[s.key] ?? 0) -
						((b as unknown as Record<string, number>)[s.key] ?? 0)) *
					dir
				);
			}
			const av = String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv = String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage roles.' });
			return;
		}
		void load();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Roles & permissions"
		description="Manage user role grants and account-level moderation (ban + timeout). Baseline roles (superuser / admin / member) are seeded automatically; additional slugs are created through the PocketBase admin UI as future milestones (M16+ tournaments, M17 discord) need them."
	/>

	<Tabs value={activeTab} onValueChange={(e) => (activeTab = e.value as 'roles' | 'assignments')}>
		<Tabs.List>
			<Tabs.Trigger value="roles">
				<ShieldIcon class="size-4" />
				<span>Roles</span>
			</Tabs.Trigger>
			<Tabs.Trigger value="assignments">
				<UserIcon class="size-4" />
				<span>Assignments</span>
			</Tabs.Trigger>
			<Tabs.Indicator />
		</Tabs.List>

		<Tabs.Content value="roles">
			<div class="flex flex-col gap-4 pt-4">
				<div class="flex items-center justify-end gap-2">
					<button
						class="btn preset-tonal"
						onclick={() => load()}
						disabled={loading}
						aria-label="Refresh"
					>
						{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
								class="size-4"
							/>{/if}
						<span>Refresh</span>
					</button>
				</div>

				{#snippet slugCell({ row }: { row: RoleRow & { memberCount: number } })}
					<span class="badge preset-tonal-primary">
						<ShieldIcon class="size-3" />
						<span class="font-mono">{row.slug}</span>
					</span>
					{#if BASELINE_SLUGS.has(row.slug)}
						<span class="badge preset-tonal text-xs">baseline</span>
					{/if}
				{/snippet}

				<Card size="flush" class="overflow-x-auto">
					<DataTable
						rows={rolesWithCount}
						groups={[
							{
								columns: [
									{ key: 'slug', label: 'Slug', cell: slugCell },
									{ key: 'label', label: 'Label' },
									{ key: 'level', label: 'Level' },
									{ key: 'memberCount', label: 'Members' },
									{ key: 'description', label: 'Description' }
								]
							}
						] satisfies DataColumnGroup<RoleRow & { memberCount: number }>[]}
						rowKey={(r) => r.id}
						density="comfortable"
						sort={rolesSort}
						onSortChange={(s) => (rolesSort = s)}
						loading={loading && rolesWithCount.length === 0}
						emptyMessage="No roles defined."
					/>
				</Card>
			</div>
		</Tabs.Content>

		<Tabs.Content value="assignments">
			<div class="flex flex-col gap-4 pt-4">
				<div class="flex items-center justify-between gap-2">
					<div class="input-group flex-1 grid-cols-[auto_1fr]">
						<div class="ig-cell preset-tonal">
							<SearchIcon class="size-4" />
						</div>
						<input
							type="search"
							class="ig-input"
							placeholder="Filter by username, email, or role"
							bind:value={userFilter}
						/>
					</div>
					<button
						class="btn preset-tonal"
						onclick={() => load()}
						disabled={loading}
						aria-label="Refresh"
					>
						{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
								class="size-4"
							/>{/if}
						<span>Refresh</span>
					</button>
				</div>

				{#snippet usernameCell({ row }: { row: UserRow })}
					<div class="flex flex-col">
						<span class="font-medium">{row.username}</span>
						<span class="text-xs opacity-60">{row.email}</span>
					</div>
				{/snippet}
				{#snippet rolesCell({ row }: { row: UserRow })}
					<div class="flex flex-wrap items-center gap-1">
						{#each row.roles as slug (slug)}
							<button
								type="button"
								class="badge preset-tonal-primary"
								title="Click to revoke"
								onclick={() => revoke(row, slug)}
							>
								<ShieldIcon class="size-3" />
								<span class="font-mono text-xs">{slug}</span>
								<Trash2Icon class="size-3" />
							</button>
						{/each}
						{#if row.roles.length === 0}
							<span class="text-xs opacity-50">—</span>
						{/if}
					</div>
				{/snippet}
				{#snippet stateCell({ row }: { row: UserRow })}
					{#if row.is_deleted}
						<span class="badge preset-tonal-error">deleted</span>
					{:else if row.is_banned}
						{#if row.banned_until}
							<span class="badge preset-tonal-warning" title={row.banned_until}>
								<ClockIcon class="size-3" />
								<span>timeout</span>
							</span>
						{:else}
							<span class="badge preset-tonal-error">
								<BanIcon class="size-3" />
								<span>banned</span>
							</span>
						{/if}
					{:else}
						<span class="badge preset-tonal-success">
							<CheckIcon class="size-3" />
							<span>active</span>
						</span>
					{/if}
				{/snippet}
				{#snippet actionsCell({ row }: { row: UserRow })}
					<div
						role="presentation"
						class="inline-flex items-center justify-end gap-1"
						onclick={(e) => e.stopPropagation()}
						onkeydown={(e) => e.stopPropagation()}
					>
						<button
							class="btn-icon preset-tonal btn-sm"
							title="Grant role"
							onclick={() => openGrant(row)}
							disabled={row.is_deleted}
						>
							<PlusIcon class="size-4" />
						</button>
						{#if row.is_banned}
							<button
								class="btn-icon preset-tonal-success btn-sm"
								title="Unban"
								onclick={() => unban(row)}
								disabled={row.is_deleted}
							>
								<CheckIcon class="size-4" />
							</button>
						{:else}
							<button
								class="btn-icon preset-tonal-warning btn-sm"
								title="Timeout"
								onclick={() => openBan(row, 'timeout')}
								disabled={row.is_deleted}
							>
								<ClockIcon class="size-4" />
							</button>
							<button
								class="btn-icon preset-tonal-error btn-sm"
								title="Ban"
								onclick={() => openBan(row, 'ban')}
								disabled={row.is_deleted}
							>
								<BanIcon class="size-4" />
							</button>
						{/if}
					</div>
				{/snippet}

				<Card size="flush" class="overflow-x-auto">
					<DataTable
						rows={filteredUsers}
						groups={[
							{
								columns: [
									{ key: 'username', label: 'User', cell: usernameCell },
									{ key: 'roles_summary', label: 'Roles', cell: rolesCell, sortable: false },
									{ key: 'state', label: 'State', cell: stateCell, sortable: false },
									{ key: 'actions', label: '', cell: actionsCell, sortable: false, align: 'right' }
								]
							}
						] satisfies DataColumnGroup<UserRow>[]}
						rowKey={(r) => r.id}
						density="comfortable"
						sort={userSort}
						onSortChange={(s) => (userSort = s)}
						secondarySort={{ key: 'username', dir: 'asc' }}
						loading={loading && filteredUsers.length === 0}
						emptyMessage={userFilter ? 'No matches.' : 'No users.'}
					/>
				</Card>
			</div>
		</Tabs.Content>
	</Tabs>
</div>

<Dialog
	open={grantDialogOpen}
	onClose={() => {
		if (!grantBusy) grantDialogOpen = false;
	}}
	title="Grant role"
>
	{#if grantTarget}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				void grant();
			}}
			class="flex flex-col gap-3"
		>
			<p class="text-sm">
				Grant a role to <span class="font-medium">{grantTarget.username}</span>.
			</p>
			<label class="label">
				<span class="label-text">Role</span>
				<select class="select" bind:value={grantSlug} disabled={grantBusy}>
					{#each roles as r (r.id)}
						<option value={r.slug}>{r.slug} ({r.label}, level {r.level})</option>
					{/each}
				</select>
			</label>
			<p class="text-xs opacity-60">
				Idempotent — granting a role the user already holds is a no-op (no duplicate audit row).
			</p>
			<div class="flex justify-end gap-2">
				<button
					type="button"
					class="btn preset-tonal"
					onclick={() => (grantDialogOpen = false)}
					disabled={grantBusy}
				>
					Cancel
				</button>
				<button type="submit" class="btn preset-filled" disabled={grantBusy || !grantSlug}>
					{#if grantBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
					<span>Grant</span>
				</button>
			</div>
		</form>
	{/if}
</Dialog>

<Dialog
	open={banDialogOpen}
	onClose={() => {
		if (!banBusy) banDialogOpen = false;
	}}
	title={banMode === 'ban' ? 'Ban user' : 'Timeout user'}
>
	{#if banTarget}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				void applyBan();
			}}
			class="flex flex-col gap-3"
		>
			<p class="text-sm">
				{banMode === 'ban' ? 'Indefinitely ban' : 'Time out'}
				<span class="font-medium">{banTarget.username}</span>.
				{#if banMode === 'ban'}
					Their next login attempt will fail with HTTP 403 until you lift the ban.
				{:else}
					Their next login attempt will fail with HTTP 403 until the expiry passes (passive expiry —
					no cron clears the flag, but the AuthRule admits once the timestamp is in the past).
				{/if}
			</p>
			{#if banMode === 'timeout'}
				<label class="label">
					<span class="label-text">Expires at (UTC, ISO 8601)</span>
					<input type="datetime-local" class="input" bind:value={banExpiresAt} disabled={banBusy} />
				</label>
			{/if}
			<label class="label">
				<span class="label-text">Reason (optional, written to audit log)</span>
				<input
					type="text"
					class="input"
					bind:value={banReason}
					placeholder="e.g. CoC §3.2"
					disabled={banBusy}
				/>
			</label>
			<div class="flex justify-end gap-2">
				<button
					type="button"
					class="btn preset-tonal"
					onclick={() => (banDialogOpen = false)}
					disabled={banBusy}
				>
					Cancel
				</button>
				<button
					type="submit"
					class={banMode === 'ban' ? 'preset-filled-error btn' : 'preset-filled-warning btn'}
					disabled={banBusy || (banMode === 'timeout' && !banExpiresAt)}
				>
					{#if banBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
					<span>{banMode === 'ban' ? 'Ban' : 'Timeout'}</span>
				</button>
			</div>
		</form>
	{/if}
</Dialog>
