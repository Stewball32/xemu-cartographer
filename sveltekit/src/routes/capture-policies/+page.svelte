<script lang="ts">
	// Capture policies admin UI — list, create, edit, delete rows in the
	// capture_policies collection. Backend rules admit users with
	// isAdmin=true (plus PB superusers via the standard bypass).
	//
	// The server-side hot-reload hook from PR 16 picks up every change
	// here within one tick, so edits propagate to runners without a
	// server restart.

	import { onMount } from 'svelte';
	import {
		LoaderIcon,
		PencilIcon,
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

	type Mode = 'auto' | 'always' | 'never';
	type Cadence = 'default' | 'engine' | '30hz' | '10hz' | '5hz' | '250ms' | '500ms' | '1s';

	interface Policy {
		id: string;
		instance: string;
		class: string;
		mode: Mode;
		cadence: Cadence;
		sink: string;
		description: string;
		created: string;
		updated: string;
	}

	type FormState = {
		id: string | null; // null = create
		instance: string;
		class: string;
		mode: Mode;
		cadence: Cadence;
		sink: string;
		description: string;
	};

	const MODES: Mode[] = ['auto', 'always', 'never'];
	const CADENCES: Cadence[] = ['default', 'engine', '30hz', '10hz', '5hz', '250ms', '500ms', '1s'];

	// Canonical v2 classes a policy can target. "*" wildcards apply across
	// every class — keep at the top of the picker.
	const CLASSES = [
		'*',
		'xbox',
		'scenario',
		'game',
		'tick',
		'objects',
		'debug',
		'event',
		'summary',
		'previous_game'
	] as const;

	let rows = $state<Policy[]>([]);
	let loading = $state(true);
	let filter = $state('');
	let sortState = $state<SortState>({ key: 'instance', dir: 'asc' });

	let dialogOpen = $state(false);
	let form = $state<FormState>(emptyForm());
	let formBusy = $state(false);

	let deleteBusy = $state<Record<string, boolean>>({});

	function emptyForm(): FormState {
		return {
			id: null,
			instance: '*',
			class: 'tick',
			mode: 'auto',
			cadence: 'default',
			sink: '',
			description: ''
		};
	}

	async function load() {
		try {
			loading = true;
			const list = await pb
				.collection('capture_policies')
				.getFullList<Policy>({ sort: 'instance,class' });
			rows = list;
		} catch (err) {
			toaster.error({ title: 'Load failed', description: describeAsyncError(err) });
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		form = emptyForm();
		dialogOpen = true;
	}

	function openEdit(p: Policy) {
		form = {
			id: p.id,
			instance: p.instance,
			class: p.class,
			mode: p.mode,
			cadence: p.cadence,
			sink: p.sink ?? '',
			description: p.description ?? ''
		};
		dialogOpen = true;
	}

	async function handleSubmit() {
		const f = form;
		if (!f.instance.trim() || !f.class.trim()) {
			toaster.error({
				title: 'Invalid',
				description: 'Instance and class are required (use "*" for wildcard).'
			});
			return;
		}
		const payload = {
			instance: f.instance.trim(),
			class: f.class.trim(),
			mode: f.mode,
			cadence: f.cadence,
			sink: f.sink.trim(),
			description: f.description.trim()
		};
		try {
			formBusy = true;
			if (f.id) {
				await toastPromise(pb.collection('capture_policies').update(f.id, payload), {
					loading: { title: 'Saving', description: `${payload.instance} / ${payload.class}` },
					success: {
						title: 'Saved',
						description: `${payload.instance} / ${payload.class}`
					},
					errorTitle: 'Save failed'
				});
			} else {
				await toastPromise(pb.collection('capture_policies').create(payload), {
					loading: { title: 'Creating', description: `${payload.instance} / ${payload.class}` },
					success: {
						title: 'Created',
						description: `${payload.instance} / ${payload.class}`
					},
					errorTitle: 'Create failed'
				});
			}
			dialogOpen = false;
			await load();
		} catch {
			// toast already shown
		} finally {
			formBusy = false;
		}
	}

	async function handleDelete(p: Policy) {
		const ok = await confirmToast({
			title: 'Delete policy',
			description: `Remove the (${p.instance}, ${p.class}) policy? The runner-side demand reverts to WS-subscribers-only on the next reload.`,
			confirmLabel: 'Delete',
			type: 'warning'
		});
		if (!ok) return;
		deleteBusy = { ...deleteBusy, [p.id]: true };
		try {
			await toastPromise(pb.collection('capture_policies').delete(p.id), {
				loading: { title: 'Deleting', description: `${p.instance} / ${p.class}` },
				success: { title: 'Deleted', description: `${p.instance} / ${p.class}` },
				errorTitle: 'Delete failed'
			});
			await load();
		} catch {
			// toast already shown
		} finally {
			const next = { ...deleteBusy };
			delete next[p.id];
			deleteBusy = next;
		}
	}

	const filteredRows = $derived.by<Policy[]>(() => {
		const q = filter.trim().toLowerCase();
		if (!q) return rows;
		return rows.filter(
			(r) =>
				r.instance.toLowerCase().includes(q) ||
				r.class.toLowerCase().includes(q) ||
				r.mode.toLowerCase().includes(q) ||
				r.sink.toLowerCase().includes(q) ||
				r.description.toLowerCase().includes(q)
		);
	});

	const sortedFilteredRows = $derived.by<Policy[]>(() => {
		const s = sortState;
		if (!s) return filteredRows;
		const dir = s.dir === 'desc' ? -1 : 1;
		return [...filteredRows].sort((a, b) => {
			const key = s.key as keyof Policy;
			const av = String(a[key] ?? '');
			const bv = String(b[key] ?? '');
			const primary = av.localeCompare(bv) * dir;
			if (primary !== 0) return primary;
			return a.instance.localeCompare(b.instance) || a.class.localeCompare(b.class);
		});
	});

	function modeBadgeClass(mode: Mode): string {
		switch (mode) {
			case 'always':
				return 'badge preset-filled-success-500';
			case 'never':
				return 'badge preset-filled-error-500';
			case 'auto':
			default:
				return 'badge preset-tonal';
		}
	}

	onMount(() => {
		if (!auth.token) {
			toaster.error({ title: 'Not authenticated', description: 'Log in to manage policies.' });
			return;
		}
		load();
	});
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4 sm:gap-6">
	<PageHeader
		title="Capture policies"
		description="Per-(instance, class) rules controlling what the scraper reads and where it sinks the bytes. Edits take effect on the next reload — no server restart required."
	>
		{#snippet actions()}
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => load()}
				disabled={loading}
				aria-label="Refresh"
			>
				{#if loading}
					<LoaderIcon class="size-4 animate-spin" />
				{:else}
					<RefreshCwIcon class="size-4" />
				{/if}
				<span>Refresh</span>
			</button>
			<button type="button" class="btn preset-filled" onclick={openCreate}>
				<PlusIcon class="size-4" />
				<span>New</span>
			</button>
		{/snippet}
	</PageHeader>

	<div class="input-group grid-cols-[auto_1fr]">
		<div class="ig-cell preset-tonal">
			<SearchIcon class="size-4" />
		</div>
		<input
			type="search"
			class="ig-input"
			placeholder="Filter by instance, class, mode, sink, or description"
			bind:value={filter}
			aria-label="Filter policies"
		/>
	</div>

	{#snippet modeCell({ row }: { row: Policy })}
		<span class={modeBadgeClass(row.mode)}>{row.mode}</span>
	{/snippet}
	{#snippet sinkCell({ row }: { row: Policy })}
		<span class="font-mono text-xs">{row.sink || '—'}</span>
	{/snippet}
	{#snippet actionsCell({ row }: { row: Policy })}
		<div
			role="presentation"
			class="inline-flex items-center justify-end gap-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<button
				type="button"
				class="btn-icon preset-tonal btn-sm"
				aria-label="Edit"
				title="Edit"
				onclick={() => openEdit(row)}
			>
				<PencilIcon class="size-4" />
			</button>
			<button
				type="button"
				class="btn-icon preset-tonal-error btn-sm"
				aria-label="Delete"
				title="Delete"
				onclick={() => handleDelete(row)}
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
			rows={sortedFilteredRows}
			groups={[
				{
					columns: [
						{ key: 'instance', label: 'Instance' },
						{ key: 'class', label: 'Class' },
						{ key: 'mode', label: 'Mode', cell: modeCell },
						{ key: 'cadence', label: 'Cadence' },
						{ key: 'sink', label: 'Sink', cell: sinkCell },
						{ key: 'description', label: 'Description' },
						{
							key: 'actions',
							label: '',
							cell: actionsCell,
							sortable: false,
							align: 'right'
						}
					]
				}
			] satisfies DataColumnGroup<Policy>[]}
			rowKey={(r) => r.id}
			density="comfortable"
			sort={sortState}
			onSortChange={(s) => (sortState = s)}
			secondarySort={{ key: 'instance', dir: 'asc' }}
			loading={loading && sortedFilteredRows.length === 0}
			emptyMessage={filter
				? 'No matches for that filter.'
				: 'No policies yet. Create one to gate reads or persist envelopes.'}
		/>
	</Card>
</div>

<Dialog
	open={dialogOpen}
	onClose={() => {
		if (!formBusy) dialogOpen = false;
	}}
	title={form.id ? 'Edit policy' : 'New policy'}
>
	<form
		onsubmit={(e) => {
			e.preventDefault();
			handleSubmit();
		}}
		class="flex flex-col gap-3"
	>
		<div class="grid grid-cols-2 gap-3">
			<label class="label">
				<span class="label-text">Instance</span>
				<input
					type="text"
					class="input"
					bind:value={form.instance}
					placeholder="* or runner name"
					disabled={formBusy}
				/>
			</label>
			<label class="label">
				<span class="label-text">Class</span>
				<select class="select" bind:value={form.class} disabled={formBusy}>
					{#each CLASSES as cls (cls)}
						<option value={cls}>{cls}</option>
					{/each}
				</select>
			</label>
		</div>
		<div class="grid grid-cols-2 gap-3">
			<label class="label">
				<span class="label-text">Mode</span>
				<select class="select" bind:value={form.mode} disabled={formBusy}>
					{#each MODES as m (m)}
						<option value={m}>{m}</option>
					{/each}
				</select>
			</label>
			<label class="label">
				<span class="label-text">Cadence</span>
				<select class="select" bind:value={form.cadence} disabled={formBusy}>
					{#each CADENCES as c (c)}
						<option value={c}>{c}</option>
					{/each}
				</select>
			</label>
		</div>
		<label class="label">
			<span class="label-text">Sink</span>
			<input
				type="text"
				class="input"
				bind:value={form.sink}
				placeholder={'file:replays/{instance}.ndjson  ·  pb:game_events  ·  (empty = broadcast only)'}
				disabled={formBusy}
			/>
			<span class="text-xs text-surface-600-400">
				Schemes: <code>file:</code> (NDJSON append), <code>pb:</code> (PocketBase collection).
				Placeholders: <code>&#123;instance&#125;</code>.
			</span>
		</label>
		<label class="label">
			<span class="label-text">Description</span>
			<input
				type="text"
				class="input"
				bind:value={form.description}
				placeholder="optional · human label"
				disabled={formBusy}
			/>
		</label>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				class="btn preset-tonal"
				onclick={() => (dialogOpen = false)}
				disabled={formBusy}
			>
				Cancel
			</button>
			<button type="submit" class="btn preset-filled" disabled={formBusy}>
				{#if formBusy}<LoaderIcon class="size-4 animate-spin" />{/if}
				<span>{form.id ? 'Save' : 'Create'}</span>
			</button>
		</div>
	</form>
</Dialog>
