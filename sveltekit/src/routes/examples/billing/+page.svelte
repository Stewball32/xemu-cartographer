<script lang="ts">
	import { Progress } from '@skeletonlabs/skeleton-svelte';
	import {
		DownloadIcon,
		CreditCardIcon,
		ReceiptIcon,
		TrendingUpIcon,
		CheckCircle2Icon,
		ClockIcon,
		XCircleIcon
	} from '@lucide/svelte';
	import type { Component } from 'svelte';

	type Status = 'Paid' | 'Pending' | 'Failed';

	interface Invoice {
		id: string;
		date: string;
		amount: string;
		status: Status;
	}

	const statusMeta: Record<Status, { preset: string; icon: Component<{ class?: string }> }> = {
		Paid: { preset: 'preset-filled-success-500', icon: CheckCircle2Icon },
		Pending: { preset: 'preset-filled-warning-500', icon: ClockIcon },
		Failed: { preset: 'preset-filled-error-500', icon: XCircleIcon }
	};

	const invoices: Invoice[] = [
		{ id: 'INV-00124', date: 'Apr 1, 2026', amount: '$19.00', status: 'Paid' },
		{ id: 'INV-00123', date: 'Mar 1, 2026', amount: '$19.00', status: 'Paid' },
		{ id: 'INV-00122', date: 'Feb 1, 2026', amount: '$19.00', status: 'Paid' },
		{ id: 'INV-00121', date: 'Jan 1, 2026', amount: '$19.00', status: 'Paid' },
		{ id: 'INV-00120', date: 'Dec 1, 2025', amount: '$19.00', status: 'Paid' },
		{ id: 'INV-00119', date: 'Nov 1, 2025', amount: '$19.00', status: 'Failed' }
	];

	const usage = [
		{ label: 'API calls', used: 42800, limit: 100000, unit: '' },
		{ label: 'Storage', used: 6.4, limit: 10, unit: ' GB' },
		{ label: 'Bandwidth', used: 72, limit: 100, unit: ' GB' },
		{ label: 'Team seats', used: 3, limit: 5, unit: '' }
	];
</script>

<h1 class="mb-2 h2">Billing</h1>
<p class="mb-6 text-sm opacity-70">Manage your plan, payment method, and invoice history.</p>

<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
	<!-- Current Plan -->
	<div class="space-y-4 card p-6 lg:col-span-2">
		<div class="flex items-start justify-between">
			<div>
				<h2 class="h4">Pro Plan</h2>
				<p class="text-sm opacity-70">$19 / month — renews on May 1, 2026</p>
			</div>
			<span class="badge preset-filled-primary-500">Active</span>
		</div>

		<hr class="hr" />

		<div class="grid gap-6 sm:grid-cols-2">
			{#each usage as row (row.label)}
				<div class="space-y-2">
					<div class="flex items-center justify-between text-sm">
						<span class="opacity-70">{row.label}</span>
						<span class="font-semibold">
							{row.used.toLocaleString()}{row.unit} / {row.limit.toLocaleString()}{row.unit}
						</span>
					</div>
					<Progress value={(row.used / row.limit) * 100} max={100}>
						<Progress.Track class="h-2 preset-outlined-surface-200-800">
							<Progress.Range
								class={row.used / row.limit >= 0.9
									? 'bg-error-500'
									: row.used / row.limit >= 0.7
										? 'bg-warning-500'
										: 'bg-primary-500'}
							/>
						</Progress.Track>
					</Progress>
				</div>
			{/each}
		</div>

		<div class="flex flex-wrap gap-2 pt-2">
			<button class="btn preset-filled-primary-500 btn-sm">Upgrade Plan</button>
			<button class="btn preset-tonal btn-sm">Change Plan</button>
			<button class="btn preset-tonal btn-sm text-error-500">Cancel</button>
		</div>
	</div>

	<!-- Payment Method -->
	<div class="space-y-4 card p-6">
		<h2 class="h4">Payment Method</h2>
		<div class="flex items-center gap-3 card preset-outlined-surface-200-800 p-4">
			<CreditCardIcon class="size-8 text-primary-500" />
			<div class="flex-1">
				<p class="text-sm font-semibold">Visa •••• 4242</p>
				<p class="text-xs opacity-60">Expires 08/28</p>
			</div>
		</div>
		<button class="btn w-full preset-tonal btn-sm">Update Card</button>

		<hr class="hr" />

		<div class="space-y-2">
			<div class="flex justify-between text-sm">
				<span class="opacity-70">Next payment</span>
				<span class="font-semibold">May 1, 2026</span>
			</div>
			<div class="flex justify-between text-sm">
				<span class="opacity-70">Amount</span>
				<span class="font-semibold">$19.00</span>
			</div>
		</div>
	</div>
</div>

<!-- Stat cards -->
<div class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
	<div class="flex items-center gap-4 card p-6">
		<div class="flex items-center justify-center rounded-full bg-primary-500/10 p-3">
			<ReceiptIcon class="size-5 text-primary-500" />
		</div>
		<div>
			<p class="text-sm opacity-70">Lifetime spent</p>
			<p class="text-2xl font-bold">$456</p>
		</div>
	</div>
	<div class="flex items-center gap-4 card p-6">
		<div class="flex items-center justify-center rounded-full bg-success-500/10 p-3">
			<CheckCircle2Icon class="size-5 text-success-500" />
		</div>
		<div>
			<p class="text-sm opacity-70">Invoices paid</p>
			<p class="text-2xl font-bold">24</p>
		</div>
	</div>
	<div class="flex items-center gap-4 card p-6">
		<div class="flex items-center justify-center rounded-full bg-secondary-500/10 p-3">
			<TrendingUpIcon class="size-5 text-secondary-500" />
		</div>
		<div>
			<p class="text-sm opacity-70">Plan tenure</p>
			<p class="text-2xl font-bold">14 mo</p>
		</div>
	</div>
</div>

<!-- Invoices Table -->
<div class="mt-4 space-y-4 card p-6">
	<h2 class="h4">Invoice History</h2>
	<div class="table-wrap">
		<table class="table">
			<thead>
				<tr>
					<th>Invoice</th>
					<th>Date</th>
					<th>Amount</th>
					<th>Status</th>
					<th></th>
				</tr>
			</thead>
			<tbody class="[&>tr]:hover:preset-tonal-primary">
				{#each invoices as inv (inv.id)}
					{@const meta = statusMeta[inv.status]}
					{@const Icon = meta.icon}
					<tr>
						<td class="text-sm font-semibold">{inv.id}</td>
						<td class="text-sm opacity-70">{inv.date}</td>
						<td class="text-sm">{inv.amount}</td>
						<td>
							<span class="badge inline-flex items-center gap-1 {meta.preset} text-xs">
								<Icon class="size-3" />
								{inv.status}
							</span>
						</td>
						<td class="text-right">
							<button class="btn-icon btn-icon-sm preset-tonal" aria-label="Download {inv.id}">
								<DownloadIcon class="size-4" />
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>
