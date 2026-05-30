<script lang="ts">
	// Players (M7f, M22b, M22c). Admin moderation surface for user gamertags.
	// Was part of the monolithic /admin/identity/ page; split out so
	// admins can land directly on the page they need. The
	// /admin/+layout.ts guard enforces isAdmin; this page assumes it.
	//
	// M22b — `blocked: bool` replaced with `status` 4-state enum (approved /
	// allowed / pending / blocked). The block toggle in row actions flips
	// between `blocked` and `allowed`; status transitions are audited
	// through internal/audit by the gamertags_status_transitions hook.
	//
	// M22c — tabbed queue view. Three sub-lists:
	//   - Pending: status=pending (M22e reserved-name pre-list / community
	//     report will populate this; empty until then).
	//   - Needs double-check: status=allowed — every new submission lands
	//     here pre-approval, plus auto-downgrades from approved (owner
	//     edited tag of an approved row).
	//   - All: every row regardless of status.
	// Approve action (single-row) writes status="approved" + an
	// ActionApprove audit row via the hook. Bulk multi-select is deferred.
	//
	// User-account moderation (ban/timeout, isAdmin toggles, soft-delete
	// review) is deliberately deferred — that surface lands in M8 when the
	// roles + permissions schema arrives. For now this page is gamertag-only.

	import { onMount } from 'svelte';
	import {
		CheckIcon,
		LoaderIcon,
		PencilIcon,
		RefreshCwIcon,
		SearchIcon,
		ShieldAlertIcon,
		ShieldOffIcon,
		Trash2Icon
	} from '@lucide/svelte';
	import { Tabs } from '@skeletonlabs/skeleton-svelte';
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
		sanitized: string;
		status: string;
		created: string;
		updated: string;
		expand?: { user?: UserExpand };
	}

	type QueueTab = 'pending' | 'queue' | 'all';

	let gtRows = $state<GamertagRow[]>([]);
	let gtLoading = $state(true);
	let gtFilter = $state('');
	let gtSort = $state<SortState>({ key: 'tag', dir: 'asc' });
	let activeTab = $state<QueueTab>('pending');
	let gtDialogOpen = $state(false);
	let gtForm = $state({ id: '', tag: '' });
	let gtFormBusy = $state(false);
	let gtBlockBusy = $state<Record<string, boolean>>({});
	let gtApproveBusy = $state<Record<string, boolean>>({});
	let gtDeleteBusy = $state<Record<string, boolean>>({});

	async function loadGamertags() {
		try {
			gtLoading = true;
			gtRows = await pb
				.collection('gamertags')
				.getFullList<GamertagRow>({ expand: 'user', sort: 'tag' });
		} catch (err) {
			toaster.error({ title: 'Load gamertags failed', description: describeAsyncError(err) });
		} finally {
			gtLoading = false;
		}
	}

	// Block/unblock flips between exactly two states (blocked ↔ allowed). Any
	// other transition uses the explicit approve action or the M22e pending
	// flag (deferred).
	async function toggleBlock(row: GamertagRow) {
		gtBlockBusy = { ...gtBlockBusy, [row.id]: true };
		const wasBlocked = row.status === 'blocked';
		const nextStatus = wasBlocked ? 'allowed' : 'blocked';
		try {
			await toastPromise(pb.collection('gamertags').update(row.id, { status: nextStatus }), {
				loading: { title: wasBlocked ? 'Unblocking' : 'Blocking', description: row.tag },
				success: { title: wasBlocked ? 'Unblocked' : 'Blocked', description: row.tag },
				errorTitle: 'Update failed'
			});
			await loadGamertags();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...gtBlockBusy };
			delete nx[row.id];
			gtBlockBusy = nx;
		}
	}

	// Approve promotes the row to status="approved". The backend hook emits
	// an ActionApprove audit row with the requesting admin as actor.
	async function approveGamertag(row: GamertagRow) {
		gtApproveBusy = { ...gtApproveBusy, [row.id]: true };
		try {
			await toastPromise(pb.collection('gamertags').update(row.id, { status: 'approved' }), {
				loading: { title: 'Approving', description: row.tag },
				success: { title: 'Approved', description: row.tag },
				errorTitle: 'Approve failed'
			});
			await loadGamertags();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...gtApproveBusy };
			delete nx[row.id];
			gtApproveBusy = nx;
		}
	}

	function openGtEdit(row: GamertagRow) {
		gtForm = { id: row.id, tag: row.tag };
		gtDialogOpen = true;
	}

	// Edit dialog is tag-only: status transitions go through the row buttons.
	// Writing { tag } as a PATCH leaves status untouched, so editing an
	// approved tag here triggers the auto-downgrade hook on the backend
	// (status → "allowed" + ActionEdit audit row).
	async function saveGamertag() {
		const f = gtForm;
		if (!f.tag.trim()) {
			toaster.error({ title: 'Invalid', description: 'Tag is required.' });
			return;
		}
		try {
			gtFormBusy = true;
			await toastPromise(pb.collection('gamertags').update(f.id, { tag: f.tag.trim() }), {
				loading: { title: 'Saving', description: f.tag },
				success: { title: 'Saved', description: f.tag },
				errorTitle: 'Save failed'
			});
			gtDialogOpen = false;
			await loadGamertags();
		} catch {
			// toast already shown
		} finally {
			gtFormBusy = false;
		}
	}

	async function deleteGamertag(row: GamertagRow) {
		const ok = await confirmToast({
			title: 'Delete gamertag',
			description: `Permanently remove "${row.tag}"? Owner: ${row.expand?.user?.username ?? row.user}. This also clears users.default_gamertag if it was set to this row.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		gtDeleteBusy = { ...gtDeleteBusy, [row.id]: true };
		try {
			await toastPromise(pb.collection('gamertags').delete(row.id), {
				loading: { title: 'Deleting', description: row.tag },
				success: { title: 'Deleted', description: row.tag },
				errorTitle: 'Delete failed'
			});
			await loadGamertags();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...gtDeleteBusy };
			delete nx[row.id];
			gtDeleteBusy = nx;
		}
	}

	// Per-tab counts run on the unfiltered set so admins see total backlog
	// regardless of what's typed in the search box.
	const pendingCount = $derived(gtRows.filter((r) => r.status === 'pending').length);
	const queueCount = $derived(gtRows.filter((r) => r.status === 'allowed').length);
	const allCount = $derived(gtRows.length);

	const gtVisible = $derived.by<GamertagRow[]>(() => {
		let base = gtRows;
		if (activeTab === 'pending') base = base.filter((r) => r.status === 'pending');
		else if (activeTab === 'queue') base = base.filter((r) => r.status === 'allowed');

		const q = gtFilter.trim().toLowerCase();
		const filtered = !q
			? base
			: base.filter(
					(r) =>
						r.tag.toLowerCase().includes(q) ||
						r.sanitized?.includes(q) ||
						r.expand?.user?.username?.toLowerCase().includes(q) ||
						r.expand?.user?.email?.toLowerCase().includes(q)
				);
		const s = gtSort;
		if (!s) return filtered;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...filtered].sort((a, b) => {
			const av =
				s.key === 'user'
					? (a.expand?.user?.username ?? '')
					: String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv =
				s.key === 'user'
					? (b.expand?.user?.username ?? '')
					: String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	const emptyMessage = $derived.by(() => {
		if (gtFilter) return 'No matches.';
		if (activeTab === 'pending') return 'No pending review.';
		if (activeTab === 'queue') return 'No tags waiting for double-check.';
		return 'No gamertags yet.';
	});

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage players.' });
			return;
		}
		void loadGamertags();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Players"
		description="Moderate user gamertags through the review queue. Account-level controls (admin/ban/timeout) move here in M8."
	/>

	<Tabs value={activeTab} onValueChange={(e) => (activeTab = e.value as QueueTab)}>
		<Tabs.List>
			<Tabs.Trigger value="pending">
				<span>Pending</span>
				<span class="badge preset-tonal-warning text-xs">{pendingCount}</span>
			</Tabs.Trigger>
			<Tabs.Trigger value="queue">
				<span>Needs double-check</span>
				<span class="badge preset-tonal text-xs">{queueCount}</span>
			</Tabs.Trigger>
			<Tabs.Trigger value="all">
				<span>All</span>
				<span class="badge preset-tonal text-xs">{allCount}</span>
			</Tabs.Trigger>
			<Tabs.Indicator />
		</Tabs.List>
	</Tabs>

	<div class="flex items-center justify-between gap-2">
		<div class="input-group flex-1 grid-cols-[auto_1fr]">
			<div class="ig-cell preset-tonal">
				<SearchIcon class="size-4" />
			</div>
			<input
				type="search"
				class="ig-input"
				placeholder="Filter by tag, username, or email"
				bind:value={gtFilter}
			/>
		</div>
		<button
			class="btn preset-tonal"
			onclick={() => loadGamertags()}
			disabled={gtLoading}
			aria-label="Refresh"
		>
			{#if gtLoading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
					class="size-4"
				/>{/if}
			<span>Refresh</span>
		</button>
	</div>

	{#snippet gtUserCell({ row }: { row: GamertagRow })}
		<div class="flex flex-col">
			<span class="font-semibold">{row.expand?.user?.username ?? '—'}</span>
			<span class="text-xs opacity-50">{row.expand?.user?.email ?? row.user}</span>
		</div>
	{/snippet}
	{#snippet gtTagCell({ row }: { row: GamertagRow })}
		<span class="font-mono">{row.tag}</span>
	{/snippet}
	{#snippet gtStatusCell({ row }: { row: GamertagRow })}
		{#if row.status === 'blocked'}
			<span class="badge preset-tonal-error">
				<ShieldAlertIcon class="size-3" />
				Blocked
			</span>
		{:else if row.status === 'pending'}
			<span class="badge preset-tonal-warning">Pending</span>
		{:else if row.status === 'approved'}
			<span class="badge preset-tonal-success">Approved</span>
		{:else}
			<span class="text-xs opacity-50">Allowed</span>
		{/if}
	{/snippet}
	{#snippet gtActionsCell({ row }: { row: GamertagRow })}
		<div
			role="presentation"
			class="inline-flex items-center justify-end gap-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			{#if row.status !== 'approved'}
				<button
					class="btn-icon preset-tonal-success btn-sm"
					title="Approve"
					onclick={() => approveGamertag(row)}
					disabled={!!gtApproveBusy[row.id]}
				>
					{#if gtApproveBusy[row.id]}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<CheckIcon class="size-4" />
					{/if}
				</button>
			{/if}
			<button
				class="btn-icon preset-tonal btn-sm"
				title={row.status === 'blocked' ? 'Unblock' : 'Block'}
				onclick={() => toggleBlock(row)}
				disabled={!!gtBlockBusy[row.id]}
			>
				{#if gtBlockBusy[row.id]}
					<LoaderIcon class="size-4 animate-spin" />
				{:else if row.status === 'blocked'}
					<ShieldOffIcon class="size-4" />
				{:else}
					<ShieldAlertIcon class="size-4" />
				{/if}
			</button>
			<button class="btn-icon preset-tonal btn-sm" title="Edit" onclick={() => openGtEdit(row)}>
				<PencilIcon class="size-4" />
			</button>
			<button
				class="btn-icon preset-tonal-error btn-sm"
				title="Delete"
				onclick={() => deleteGamertag(row)}
				disabled={!!gtDeleteBusy[row.id]}
			>
				{#if gtDeleteBusy[row.id]}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<Trash2Icon class="size-4" />
				{/if}
			</button>
		</div>
	{/snippet}

	<Card size="flush" class="overflow-x-auto">
		<DataTable
			rows={gtVisible}
			groups={[
				{
					columns: [
						{ key: 'user', label: 'User', cell: gtUserCell },
						{ key: 'tag', label: 'Tag', cell: gtTagCell },
						{ key: 'status', label: 'Status', cell: gtStatusCell },
						{ key: 'actions', label: '', cell: gtActionsCell, sortable: false, align: 'right' }
					]
				}
			] satisfies DataColumnGroup<GamertagRow>[]}
			rowKey={(r) => r.id}
			density="comfortable"
			sort={gtSort}
			onSortChange={(s) => (gtSort = s)}
			secondarySort={{ key: 'tag', dir: 'asc' }}
			loading={gtLoading && gtVisible.length === 0}
			{emptyMessage}
		/>
	</Card>
</div>

<Dialog
	open={gtDialogOpen}
	onClose={() => {
		if (!gtFormBusy) gtDialogOpen = false;
	}}
	title="Edit gamertag"
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void saveGamertag();
		}}
		class="flex flex-col gap-3"
	>
		<label class="label">
			<span class="label-text">Tag</span>
			<input
				type="text"
				class="input"
				bind:value={gtForm.tag}
				maxlength="12"
				disabled={gtFormBusy}
			/>
		</label>
		<p class="text-xs opacity-60">
			Use the shield button in the row actions to block or unblock, or the check button to approve.
			Editing an approved tag automatically flips its status back to "allowed" for re-check.
		</p>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (gtDialogOpen = false)}
				disabled={gtFormBusy}>Cancel</button
			>
			<button type="submit" class="btn preset-filled" disabled={gtFormBusy}>
				{#if gtFormBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Save</span>
			</button>
		</div>
	</form>
</Dialog>
