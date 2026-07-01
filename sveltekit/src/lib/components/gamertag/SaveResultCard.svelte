<script lang="ts">
	import { DownloadIcon, FileIcon, InfoIcon, AlertTriangleIcon } from '@lucide/svelte';
	import { downloadRecordFile, formatBytes, type FileRecord } from '$lib/utils/gamertag';
	import { toaster } from '$lib/stores/toaster';
	import { isDeferred, type SaveInfo, type DeferredInfo } from '$lib/types/gamertag';

	let {
		record,
		info,
		downloadName,
		field = 'save_bundle'
	}: {
		// PB record with the file field; null until first save.
		record: FileRecord | null;
		info: SaveInfo | DeferredInfo | null;
		downloadName: string;
		field?: string;
	} = $props();

	let busy = $state(false);
	const hasFile = $derived(!!record && !!(record as Record<string, unknown>)[field]);

	async function download() {
		if (!record) return;
		busy = true;
		try {
			await downloadRecordFile(record, field, downloadName);
			toaster.success({ title: 'Downloaded', description: downloadName });
		} catch (e) {
			toaster.error({
				title: 'Download failed',
				description: e instanceof Error ? e.message : String(e)
			});
		} finally {
			busy = false;
		}
	}
</script>

<div class="space-y-3 card preset-tonal p-4 text-sm">
	{#if !record}
		<p class="text-surface-600-400">Save your gamertag to generate the file.</p>
	{:else if isDeferred(info)}
		<div class="flex items-start gap-2">
			<InfoIcon class="mt-0.5 size-4 shrink-0 text-warning-500" />
			<div>
				<p class="font-medium">No save file generated yet</p>
				<p class="mt-1 text-xs opacity-80">{(info as DeferredInfo).note}</p>
			</div>
		</div>
	{:else if info}
		{@const si = info as SaveInfo}
		<div class="flex items-center justify-between gap-2">
			<div>
				<p class="font-medium">Generated save</p>
				<p class="font-mono text-xs text-surface-600-400">{si.fatx_dir}</p>
			</div>
			<button class="btn preset-filled btn-sm" onclick={download} disabled={busy || !hasFile}>
				<DownloadIcon class="size-4" />
				<span>Download</span>
			</button>
		</div>

		<ul class="space-y-1">
			{#each si.files as f (f.name)}
				<li class="flex items-center gap-2 text-xs">
					<FileIcon class="size-3 shrink-0 opacity-60" />
					<span class="font-mono">{f.name}</span>
					<span class="opacity-60">{formatBytes(f.size)}</span>
					<span class="ml-auto truncate font-mono opacity-40" title={f.sha1}
						>{f.sha1.slice(0, 12)}…</span
					>
				</li>
			{/each}
		</ul>

		{#if si.digest?.edited && !si.digest?.resolved}
			<div class="flex items-start gap-2 text-xs text-warning-700-300">
				<AlertTriangleIcon class="mt-0.5 size-3.5 shrink-0" />
				<span>
					Edited-settings file: the 20-byte content digest is preserved from the template (stale).
					Whether Halo re-verifies it on load is unconfirmed — see the LAN-hub notes.
				</span>
			</div>
		{/if}
	{/if}
</div>
