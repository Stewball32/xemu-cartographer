<script lang="ts">
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import { CheckIcon, XIcon, SparklesIcon } from '@lucide/svelte';

	interface Plan {
		name: string;
		tagline: string;
		monthly: number;
		yearly: number;
		features: { label: string; included: boolean }[];
		highlight: boolean;
		ctaLabel: string;
		ctaPreset: string;
	}

	let yearly = $state(false);

	const plans: Plan[] = [
		{
			name: 'Hobby',
			tagline: 'For personal projects and experiments.',
			monthly: 0,
			yearly: 0,
			features: [
				{ label: '1 project', included: true },
				{ label: 'Community support', included: true },
				{ label: '100 MB storage', included: true },
				{ label: 'Custom domain', included: false },
				{ label: 'Team collaboration', included: false },
				{ label: 'Priority support', included: false }
			],
			highlight: false,
			ctaLabel: 'Get Started',
			ctaPreset: 'preset-tonal'
		},
		{
			name: 'Pro',
			tagline: 'For professionals shipping real products.',
			monthly: 19,
			yearly: 190,
			features: [
				{ label: 'Unlimited projects', included: true },
				{ label: 'Email support', included: true },
				{ label: '10 GB storage', included: true },
				{ label: 'Custom domain', included: true },
				{ label: 'Team collaboration (up to 5)', included: true },
				{ label: 'Priority support', included: false }
			],
			highlight: true,
			ctaLabel: 'Upgrade to Pro',
			ctaPreset: 'preset-filled-primary-500'
		},
		{
			name: 'Enterprise',
			tagline: 'For organizations with advanced needs.',
			monthly: 99,
			yearly: 990,
			features: [
				{ label: 'Unlimited projects', included: true },
				{ label: 'Dedicated support', included: true },
				{ label: '1 TB storage', included: true },
				{ label: 'Custom domain + SSO', included: true },
				{ label: 'Unlimited team members', included: true },
				{ label: 'SLA + priority support', included: true }
			],
			highlight: false,
			ctaLabel: 'Contact Sales',
			ctaPreset: 'preset-filled'
		}
	];
</script>

<div class="mx-auto max-w-6xl">
	<div class="mb-10 space-y-4 text-center">
		<h1 class="h1">Pricing</h1>
		<p class="mx-auto max-w-2xl opacity-70">
			Start free and scale as you grow. Switch plans or cancel anytime — no contracts, no lock-in.
		</p>
		<div class="flex items-center justify-center gap-3">
			<span class="text-sm {!yearly ? 'font-semibold' : 'opacity-60'}">Monthly</span>
			<Switch checked={yearly} onCheckedChange={(details) => (yearly = details.checked)}>
				<Switch.Control>
					<Switch.Thumb />
				</Switch.Control>
				<Switch.HiddenInput />
			</Switch>
			<span class="text-sm {yearly ? 'font-semibold' : 'opacity-60'}">Yearly</span>
			<span class="badge preset-tonal-success text-xs">Save 17%</span>
		</div>
	</div>

	<div class="grid gap-6 md:grid-cols-3">
		{#each plans as plan (plan.name)}
			<div
				class="relative flex flex-col gap-6 card p-8 {plan.highlight
					? 'preset-outlined-primary-500 ring-2 ring-primary-500'
					: ''}"
			>
				{#if plan.highlight}
					<span
						class="absolute -top-3 left-1/2 badge flex -translate-x-1/2 items-center gap-1 preset-filled-primary-500"
					>
						<SparklesIcon class="size-3" />
						Most Popular
					</span>
				{/if}

				<div class="space-y-2">
					<h2 class="h3">{plan.name}</h2>
					<p class="text-sm opacity-70">{plan.tagline}</p>
				</div>

				<div>
					<span class="text-5xl font-bold">${yearly ? plan.yearly : plan.monthly}</span>
					<span class="text-sm opacity-60">/ {yearly ? 'year' : 'month'}</span>
				</div>

				<button class="btn {plan.ctaPreset}">{plan.ctaLabel}</button>

				<hr class="hr" />

				<ul class="space-y-3">
					{#each plan.features as feature (feature.label)}
						<li class="flex items-center gap-3 text-sm">
							{#if feature.included}
								<CheckIcon class="size-4 text-success-500" />
								<span>{feature.label}</span>
							{:else}
								<XIcon class="size-4 opacity-40" />
								<span class="line-through opacity-50">{feature.label}</span>
							{/if}
						</li>
					{/each}
				</ul>
			</div>
		{/each}
	</div>
</div>
