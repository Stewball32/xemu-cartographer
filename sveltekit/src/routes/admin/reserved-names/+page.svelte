<script lang="ts">
	// Reserved names (M22e). Admin-curated pre-list of substring patterns
	// that auto-flag matching gamertags + team names as status="pending" on
	// create or update. Match is substring-on-lowercase: pattern "ass"
	// matches "Sassy" and "BASS". Per the M22 doc, matches flag for review
	// rather than auto-blocking so false positives still reach admin eyes.
	//
	// /admin/+layout.ts enforces isAdmin.

	import { onMount } from 'svelte';
	import {
		BookmarkXIcon,
		LoaderIcon,
		PlusIcon,
		RefreshCwIcon,
		SearchIcon,
		Trash2Icon
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
	}
	interface ReservedRow {
		id: string;
		pattern: string;
		description: string;
		created_by: string;
		created: string;
		updated: string;
		expand?: { created_by?: UserExpand };
	}

	let rows = $state<ReservedRow[]>([]);
	let loading = $state(true);
	let filter = $state('');
	let sort = $state<SortState>({ key: 'pattern', dir: 'asc' });
	let dialogOpen = $state(false);
	let form = $state({ pattern: '', description: '' });
	let formBusy = $state(false);
	let deleteBusy = $state<Record<string, boolean>>({});

	async function load() {
		try {
			loading = true;
			rows = await pb
				.collection('reserved_names')
				.getFullList<ReservedRow>({ expand: 'created_by', sort: 'pattern' });
		} catch (err) {
			toaster.error({ title: 'Load reserved names failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function openAdd() {
		form = { pattern: '', description: '' };
		dialogOpen = true;
	}

	async function save() {
		const f = form;
		const pattern = f.pattern.trim();
		if (!pattern) {
			toaster.error({ title: 'Invalid', description: 'Pattern is required.' });
			return;
		}
		if (!auth.user) return;
		try {
			formBusy = true;
			await toastPromise(
				pb.collection('reserved_names').create({
					pattern,
					description: f.description.trim(),
					created_by: auth.user.id
				}),
				{
					loading: { title: 'Adding', description: pattern },
					success: { title: 'Added', description: pattern },
					errorTitle: 'Add failed',
					errorDescription: (err) => {
						const m = err instanceof Error ? err.message : 'Failed';
						return m.toLowerCase().includes('unique') ? 'Pattern already exists.' : m;
					}
				}
			);
			dialogOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			formBusy = false;
		}
	}

	async function deletePattern(row: ReservedRow) {
		const ok = await confirmToast({
			title: 'Delete pattern',
			description: `Remove "${row.pattern}"? Future gamertags and team names containing this substring will no longer be auto-flagged as pending. Existing pending rows are not affected.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		deleteBusy = { ...deleteBusy, [row.id]: true };
		try {
			await toastPromise(pb.collection('reserved_names').delete(row.id), {
				loading: { title: 'Deleting', description: row.pattern },
				success: { title: 'Deleted', description: row.pattern },
				errorTitle: 'Delete failed'
			});
			await load();
		} catch {
			// toast already shown
		} finally {
			const nx = { ...deleteBusy };
			delete nx[row.id];
			deleteBusy = nx;
		}
	}

	const filtered = $derived.by<ReservedRow[]>(() => {
		const q = filter.trim().toLowerCase();
		const base = !q
			? rows
			: rows.filter(
					(r) =>
						r.pattern.toLowerCase().includes(q) ||
						r.description?.toLowerCase().includes(q) ||
						r.expand?.created_by?.username?.toLowerCase().includes(q)
				);
		const s = sort;
		if (!s) return base;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...base].sort((a, b) => {
			const av =
				s.key === 'created_by'
					? (a.expand?.created_by?.username ?? '')
					: String((a as unknown as Record<string, unknown>)[s.key] ?? '');
			const bv =
				s.key === 'created_by'
					? (b.expand?.created_by?.username ?? '')
					: String((b as unknown as Record<string, unknown>)[s.key] ?? '');
			return av.localeCompare(bv) * dir;
		});
	});

	onMount(() => {
		if (!auth.token) {
			toaster.error({
				title: 'Not authenticated',
				description: 'Log in to manage reserved names.'
			});
			return;
		}
		void load();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Reserved names"
		description="Substring patterns that auto-flag matching gamertags and team names as pending review. Matches use lowercase contains (e.g. pattern 'ass' matches 'Sassy'), so false positives are expected — pending lands in the review queue, not the block list."
	/>

	<div class="flex items-center justify-between gap-2">
		<div class="field-group flex-1 grid-cols-[auto_1fr]">
			<div class="flex items-center justify-center preset-tonal px-3">
				<SearchIcon class="size-4" />
			</div>
			<input
				type="search"
				class="input"
				placeholder="Filter by pattern, description, or creator"
				bind:value={filter}
			/>
		</div>
		<button class="btn preset-tonal" onclick={() => load()} disabled={loading} aria-label="Refresh">
			{#if loading}<LoaderIcon class="size-4 animate-spin" />{:else}<RefreshCwIcon
					class="size-4"
				/>{/if}
			<span>Refresh</span>
		</button>
		<button class="btn preset-filled" onclick={() => openAdd()}>
			<PlusIcon class="size-4" />
			<span>Add pattern</span>
		</button>
	</div>

	{#snippet patternCell({ row }: { row: ReservedRow })}
		<span class="badge preset-tonal-warning">
			<BookmarkXIcon class="size-3" />
			<span class="font-mono">{row.pattern}</span>
		</span>
	{/snippet}
	{#snippet descriptionCell({ row }: { row: ReservedRow })}
		{#if row.description}
			<span>{row.description}</span>
		{:else}
			<span class="text-xs opacity-50">—</span>
		{/if}
	{/snippet}
	{#snippet creatorCell({ row }: { row: ReservedRow })}
		<span>{row.expand?.created_by?.username ?? row.created_by ?? '—'}</span>
	{/snippet}
	{#snippet actionsCell({ row }: { row: ReservedRow })}
		<div
			role="presentation"
			class="inline-flex items-center justify-end gap-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<button
				class="btn-icon preset-tonal-error btn-sm"
				title="Delete"
				onclick={() => deletePattern(row)}
				disabled={!!deleteBusy[row.id]}
			>
				{#if deleteBusy[row.id]}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<Trash2Icon class="size-4" />
				{/if}
			</button>
		</div>
	{/snippet}

	<Card size="flush" class="overflow-x-auto">
		<DataTable
			rows={filtered}
			groups={[
				{
					columns: [
						{ key: 'pattern', label: 'Pattern', cell: patternCell },
						{ key: 'description', label: 'Description', cell: descriptionCell },
						{ key: 'created_by', label: 'Added by', cell: creatorCell },
						{ key: 'created', label: 'Added' },
						{ key: 'actions', label: '', cell: actionsCell, sortable: false, align: 'right' }
					]
				}
			] satisfies DataColumnGroup<ReservedRow>[]}
			rowKey={(r) => r.id}
			density="comfortable"
			{sort}
			onSortChange={(s) => (sort = s)}
			secondarySort={{ key: 'pattern', dir: 'asc' }}
			loading={loading && filtered.length === 0}
			emptyMessage={filter ? 'No matches.' : 'No reserved patterns yet.'}
		/>
	</Card>
</div>

<Dialog
	open={dialogOpen}
	onClose={() => {
		if (!formBusy) dialogOpen = false;
	}}
	title="Add reserved name"
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void save();
		}}
		class="flex flex-col gap-3"
	>
		<label class="label">
			<span class="label-text">Pattern</span>
			<input
				type="text"
				class="input font-mono"
				bind:value={form.pattern}
				maxlength="64"
				placeholder="e.g. ass"
				disabled={formBusy}
			/>
		</label>
		<label class="label">
			<span class="label-text">Description (optional)</span>
			<input
				type="text"
				class="input"
				bind:value={form.description}
				maxlength="300"
				placeholder="why this pattern is flagged"
				disabled={formBusy}
			/>
		</label>
		<p class="text-xs opacity-60">
			Matches use lowercase contains across gamertags and team names. Adding "ass" will flag "Sassy"
			and "Bass" as pending review — false positives are expected; the admin queue is where they get
			cleared.
		</p>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (dialogOpen = false)}
				disabled={formBusy}>Cancel</button
			>
			<button type="submit" class="btn preset-filled" disabled={formBusy}>
				{#if formBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>Add</span>
			</button>
		</div>
	</form>
</Dialog>
