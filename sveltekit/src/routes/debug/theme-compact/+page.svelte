<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Accordion,
		Pagination,
		Progress,
		SegmentedControl,
		Slider,
		Steps,
		Switch,
		Tabs
	} from '@skeletonlabs/skeleton-svelte';
	import {
		TriangleAlertIcon,
		CircleCheckBigIcon,
		CheckIcon,
		InfoIcon,
		MapPinIcon,
		StarIcon,
		CircleXIcon
	} from '@lucide/svelte';
	import { APP_NAME } from '$lib/config/app';
	import { mode } from '$lib/stores/mode.svelte';
	import Card from '$lib/components/ui/Card.svelte';

	type ColorName =
		| 'primary'
		| 'secondary'
		| 'tertiary'
		| 'success'
		| 'warning'
		| 'error'
		| 'surface';

	type Swatch = { step: number; bg: string; text: string };
	type PaletteRow = { name: ColorName; steps: Swatch[] };

	const PALETTE: PaletteRow[] = [
		{
			name: 'primary',
			steps: [
				{ step: 50, bg: 'bg-primary-50', text: 'text-primary-contrast-50' },
				{ step: 100, bg: 'bg-primary-100', text: 'text-primary-contrast-100' },
				{ step: 200, bg: 'bg-primary-200', text: 'text-primary-contrast-200' },
				{ step: 300, bg: 'bg-primary-300', text: 'text-primary-contrast-300' },
				{ step: 400, bg: 'bg-primary-400', text: 'text-primary-contrast-400' },
				{ step: 500, bg: 'bg-primary-500', text: 'text-primary-contrast-500' },
				{ step: 600, bg: 'bg-primary-600', text: 'text-primary-contrast-600' },
				{ step: 700, bg: 'bg-primary-700', text: 'text-primary-contrast-700' },
				{ step: 800, bg: 'bg-primary-800', text: 'text-primary-contrast-800' },
				{ step: 900, bg: 'bg-primary-900', text: 'text-primary-contrast-900' },
				{ step: 950, bg: 'bg-primary-950', text: 'text-primary-contrast-950' }
			]
		},
		{
			name: 'secondary',
			steps: [
				{ step: 50, bg: 'bg-secondary-50', text: 'text-secondary-contrast-50' },
				{ step: 100, bg: 'bg-secondary-100', text: 'text-secondary-contrast-100' },
				{ step: 200, bg: 'bg-secondary-200', text: 'text-secondary-contrast-200' },
				{ step: 300, bg: 'bg-secondary-300', text: 'text-secondary-contrast-300' },
				{ step: 400, bg: 'bg-secondary-400', text: 'text-secondary-contrast-400' },
				{ step: 500, bg: 'bg-secondary-500', text: 'text-secondary-contrast-500' },
				{ step: 600, bg: 'bg-secondary-600', text: 'text-secondary-contrast-600' },
				{ step: 700, bg: 'bg-secondary-700', text: 'text-secondary-contrast-700' },
				{ step: 800, bg: 'bg-secondary-800', text: 'text-secondary-contrast-800' },
				{ step: 900, bg: 'bg-secondary-900', text: 'text-secondary-contrast-900' },
				{ step: 950, bg: 'bg-secondary-950', text: 'text-secondary-contrast-950' }
			]
		},
		{
			name: 'tertiary',
			steps: [
				{ step: 50, bg: 'bg-tertiary-50', text: 'text-tertiary-contrast-50' },
				{ step: 100, bg: 'bg-tertiary-100', text: 'text-tertiary-contrast-100' },
				{ step: 200, bg: 'bg-tertiary-200', text: 'text-tertiary-contrast-200' },
				{ step: 300, bg: 'bg-tertiary-300', text: 'text-tertiary-contrast-300' },
				{ step: 400, bg: 'bg-tertiary-400', text: 'text-tertiary-contrast-400' },
				{ step: 500, bg: 'bg-tertiary-500', text: 'text-tertiary-contrast-500' },
				{ step: 600, bg: 'bg-tertiary-600', text: 'text-tertiary-contrast-600' },
				{ step: 700, bg: 'bg-tertiary-700', text: 'text-tertiary-contrast-700' },
				{ step: 800, bg: 'bg-tertiary-800', text: 'text-tertiary-contrast-800' },
				{ step: 900, bg: 'bg-tertiary-900', text: 'text-tertiary-contrast-900' },
				{ step: 950, bg: 'bg-tertiary-950', text: 'text-tertiary-contrast-950' }
			]
		},
		{
			name: 'success',
			steps: [
				{ step: 50, bg: 'bg-success-50', text: 'text-success-contrast-50' },
				{ step: 100, bg: 'bg-success-100', text: 'text-success-contrast-100' },
				{ step: 200, bg: 'bg-success-200', text: 'text-success-contrast-200' },
				{ step: 300, bg: 'bg-success-300', text: 'text-success-contrast-300' },
				{ step: 400, bg: 'bg-success-400', text: 'text-success-contrast-400' },
				{ step: 500, bg: 'bg-success-500', text: 'text-success-contrast-500' },
				{ step: 600, bg: 'bg-success-600', text: 'text-success-contrast-600' },
				{ step: 700, bg: 'bg-success-700', text: 'text-success-contrast-700' },
				{ step: 800, bg: 'bg-success-800', text: 'text-success-contrast-800' },
				{ step: 900, bg: 'bg-success-900', text: 'text-success-contrast-900' },
				{ step: 950, bg: 'bg-success-950', text: 'text-success-contrast-950' }
			]
		},
		{
			name: 'warning',
			steps: [
				{ step: 50, bg: 'bg-warning-50', text: 'text-warning-contrast-50' },
				{ step: 100, bg: 'bg-warning-100', text: 'text-warning-contrast-100' },
				{ step: 200, bg: 'bg-warning-200', text: 'text-warning-contrast-200' },
				{ step: 300, bg: 'bg-warning-300', text: 'text-warning-contrast-300' },
				{ step: 400, bg: 'bg-warning-400', text: 'text-warning-contrast-400' },
				{ step: 500, bg: 'bg-warning-500', text: 'text-warning-contrast-500' },
				{ step: 600, bg: 'bg-warning-600', text: 'text-warning-contrast-600' },
				{ step: 700, bg: 'bg-warning-700', text: 'text-warning-contrast-700' },
				{ step: 800, bg: 'bg-warning-800', text: 'text-warning-contrast-800' },
				{ step: 900, bg: 'bg-warning-900', text: 'text-warning-contrast-900' },
				{ step: 950, bg: 'bg-warning-950', text: 'text-warning-contrast-950' }
			]
		},
		{
			name: 'error',
			steps: [
				{ step: 50, bg: 'bg-error-50', text: 'text-error-contrast-50' },
				{ step: 100, bg: 'bg-error-100', text: 'text-error-contrast-100' },
				{ step: 200, bg: 'bg-error-200', text: 'text-error-contrast-200' },
				{ step: 300, bg: 'bg-error-300', text: 'text-error-contrast-300' },
				{ step: 400, bg: 'bg-error-400', text: 'text-error-contrast-400' },
				{ step: 500, bg: 'bg-error-500', text: 'text-error-contrast-500' },
				{ step: 600, bg: 'bg-error-600', text: 'text-error-contrast-600' },
				{ step: 700, bg: 'bg-error-700', text: 'text-error-contrast-700' },
				{ step: 800, bg: 'bg-error-800', text: 'text-error-contrast-800' },
				{ step: 900, bg: 'bg-error-900', text: 'text-error-contrast-900' },
				{ step: 950, bg: 'bg-error-950', text: 'text-error-contrast-950' }
			]
		},
		{
			name: 'surface',
			steps: [
				{ step: 50, bg: 'bg-surface-50', text: 'text-surface-contrast-50' },
				{ step: 100, bg: 'bg-surface-100', text: 'text-surface-contrast-100' },
				{ step: 200, bg: 'bg-surface-200', text: 'text-surface-contrast-200' },
				{ step: 300, bg: 'bg-surface-300', text: 'text-surface-contrast-300' },
				{ step: 400, bg: 'bg-surface-400', text: 'text-surface-contrast-400' },
				{ step: 500, bg: 'bg-surface-500', text: 'text-surface-contrast-500' },
				{ step: 600, bg: 'bg-surface-600', text: 'text-surface-contrast-600' },
				{ step: 700, bg: 'bg-surface-700', text: 'text-surface-contrast-700' },
				{ step: 800, bg: 'bg-surface-800', text: 'text-surface-contrast-800' },
				{ step: 900, bg: 'bg-surface-900', text: 'text-surface-contrast-900' },
				{ step: 950, bg: 'bg-surface-950', text: 'text-surface-contrast-950' }
			]
		}
	];

	const SEMANTIC_COLORS: ColorName[] = [
		'primary',
		'secondary',
		'tertiary',
		'success',
		'warning',
		'error'
	];

	// Theme name read from the data-theme attribute on <html>; mode comes from
	// the global mode store. SSR is disabled in this project so reading on mount
	// is safe.
	let themeName = $state('—');

	onMount(() => {
		themeName = document.documentElement.dataset.theme ?? '(none)';
	});

	// Half-complete states for the screenshot.
	let switchA = $state(true);
	let switchB = $state(false);
	let sliderValue = $state([50]);
	let textValue = $state('Blood Gulch');
	let selectValue = $state('hillshade');
	let segmentValue = $state('hillshade');
	let stepIndex = $state(2); // step 0,1 complete · step 2 current · step 3 pending
	let pageNum = $state(3);
	let activeTab = $state('players');

	const STEPS = [
		{ title: 'Survey', body: 'Walk the perimeter, note landmarks.' },
		{ title: 'Plot', body: 'Lay out the topographic grid.' },
		{ title: 'Annotate', body: 'Add elevation contours and structures.' },
		{ title: 'Publish', body: 'Export and share with the squad.' }
	];

	type RunnerRow = {
		name: string;
		status: 'running' | 'pregame' | 'stopped';
		players: string;
		tick: string;
	};
	const RUNNERS: RunnerRow[] = [
		{ name: 'smoke-1', status: 'running', players: '4 / 16', tick: '12,047' },
		{ name: 'ranked-2', status: 'running', players: '2 / 16', tick: '8,432' },
		{ name: 'custom-3', status: 'pregame', players: '1 / 16', tick: '0' },
		{ name: 'local-test', status: 'stopped', players: '0 / 16', tick: '—' }
	];
	const STATUS_PRESET: Record<RunnerRow['status'], string> = {
		running: 'preset-filled-success-500',
		pregame: 'preset-filled-warning-500',
		stopped: 'preset-tonal-surface'
	};
</script>

<svelte:head>
	<title>Theme Preview (compact) · {APP_NAME}</title>
</svelte:head>

<div class="mx-auto flex max-w-350 flex-col gap-1">
	<!-- ============================================================== HEADER -->
	<header class="flex flex-wrap items-end justify-between gap-1">
		<div class="min-w-0">
			<div class="flex flex-wrap items-center gap-2">
				<h1 class="h3">Theme Preview · {themeName}</h1>
				<span
					class="badge text-[10px] font-semibold tracking-wider uppercase {mode.current === 'dark'
						? 'preset-filled-tertiary-500'
						: 'preset-filled-warning-500'}"
				>
					{mode.current}
				</span>
			</div>
			<!-- <p class="mt-1 text-xs text-surface-700-300">
				Skeleton v4 — palette, components, and chrome wrappers in one screenshot. Components show
				half-complete states (slider 50%, progress 50/75%, stepper 3 / 4, pagination mid-set).
			</p> -->
		</div>
		<div class="hidden text-xs text-surface-700-300 sm:block">
			Toggle dark/light from the header switch ›
		</div>
	</header>

	<!-- =========================================================== PALETTE -->
	<Card tone="outlined" size="sm">
		<div class="mb-2 flex items-baseline justify-between">
			<div class="text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
				Palette · 50 → 950 (with contrast text)
			</div>
			<div class="text-[10px] text-surface-700-300">7 color groups × 11 steps</div>
		</div>
		<div class="flex flex-col gap-1">
			{#each PALETTE as row (row.name)}
				<div class="flex items-center gap-2">
					<span
						class="w-20 shrink-0 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase"
					>
						{row.name}
					</span>
					<div class="grid flex-1 grid-cols-11 overflow-hidden rounded">
						{#each row.steps as swatch (swatch.step)}
							<div
								class="flex h-7 items-center justify-center text-[10px] font-medium {swatch.bg} {swatch.text}"
								title="{row.name}-{swatch.step}"
							>
								{swatch.step}
							</div>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</Card>

	<div class="grid grid-cols-1 gap-2 lg:grid-cols-3">
		<!-- =================================================== COLUMN 1 -->
		<div class="flex flex-col gap-2">
			<Card tone="outlined" size="sm">
				<div class="mb-1 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Typography
				</div>
				<h1 class="h2">Heading One</h1>
				<h2 class="h4">Heading Two · subtitle</h2>
				<p class="mt-1 text-sm">
					Body paragraph with an <a class="anchor" href="/debug/theme-compact/">anchor link</a>,
					some <code>inline code</code>, and a <kbd class="kbd">Ctrl</kbd> +
					<kbd class="kbd">K</kbd> shortcut. <strong>Bold</strong> · <em>italic</em>.
				</p>
				<blockquote class="mt-2 blockquote text-sm">
					“No foreign lands, only foreign travelers.”
				</blockquote>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Card tones
				</div>
				<div class="grid grid-cols-3 gap-2">
					<Card tone="outlined" size="sm">
						<div class="text-xs font-semibold">Outlined</div>
						<div class="mt-1 text-[10px] text-surface-700-300">border, no fill</div>
					</Card>
					<Card tone="tonal" size="sm">
						<div class="text-xs font-semibold">Tonal</div>
						<div class="mt-1 text-[10px] text-surface-700-300">preset-tonal</div>
					</Card>
					<Card tone="filled" size="sm">
						<div class="text-xs font-semibold">Filled</div>
						<div class="mt-1 text-[10px] text-surface-700-300">surface-100-900</div>
					</Card>
				</div>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Alerts
				</div>
				<div class="flex flex-col gap-1.5">
					<aside class="alert flex items-center gap-2 preset-filled-success-500 text-xs">
						<CircleCheckBigIcon class="size-4 shrink-0" />
						<p><strong>Save complete</strong> · Map exported to ~/maps/blood-gulch.json</p>
					</aside>
					<aside class="alert flex items-center gap-2 preset-filled-warning-500 text-xs">
						<TriangleAlertIcon class="size-4 shrink-0" />
						<p><strong>Tick rate degraded</strong> · 28 Hz (target 30 Hz)</p>
					</aside>
					<aside class="alert flex items-center gap-2 preset-filled-error-500 text-xs">
						<CircleXIcon class="size-4 shrink-0" />
						<p><strong>Connection lost</strong> · Reconnecting in 3s…</p>
					</aside>
					<aside class="alert flex items-center gap-2 preset-tonal-primary text-xs">
						<InfoIcon class="size-4 shrink-0" />
						<p><strong>Update available</strong> · Cartographer v0.7.1</p>
					</aside>
				</div>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Progress
				</div>
				<div class="flex items-center gap-4">
					<div class="flex flex-1 flex-col items-center justify-between text-xs">
						<span>Linear</span>
						<span class="text-surface-700-300">50 / 100</span>
						<Progress value={50}>
							<Progress.Track
								class="block h-2 w-full overflow-hidden rounded-full bg-surface-300-700"
							>
								<Progress.Range class="block h-full bg-primary-500" />
							</Progress.Track>
						</Progress>
					</div>
					<div class="flex flex-1 flex-col items-center justify-between text-xs">
						<span>Secondary</span>
						<span class="text-surface-700-300">75 / 100</span>
						<Progress value={75}>
							<Progress.Track
								class="block h-2 w-full overflow-hidden rounded-full bg-surface-300-700"
							>
								<Progress.Range class="block h-full bg-secondary-500" />
							</Progress.Track>
						</Progress>
					</div>

					<div class="flex flex-col items-center justify-between text-xs">
						<Progress value={50}>
							<Progress.Circle class="size-16 [--size:64px] [--thickness:8px]">
								<div class="absolute inset-0 flex items-center justify-center">
									<Progress.ValueText />
								</div>
								<Progress.CircleTrack class="stroke-surface-300-700" />
								<Progress.CircleRange class="stroke-tertiary-500" />
							</Progress.Circle>
						</Progress>
					</div>
				</div>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Steps · 3 of 4
				</div>
				<Steps count={STEPS.length} step={stepIndex} onStepChange={(d) => (stepIndex = d.step)}>
					<Steps.List class="flex items-center justify-between">
						{#each STEPS as item, index (item.title)}
							<Steps.Item {index} class="flex flex-1 items-center gap-1">
								<Steps.Trigger class="flex flex-col items-center gap-1">
									<Steps.Indicator
										class="flex size-6 items-center justify-center rounded-full border border-surface-300-700 text-[10px] data-complete:border-success-500 data-complete:bg-success-500 data-complete:text-success-contrast-500 data-current:border-primary-500 data-current:bg-primary-500 data-current:text-primary-contrast-500"
									>
										{#if index < stepIndex}
											<CheckIcon class="size-3" />
										{:else}
											{index + 1}
										{/if}
									</Steps.Indicator>
									<span class="text-[10px]">{item.title}</span>
								</Steps.Trigger>
								{#if index < STEPS.length - 1}
									<Steps.Separator
										class="h-px flex-1 bg-surface-300-700 data-complete:bg-success-500"
									/>
								{/if}
							</Steps.Item>
						{/each}
					</Steps.List>
					<div class="mt-2 text-xs text-surface-700-300">
						{STEPS[stepIndex]?.body}
					</div>
				</Steps>
			</Card>
		</div>

		<!-- =================================================== COLUMN 2 -->
		<div class="flex flex-col gap-2">
			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Buttons
				</div>
				<div class="flex flex-col gap-1.5">
					<div class="flex flex-wrap items-center gap-1.5">
						<span class="w-14 text-[10px] text-surface-700-300">Filled</span>
						<button type="button" class="btn preset-filled-primary-500 btn-sm">primary</button>
						<button type="button" class="btn preset-filled-secondary-500 btn-sm">secondary</button>
						<button type="button" class="btn preset-filled-tertiary-500 btn-sm">tertiary</button>
						<button type="button" class="btn preset-filled-error-500 btn-sm">error</button>
					</div>
					<div class="flex flex-wrap items-center gap-1.5">
						<span class="w-14 text-[10px] text-surface-700-300">Tonal</span>
						<button type="button" class="btn preset-tonal-primary btn-sm">primary</button>
						<button type="button" class="btn preset-tonal-secondary btn-sm">secondary</button>
						<button type="button" class="btn preset-tonal-tertiary btn-sm">tertiary</button>
						<button type="button" class="btn preset-tonal-error btn-sm">error</button>
					</div>
					<div class="flex flex-wrap items-center gap-1.5">
						<span class="w-14 text-[10px] text-surface-700-300">Outlined</span>
						<button type="button" class="btn preset-outlined-primary-500 btn-sm">primary</button>
						<button type="button" class="btn preset-outlined-secondary-500 btn-sm">secondary</button
						>
						<button type="button" class="btn preset-filled-primary-500 btn-sm" disabled
							>disabled</button
						>
						<button type="button" class="btn-icon preset-tonal-primary" aria-label="Star">
							<StarIcon class="size-4" />
						</button>
					</div>
				</div>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Badges · Chips
				</div>
				<div class="flex flex-wrap gap-1.5">
					{#each SEMANTIC_COLORS as color (color)}
						<span class="badge preset-filled-{color}-500 text-xs">{color}</span>
					{/each}
				</div>
				<div class="mt-2 flex flex-wrap gap-1.5">
					{#each SEMANTIC_COLORS as color (color)}
						<span class="chip preset-tonal-{color} text-xs">{color}</span>
					{/each}
					<span class="chip preset-filled-primary-500 text-xs">
						<MapPinIcon class="size-3" />
						<span>Pinned</span>
					</span>
				</div>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Form controls
				</div>
				<div class="grid grid-cols-2 gap-2">
					<label class="label">
						<span class="label-text text-xs">Map name</span>
						<input class="input text-sm" type="text" bind:value={textValue} />
					</label>
					<label class="label">
						<span class="label-text text-xs">Style</span>
						<select class="select text-sm" bind:value={selectValue}>
							<option value="contour">Contour</option>
							<option value="hillshade">Hillshade</option>
							<option value="satellite">Satellite</option>
						</select>
					</label>
					<label class="col-span-2 label">
						<span class="label-text text-xs">Email · invalid</span>
						<input
							class="input text-sm ring-2 ring-error-500"
							type="email"
							value="not-an-email"
							aria-invalid="true"
						/>
					</label>
					<div class="col-span-2 flex flex-wrap items-center gap-3 text-xs">
						<label class="flex items-center gap-1.5">
							<input class="checkbox" type="checkbox" checked />
							<span>Contours</span>
						</label>
						<label class="flex items-center gap-1.5">
							<input class="checkbox" type="checkbox" />
							<span>Grid</span>
						</label>
						<label class="flex items-center gap-1.5">
							<input class="radio" type="radio" name="r" checked />
							<span>Topo</span>
						</label>
						<label class="flex items-center gap-1.5">
							<input class="radio" type="radio" name="r" />
							<span>Political</span>
						</label>
					</div>
					<div class="col-span-2 flex items-center gap-3">
						<Switch
							checked={switchA}
							onCheckedChange={(d) => (switchA = d.checked)}
							aria-label="Switch A"
						>
							<Switch.Control
								class="inline-flex h-5 w-9 cursor-pointer items-center rounded-full bg-surface-300-700 transition-colors data-[state=checked]:bg-primary-500"
							>
								<Switch.Thumb class="size-4 rounded-full bg-white shadow-sm transition-transform" />
							</Switch.Control>
							<Switch.HiddenInput />
						</Switch>
						<Switch
							checked={switchB}
							onCheckedChange={(d) => (switchB = d.checked)}
							aria-label="Switch B"
						>
							<Switch.Control
								class="inline-flex h-5 w-9 cursor-pointer items-center rounded-full bg-surface-300-700 transition-colors data-[state=checked]:bg-primary-500"
							>
								<Switch.Thumb class="size-4 rounded-full bg-white shadow-sm transition-transform" />
							</Switch.Control>
							<Switch.HiddenInput />
						</Switch>
						<span class="text-xs text-surface-700-300">
							A: {switchA ? 'on' : 'off'} · B: {switchB ? 'on' : 'off'}
						</span>
					</div>
					<div class="col-span-2">
						<div class="mb-1 flex items-center justify-between text-xs">
							<span>Slider</span>
							<span class="text-surface-700-300">{sliderValue[0]}%</span>
						</div>
						<Slider
							value={sliderValue}
							onValueChange={(d) => (sliderValue = d.value)}
							min={0}
							max={100}
							step={1}
						>
							<Slider.Control class="relative flex h-5 w-full items-center">
								<Slider.Track class="relative h-1.5 w-full grow rounded-full bg-surface-300-700">
									<Slider.Range class="absolute h-full rounded-full bg-primary-500" />
								</Slider.Track>
								<Slider.Thumb
									index={0}
									class="block size-4 rounded-full bg-primary-500 shadow ring-2 ring-surface-50 focus:outline-none dark:ring-surface-950"
								>
									<Slider.HiddenInput />
								</Slider.Thumb>
							</Slider.Control>
						</Slider>
					</div>
					<div class="col-span-2">
						<div class="mb-1 text-xs">Segmented</div>
						<SegmentedControl
							class="flex flex-row"
							value={segmentValue}
							onValueChange={(d) => (segmentValue = d.value ?? segmentValue)}
						>
							<SegmentedControl.Indicator />
							<SegmentedControl.Item value="contour">
								<SegmentedControl.ItemText>Contour</SegmentedControl.ItemText>
								<SegmentedControl.ItemHiddenInput />
							</SegmentedControl.Item>
							<SegmentedControl.Item value="hillshade">
								<SegmentedControl.ItemText>Hillshade</SegmentedControl.ItemText>
								<SegmentedControl.ItemHiddenInput />
							</SegmentedControl.Item>
							<SegmentedControl.Item value="satellite">
								<SegmentedControl.ItemText>Satellite</SegmentedControl.ItemText>
								<SegmentedControl.ItemHiddenInput />
							</SegmentedControl.Item>
						</SegmentedControl>
					</div>
				</div>
			</Card>
		</div>

		<!-- =================================================== COLUMN 3 -->
		<div class="flex flex-col gap-2">
			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Table · scraper runners
				</div>
				<table class="table-hover table w-full text-xs">
					<thead>
						<tr>
							<th class="text-left">Name</th>
							<th class="text-left">Status</th>
							<th class="text-right">Players</th>
							<th class="text-right">Tick</th>
						</tr>
					</thead>
					<tbody>
						{#each RUNNERS as r (r.name)}
							<tr>
								<td><code class="text-[11px]">{r.name}</code></td>
								<td>
									<span class="badge {STATUS_PRESET[r.status]} text-[10px]">{r.status}</span>
								</td>
								<td class="text-right">{r.players}</td>
								<td class="text-right tabular-nums">{r.tick}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</Card>

			<Card tone="outlined" size="sm">
				<div class="mb-2 text-[10px] font-semibold tracking-wider text-surface-700-300 uppercase">
					Tabs · Accordion · Pagination
				</div>
				<Tabs value={activeTab} onValueChange={(d) => (activeTab = d.value)}>
					<Tabs.List>
						<Tabs.Trigger value="overview">Overview</Tabs.Trigger>
						<Tabs.Trigger value="players">Players</Tabs.Trigger>
						<Tabs.Trigger value="events">Events</Tabs.Trigger>
						<Tabs.Indicator />
					</Tabs.List>
					<Tabs.Content value="overview" class="pt-2 text-xs">
						Summary of the current map run.
					</Tabs.Content>
					<Tabs.Content value="players" class="pt-2 text-xs">
						Roster: 4 active players, 2 spectators.
					</Tabs.Content>
					<Tabs.Content value="events" class="pt-2 text-xs">
						12 events recorded since match start.
					</Tabs.Content>
				</Tabs>
				<div class="mt-2">
					<Accordion value={['general']} multiple>
						<Accordion.Item value="general">
							<Accordion.ItemTrigger class="flex w-full items-center justify-between py-1 text-sm">
								<span>General settings</span>
								<Accordion.ItemIndicator>+</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-2 text-xs text-surface-700-300">
								Units, default zoom, autosave interval.
							</Accordion.ItemContent>
						</Accordion.Item>
						<Accordion.Item value="advanced">
							<Accordion.ItemTrigger class="flex w-full items-center justify-between py-1 text-sm">
								<span>Advanced</span>
								<Accordion.ItemIndicator>+</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-2 text-xs text-surface-700-300">
								Tick rate, broadcast cadence, log size.
							</Accordion.ItemContent>
						</Accordion.Item>
					</Accordion>
				</div>
				<div class="mt-2">
					<Pagination
						count={45}
						pageSize={5}
						page={pageNum}
						onPageChange={(d) => (pageNum = d.page)}
					>
						<div class="flex flex-wrap items-center gap-1">
							<Pagination.PrevTrigger class="btn preset-tonal btn-sm">‹</Pagination.PrevTrigger>
							<Pagination.Context>
								{#snippet children(api)}
									{#each api().pages as p, i (i)}
										{#if p.type === 'page'}
											<Pagination.Item
												{...p}
												class="btn preset-tonal btn-sm data-selected:preset-filled-primary-500"
											>
												{p.value}
											</Pagination.Item>
										{:else}
											<Pagination.Ellipsis index={i} class="px-1 text-xs">…</Pagination.Ellipsis>
										{/if}
									{/each}
								{/snippet}
							</Pagination.Context>
							<Pagination.NextTrigger class="btn preset-tonal btn-sm">›</Pagination.NextTrigger>
						</div>
					</Pagination>
				</div>
			</Card>
		</div>
	</div>
</div>
