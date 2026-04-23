<script lang="ts">
	import { Steps } from '@skeletonlabs/skeleton-svelte';
	import { UserIcon, BriefcaseIcon, CreditCardIcon, CheckCircle2Icon } from '@lucide/svelte';

	interface Step {
		title: string;
		description: string;
		icon: typeof UserIcon;
	}

	const steps: Step[] = [
		{ title: 'Account', description: 'Tell us about yourself', icon: UserIcon },
		{ title: 'Workspace', description: 'Set up your workspace', icon: BriefcaseIcon },
		{ title: 'Billing', description: 'Add payment details', icon: CreditCardIcon },
		{ title: 'Review', description: 'Confirm and submit', icon: CheckCircle2Icon }
	];

	let currentStep = $state(0);

	let form = $state({
		firstName: 'Jane',
		lastName: 'Doe',
		email: 'jane.doe@example.com',
		workspaceName: 'Acme Inc.',
		workspaceSlug: 'acme',
		teamSize: '11-50',
		plan: 'pro',
		cardName: '',
		cardNumber: '',
		cardExpiry: '',
		cardCvc: ''
	});
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-2 h2">Setup Wizard</h1>
	<p class="mb-8 text-sm opacity-70">Complete the following steps to finish onboarding.</p>

	<Steps count={steps.length} step={currentStep} onStepChange={(e) => (currentStep = e.step)}>
		<Steps.List>
			{#each steps as item, index (index)}
				<Steps.Item {index}>
					<Steps.Trigger class="flex items-center gap-2">
						<Steps.Indicator>{index + 1}</Steps.Indicator>
						<span class="hidden sm:inline">{item.title}</span>
					</Steps.Trigger>
				</Steps.Item>
				{#if index < steps.length - 1}
					<Steps.Separator />
				{/if}
			{/each}
		</Steps.List>

		<Steps.Content index={0}>
			<div class="mt-6 space-y-4 card p-6">
				<h2 class="h4">Account</h2>
				<p class="text-sm opacity-70">Tell us a bit about yourself to create your account.</p>
				<div class="grid gap-4 sm:grid-cols-2">
					<label class="label">
						<span class="text-sm">First name</span>
						<input class="input" type="text" bind:value={form.firstName} />
					</label>
					<label class="label">
						<span class="text-sm">Last name</span>
						<input class="input" type="text" bind:value={form.lastName} />
					</label>
				</div>
				<label class="label">
					<span class="text-sm">Email</span>
					<input class="input" type="email" bind:value={form.email} />
				</label>
			</div>
		</Steps.Content>

		<Steps.Content index={1}>
			<div class="mt-6 space-y-4 card p-6">
				<h2 class="h4">Workspace</h2>
				<p class="text-sm opacity-70">Configure your workspace. You can change these later.</p>
				<label class="label">
					<span class="text-sm">Workspace name</span>
					<input class="input" type="text" bind:value={form.workspaceName} />
				</label>
				<label class="label">
					<span class="text-sm">Workspace slug</span>
					<input class="input" type="text" bind:value={form.workspaceSlug} />
				</label>
				<label class="label">
					<span class="text-sm">Team size</span>
					<select class="select" bind:value={form.teamSize}>
						<option value="1-10">1–10</option>
						<option value="11-50">11–50</option>
						<option value="51-200">51–200</option>
						<option value="200+">200+</option>
					</select>
				</label>
			</div>
		</Steps.Content>

		<Steps.Content index={2}>
			<div class="mt-6 space-y-4 card p-6">
				<h2 class="h4">Billing</h2>
				<p class="text-sm opacity-70">Add a payment method for your selected plan.</p>
				<label class="label">
					<span class="text-sm">Plan</span>
					<select class="select" bind:value={form.plan}>
						<option value="hobby">Hobby — Free</option>
						<option value="pro">Pro — $19/mo</option>
						<option value="enterprise">Enterprise — $99/mo</option>
					</select>
				</label>
				<label class="label">
					<span class="text-sm">Name on card</span>
					<input class="input" type="text" placeholder="Jane Doe" bind:value={form.cardName} />
				</label>
				<label class="label">
					<span class="text-sm">Card number</span>
					<input
						class="input"
						type="text"
						placeholder="1234 5678 9012 3456"
						bind:value={form.cardNumber}
					/>
				</label>
				<div class="grid gap-4 sm:grid-cols-2">
					<label class="label">
						<span class="text-sm">Expiry</span>
						<input class="input" type="text" placeholder="MM/YY" bind:value={form.cardExpiry} />
					</label>
					<label class="label">
						<span class="text-sm">CVC</span>
						<input class="input" type="text" placeholder="123" bind:value={form.cardCvc} />
					</label>
				</div>
			</div>
		</Steps.Content>

		<Steps.Content index={3}>
			<div class="mt-6 space-y-4 card p-6">
				<h2 class="h4">Review</h2>
				<p class="text-sm opacity-70">Double check everything before submitting.</p>
				<dl class="grid gap-2 text-sm">
					<div class="flex justify-between border-b border-surface-200-800 py-2">
						<dt class="opacity-70">Name</dt>
						<dd class="font-semibold">{form.firstName} {form.lastName}</dd>
					</div>
					<div class="flex justify-between border-b border-surface-200-800 py-2">
						<dt class="opacity-70">Email</dt>
						<dd class="font-semibold">{form.email}</dd>
					</div>
					<div class="flex justify-between border-b border-surface-200-800 py-2">
						<dt class="opacity-70">Workspace</dt>
						<dd class="font-semibold">{form.workspaceName} ({form.workspaceSlug})</dd>
					</div>
					<div class="flex justify-between border-b border-surface-200-800 py-2">
						<dt class="opacity-70">Team size</dt>
						<dd class="font-semibold">{form.teamSize}</dd>
					</div>
					<div class="flex justify-between py-2">
						<dt class="opacity-70">Plan</dt>
						<dd class="font-semibold capitalize">{form.plan}</dd>
					</div>
				</dl>
				<button class="btn w-full preset-filled-primary-500">Submit</button>
			</div>
		</Steps.Content>

		<div class="mt-6 flex justify-between">
			<Steps.PrevTrigger class="btn preset-tonal">Back</Steps.PrevTrigger>
			<Steps.NextTrigger class="btn preset-filled">Next</Steps.NextTrigger>
		</div>
	</Steps>
</div>
