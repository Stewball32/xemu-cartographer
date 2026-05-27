<script lang="ts">
	// Rosters (M7f). Admin moderation surface for teams and roster history.
	// Was part of the monolithic /admin/identity/ page; split out so admins
	// can land directly on the surface they need. /admin/+layout.ts enforces
	// isAdmin; this page assumes it.

	import { onMount } from 'svelte';
	import { Tabs } from '@skeletonlabs/skeleton-svelte';
	import {
		LoaderIcon,
		PencilIcon,
		RefreshCwIcon,
		SearchIcon,
		Trash2Icon,
		UserIcon,
		UsersIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import DataTable from '$lib/components/ui/DataTable.svelte';
	import type { DataColumnGroup, SortState } from '$lib/components/ui/data-table';

	interface UserExpand {
		id: string;
		username: string;
		email: string;
	}
	interface GamertagRow {
		id: string;
		user: string;
		tag: string;
		blocked: boolean;
		expand?: { user?: UserExpand };
	}
	interface TeamRow {
		id: string;
		name: string;
		slug: string;
		created_by: string;
		created: string;
		updated: string;
		expand?: { created_by?: UserExpand };
	}
	interface RosterRow {
		id: string;
		team: string;
		gamertag: string;
		is_owner: boolean;
		is_manager: boolean;
		joined_at: string;
		left_at: string;
		created: string;
		updated: string;
		expand?: {
			team?: TeamRow;
			gamertag?: GamertagRow & { expand?: { user?: UserExpand } };
		};
	}

	let activeTab = $state('teams');

	// Teams state
	let tmRows = $state<TeamRow[]>([]);
	let tmLoading = $state(true);
	let tmFilter = $state('');
	let tmSort = $state<SortState>({ key: 'name', dir: 'asc' });
	let tmDialogOpen = $state(false);
	let tmForm = $state({ id: '', name: '', slug: '' });
	let tmFormBusy = $state(false);
	let tmDeleteBusy = $state<Record<string, boolean>>({});

	// Rosters state
	let rsRows = $state<RosterRow[]>([]);
	let rsLoading = $state(true);
	let rsFilter = $state('');
	let rsSort = $state<SortState>({ key: 'team', dir: 'asc' });
	let rsDialogOpen = $state(false);
	let rsForm = $state({
		id: '',
		is_owner: false,
		is_manager: false,
		joined_at: '',
		left_at: ''
	});
	let rsFormBusy = $state(false);
	let rsDeleteBusy = $state<Record<string, boolean>>({});

	async function loadTeams() {
		try {
			tmLoading = true;
			tmRows = await pb
				.collection('teams')
				.getFullList<TeamRow>({ expand: 'created_by', sort: 'name' });
		} catch (err) {
			toaster.error({ title: 'Load teams failed', description: describeAsyncError(err) });
		} finally {
			tmLoading = false;
		}
	}

	async function loadRosters() {
		try {
			rsLoading = true;
			rsRows = await pb
				.collection('rosters')
				.getFullList<RosterRow>({ expand: 'team,gamertag,gamertag.user', sort: 'team.name' });
		} catch (err) {
			toaster.error({ title: 'Load rosters failed', description: describeAsyncError(err) });
		} finally {
			rsLoading = false;
		}
	}

	function openTmEdit(row: TeamRow) {
		tmForm = { id: row.id, name: row.name, slug: row.slug };
		tmDialogOpen = true;
	}

	async function saveTeam() {
		const f = tmForm;
		if (!f.name.trim() || !f.slug.trim()) {
			toaster.error({ title: 'Invalid', description: 'Name and slug are required.' });
			return;
		}
		try {
			tmFormBusy = true;
			await toastPromise(
				pb.collection('teams').update(f.id, { name: f.name.trim(), slug: f.slug.trim() }),
				{
					loading: { title: 'Saving', description: f.name },
					success: { title: 'Saved', description: f.name },
					errorTitle: 'Save failed'
				}
			);
			tmDialogOpen = false;
			await loadTeams();
		} catch {
			// toast already shown
		} finally {
			tmFormBusy = false;
		}
	}

	async function deleteTeam(row: TeamRow) {
		const ok = await confirmToast({
			title: 'Delete team',
			description: `Permanently remove team "${row.name}"? Roster history persists (relations are CascadeDelete=false), but the team metadata is gone.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		tmDeleteBusy = { ...tmDeleteBusy, [row.id]: true };
		try {
			await toastPromise(pb.collection('teams').delete(row.id), {
				loading: { title: 'Deleting', description: row.name },
				success: { title: 'Deleted', description: row.name },
				errorTitle: 'Delete failed'
			});
			await loadTeams();
			await loadRosters();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...tmDeleteBusy };
			delete nx[row.id];
			tmDeleteBusy = nx;
		}
	}

	function openRsEdit(row: RosterRow) {
		rsForm = {
			id: row.id,
			is_owner: row.is_owner,
			is_manager: row.is_manager,
			joined_at: toDateInputValue(row.joined_at),
			left_at: row.left_at ? toDateInputValue(row.left_at) : ''
		};
		rsDialogOpen = true;
	}

	async function saveRoster() {
		const f = rsForm;
		try {
			rsFormBusy = true;
			await toastPromise(
				pb.collection('rosters').update(f.id, {
					is_owner: f.is_owner,
					is_manager: f.is_manager,
					joined_at: fromDateInputValue(f.joined_at),
					left_at: f.left_at ? fromDateInputValue(f.left_at) : ''
				}),
				{
					loading: { title: 'Saving roster' },
					success: { title: 'Saved' },
					errorTitle: 'Save failed'
				}
			);
			rsDialogOpen = false;
			await loadRosters();
		} catch {
			// toast already shown
		} finally {
			rsFormBusy = false;
		}
	}

	async function deleteRoster(row: RosterRow) {
		const teamName = row.expand?.team?.name ?? row.team;
		const tag = row.expand?.gamertag?.tag ?? row.gamertag;
		const ok = await confirmToast({
			title: 'Delete roster row',
			description: `Permanently remove ${tag} from ${teamName}? Marking left_at is usually preferable — that preserves history.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		rsDeleteBusy = { ...rsDeleteBusy, [row.id]: true };
		try {
			await toastPromise(pb.collection('rosters').delete(row.id), {
				loading: { title: 'Deleting' },
				success: { title: 'Deleted' },
				errorTitle: 'Delete failed'
			});
			await loadRosters();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...rsDeleteBusy };
			delete nx[row.id];
			rsDeleteBusy = nx;
		}
	}

	function toDateInputValue(pbDate: string): string {
		if (!pbDate) return '';
		return pbDate.slice(0, 10);
	}
	function fromDateInputValue(htmlDate: string): string {
		if (!htmlDate) return '';
		return `${htmlDate} 00:00:00.000Z`;
	}

	const tmFiltered = $derived.by<TeamRow[]>(() => {
		const q = tmFilter.trim().toLowerCase();
		const filtered = !q
			? tmRows
			: tmRows.filter(
					(r) =>
						r.name.toLowerCase().includes(q) ||
						r.slug.toLowerCase().includes(q) ||
						r.expand?.created_by?.username?.toLowerCase().includes(q)
				);
		const s = tmSort;
		if (!s) return filtered;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...filtered].sort((a, b) => {
			const av = String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv = String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	const rsFiltered = $derived.by<RosterRow[]>(() => {
		const q = rsFilter.trim().toLowerCase();
		const filtered = !q
			? rsRows
			: rsRows.filter(
					(r) =>
						r.expand?.team?.name?.toLowerCase().includes(q) ||
						r.expand?.gamertag?.tag?.toLowerCase().includes(q) ||
						r.expand?.gamertag?.expand?.user?.username?.toLowerCase().includes(q)
				);
		const s = rsSort;
		if (!s) return filtered;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...filtered].sort((a, b) => {
			const av =
				s.key === 'team'
					? (a.expand?.team?.name ?? '')
					: s.key === 'gamertag'
						? (a.expand?.gamertag?.tag ?? '')
						: String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv =
				s.key === 'team'
					? (b.expand?.team?.name ?? '')
					: s.key === 'gamertag'
						? (b.expand?.gamertag?.tag ?? '')
						: String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage rosters.' });
			return;
		}
		void loadTeams();
		void loadRosters();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Rosters"
		description="Moderate team metadata and roster history across all users."
	/>

	<Tabs value={activeTab} onValueChange={(e) => (activeTab = e.value)}>
		<Tabs.List>
			<Tabs.Trigger value="teams">
				<UsersIcon class="size-4" />
				<span>Teams</span>
			</Tabs.Trigger>
			<Tabs.Trigger value="rosters">
				<UserIcon class="size-4" />
				<span>Rosters</span>
			</Tabs.Trigger>
			<Tabs.Indicator />
		</Tabs.List>

		<Tabs.Content value="teams">
			<div class="flex flex-col gap-4 pt-4">
				<div class="flex items-center justify-between gap-2">
					<div class="input-group flex-1 grid-cols-[auto_1fr]">
						<div class="ig-cell preset-tonal">
							<SearchIcon class="size-4" />
						</div>
						<input
							type="search"
							class="ig-input"
							placeholder="Filter by name, slug, or creator"
							bind:value={tmFilter}
						/>
					</div>
					<button
						class="btn preset-tonal"
						onclick={() => loadTeams()}
						disabled={tmLoading}
						aria-label="Refresh"
					>
						{#if tmLoading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
								class="size-4"
							/>{/if}
						<span>Refresh</span>
					</button>
				</div>

				{#snippet tmNameCell({ row }: { row: TeamRow })}
					<div class="flex flex-col">
						<span class="font-semibold">{row.name}</span>
						<span class="text-xs opacity-50">{row.slug}</span>
					</div>
				{/snippet}
				{#snippet tmCreatorCell({ row }: { row: TeamRow })}
					<span>{row.expand?.created_by?.username ?? row.created_by}</span>
				{/snippet}
				{#snippet tmActionsCell({ row }: { row: TeamRow })}
					<div
						role="presentation"
						class="inline-flex items-center justify-end gap-1"
						onclick={(e) => e.stopPropagation()}
						onkeydown={(e) => e.stopPropagation()}
					>
						<button
							class="btn-icon preset-tonal btn-sm"
							title="Edit"
							onclick={() => openTmEdit(row)}
						>
							<PencilIcon class="size-4" />
						</button>
						<button
							class="btn-icon preset-tonal-error btn-sm"
							title="Delete"
							onclick={() => deleteTeam(row)}
							disabled={!!tmDeleteBusy[row.id]}
						>
							{#if tmDeleteBusy[row.id]}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<Trash2Icon class="size-4" />
							{/if}
						</button>
					</div>
				{/snippet}

				<Card size="flush" class="overflow-x-auto">
					<DataTable
						rows={tmFiltered}
						groups={[
							{
								columns: [
									{ key: 'name', label: 'Team', cell: tmNameCell },
									{ key: 'created_by', label: 'Created by', cell: tmCreatorCell },
									{ key: 'created', label: 'Created' },
									{
										key: 'actions',
										label: '',
										cell: tmActionsCell,
										sortable: false,
										align: 'right'
									}
								]
							}
						] satisfies DataColumnGroup<TeamRow>[]}
						rowKey={(r) => r.id}
						density="comfortable"
						sort={tmSort}
						onSortChange={(s) => (tmSort = s)}
						secondarySort={{ key: 'name', dir: 'asc' }}
						loading={tmLoading && tmFiltered.length === 0}
						emptyMessage={tmFilter ? 'No matches.' : 'No teams yet.'}
					/>
				</Card>
			</div>
		</Tabs.Content>

		<Tabs.Content value="rosters">
			<div class="flex flex-col gap-4 pt-4">
				<div class="flex items-center justify-between gap-2">
					<div class="input-group flex-1 grid-cols-[auto_1fr]">
						<div class="ig-cell preset-tonal">
							<SearchIcon class="size-4" />
						</div>
						<input
							type="search"
							class="ig-input"
							placeholder="Filter by team, gamertag, or user"
							bind:value={rsFilter}
						/>
					</div>
					<button
						class="btn preset-tonal"
						onclick={() => loadRosters()}
						disabled={rsLoading}
						aria-label="Refresh"
					>
						{#if rsLoading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
								class="size-4"
							/>{/if}
						<span>Refresh</span>
					</button>
				</div>

				{#snippet rsTeamCell({ row }: { row: RosterRow })}
					<span>{row.expand?.team?.name ?? '—'}</span>
				{/snippet}
				{#snippet rsGamertagCell({ row }: { row: RosterRow })}
					<div class="flex flex-col">
						<span class="font-mono">{row.expand?.gamertag?.tag ?? '—'}</span>
						<span class="text-xs opacity-50">
							{row.expand?.gamertag?.expand?.user?.username ?? ''}
						</span>
					</div>
				{/snippet}
				{#snippet rsRoleCell({ row }: { row: RosterRow })}
					<div class="flex gap-1">
						{#if row.is_owner}<span class="badge preset-tonal-warning text-xs">O</span>{/if}
						{#if row.is_manager}<span class="badge preset-tonal-primary text-xs">M</span>{/if}
						{#if !row.is_owner && !row.is_manager}<span class="text-xs opacity-50">—</span>{/if}
					</div>
				{/snippet}
				{#snippet rsDatesCell({ row }: { row: RosterRow })}
					<div class="flex flex-col text-xs">
						<span>joined {row.joined_at.slice(0, 10)}</span>
						{#if row.left_at}<span class="opacity-70">left {row.left_at.slice(0, 10)}</span
							>{:else}<span class="text-success-500">active</span>{/if}
					</div>
				{/snippet}
				{#snippet rsActionsCell({ row }: { row: RosterRow })}
					<div
						role="presentation"
						class="inline-flex items-center justify-end gap-1"
						onclick={(e) => e.stopPropagation()}
						onkeydown={(e) => e.stopPropagation()}
					>
						<button
							class="btn-icon preset-tonal btn-sm"
							title="Edit"
							onclick={() => openRsEdit(row)}
						>
							<PencilIcon class="size-4" />
						</button>
						<button
							class="btn-icon preset-tonal-error btn-sm"
							title="Delete"
							onclick={() => deleteRoster(row)}
							disabled={!!rsDeleteBusy[row.id]}
						>
							{#if rsDeleteBusy[row.id]}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<Trash2Icon class="size-4" />
							{/if}
						</button>
					</div>
				{/snippet}

				<Card size="flush" class="overflow-x-auto">
					<DataTable
						rows={rsFiltered}
						groups={[
							{
								columns: [
									{ key: 'team', label: 'Team', cell: rsTeamCell },
									{ key: 'gamertag', label: 'Gamertag', cell: rsGamertagCell },
									{ key: 'role', label: 'Role', cell: rsRoleCell, sortable: false },
									{ key: 'dates', label: 'Dates', cell: rsDatesCell, sortable: false },
									{
										key: 'actions',
										label: '',
										cell: rsActionsCell,
										sortable: false,
										align: 'right'
									}
								]
							}
						] satisfies DataColumnGroup<RosterRow>[]}
						rowKey={(r) => r.id}
						density="comfortable"
						sort={rsSort}
						onSortChange={(s) => (rsSort = s)}
						secondarySort={{ key: 'team', dir: 'asc' }}
						loading={rsLoading && rsFiltered.length === 0}
						emptyMessage={rsFilter ? 'No matches.' : 'No rosters yet.'}
					/>
				</Card>
			</div>
		</Tabs.Content>
	</Tabs>
</div>

<Dialog
	open={tmDialogOpen}
	onClose={() => {
		if (!tmFormBusy) tmDialogOpen = false;
	}}
	title="Edit team"
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void saveTeam();
		}}
		class="flex flex-col gap-3"
	>
		<label class="label">
			<span class="label-text">Name</span>
			<input
				type="text"
				class="input"
				bind:value={tmForm.name}
				maxlength="60"
				disabled={tmFormBusy}
			/>
		</label>
		<label class="label">
			<span class="label-text">Slug</span>
			<input
				type="text"
				class="input"
				bind:value={tmForm.slug}
				maxlength="60"
				disabled={tmFormBusy}
			/>
		</label>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (tmDialogOpen = false)}
				disabled={tmFormBusy}>Cancel</button
			>
			<button type="submit" class="btn preset-filled" disabled={tmFormBusy}>
				{#if tmFormBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Save</span>
			</button>
		</div>
	</form>
</Dialog>

<Dialog
	open={rsDialogOpen}
	onClose={() => {
		if (!rsFormBusy) rsDialogOpen = false;
	}}
	title="Edit roster"
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void saveRoster();
		}}
		class="flex flex-col gap-3"
	>
		<div class="flex gap-4">
			<label class="flex items-center gap-2">
				<input
					type="checkbox"
					class="checkbox"
					bind:checked={rsForm.is_owner}
					disabled={rsFormBusy}
				/>
				<span class="text-sm">Owner</span>
			</label>
			<label class="flex items-center gap-2">
				<input
					type="checkbox"
					class="checkbox"
					bind:checked={rsForm.is_manager}
					disabled={rsFormBusy}
				/>
				<span class="text-sm">Manager</span>
			</label>
		</div>
		<label class="label">
			<span class="label-text">Joined</span>
			<input type="date" class="input" bind:value={rsForm.joined_at} disabled={rsFormBusy} />
		</label>
		<label class="label">
			<span class="label-text">Left (blank = active)</span>
			<input type="date" class="input" bind:value={rsForm.left_at} disabled={rsFormBusy} />
		</label>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (rsDialogOpen = false)}
				disabled={rsFormBusy}>Cancel</button
			>
			<button type="submit" class="btn preset-filled" disabled={rsFormBusy}>
				{#if rsFormBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Save</span>
			</button>
		</div>
	</form>
</Dialog>
