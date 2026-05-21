<script lang="ts">
	// Renders the plugin's score_probe map as a sorted key/value grid. The
	// shape is free-form (Record<string, unknown> from GameReader.BuildScoreProbe)
	// so values pass through formatProbeValue — pointer-shaped numbers get
	// both decimal and hex, scalars render directly, objects JSON-stringify.

	import KvGrid, { type KvField } from '../shared/KvGrid.svelte';
	import { formatProbeValue, type ProbeEntry } from './probe-vm';

	let {
		entries,
		annotationPrefix
	}: {
		entries: ProbeEntry[];
		annotationPrefix?: string;
	} = $props();

	const fields = $derived<KvField[]>(
		entries.map(([key, value]) => ({
			key,
			value,
			format: formatProbeValue,
			annotationKey: annotationPrefix ? `${annotationPrefix}.${key}` : undefined
		}))
	);
</script>

<KvGrid {fields} emptyMessage="no probe data — plugin did not report any score_probe entries" />
