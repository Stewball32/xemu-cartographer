<script lang="ts">
	// Probe tab: live memory-probe diagnostics from the active GameReader
	// plugin. score_probe lists every candidate address the reader is
	// touching for gametype / team-score / per-player-score detection;
	// state_inputs is the per-tick state-classification bag (booleans /
	// pointers that drive Idle/Ready/Live transitions). Both feed offset
	// hunting — keep them in one tab so an operator can switch views
	// without leaving the page.

	import { SegmentedControl } from '@skeletonlabs/skeleton-svelte';
	import { useDebugContext } from '../context.js';
	import { buildProbeVm } from './probe-vm';
	import ScoreProbeView from './ScoreProbeView.svelte';
	import StateInputsGrid from '../overview/StateInputsGrid.svelte';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const vm = $derived.by(() => buildProbeVm(ctx));

	type View = 'score' | 'inputs' | 'both';
	let view = $state<View>('score');

	// Annotation prefix matches the page-level `${name}:` convention so
	// /admin/debug/[name] Export/Clear can find probe-row annotations
	// alongside the other tabs' marks.
	const annPrefix = $derived(`${name}:probe`);

	const showScore = $derived(view === 'score' || view === 'both');
	const showInputs = $derived(view === 'inputs' || view === 'both');
</script>

{#snippet sectionHeader(title: string, subtitle: string)}
	<div
		class="text-surface-700-200 flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		{title}
		<span class="text-surface-500-400 font-normal normal-case">{subtitle}</span>
	</div>
{/snippet}

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center gap-3">
		<SegmentedControl value={view} onValueChange={(d) => (view = d.value as View)}>
			<SegmentedControl.Indicator />
			<SegmentedControl.Item value="score">
				<SegmentedControl.ItemText>Score ({vm.scoreEntries.length})</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
			<SegmentedControl.Item value="inputs">
				<SegmentedControl.ItemText>Inputs ({vm.stateInputEntries.length})</SegmentedControl.ItemText
				>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
			<SegmentedControl.Item value="both">
				<SegmentedControl.ItemText>Both</SegmentedControl.ItemText>
				<SegmentedControl.ItemHiddenInput />
			</SegmentedControl.Item>
		</SegmentedControl>
		<span class="text-surface-500-400 text-xs">
			plugin-reported diagnostics — values change as the reader observes new memory
		</span>
	</div>

	{#if showScore}
		<section class="flex flex-col gap-2">
			{@render sectionHeader('Score probe', 'candidate addresses the reader is sampling')}
			<div class="card preset-tonal p-3">
				<ScoreProbeView entries={vm.scoreEntries} annotationPrefix={annPrefix} />
			</div>
		</section>
	{/if}

	{#if showInputs}
		<section class="flex flex-col gap-2">
			{@render sectionHeader('State inputs', 'per-tick plugin classification inputs')}
			<StateInputsGrid showHeader={false} stateInputs={vm.stateInputs} />
		</section>
	{/if}
</div>
