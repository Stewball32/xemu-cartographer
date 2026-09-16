<script lang="ts">
	// Studio "upstream disconnected" banner (DESIGN-STEP8 §12): polls
	// GET /api/admin/scraper/upstream every 5 s while mounted and shows one
	// line when the league runs in wire mode and its daemon stream is down or
	// its feed token was refused. Renders nothing in-process.
	import { onDestroy, onMount } from 'svelte';
	import { TriangleAlertIcon } from '@lucide/svelte';
	import { adminGet } from '$lib/utils/admin-api';
	import { upstreamBannerText } from '$lib/utils/upstream-banner';
	import type { UpstreamStatus } from '$lib/types/scraper';

	let { intervalMs = 5000 }: { intervalMs?: number } = $props();

	let status = $state<UpstreamStatus | null>(null);
	let timer: ReturnType<typeof setInterval> | null = null;
	const text = $derived(upstreamBannerText(status));

	async function poll() {
		try {
			status = await adminGet<UpstreamStatus>('scraper/upstream');
		} catch (err) {
			// The route answers in both modes; a failure here is the league
			// itself being unreachable, which every other poll reports too.
			console.warn('upstream status fetch failed', err);
		}
	}

	onMount(() => {
		poll();
		timer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			poll();
		}, intervalMs);
	});

	onDestroy(() => {
		if (timer !== null) {
			clearInterval(timer);
			timer = null;
		}
	});
</script>

{#if text}
	<div role="status" class="flex items-center gap-2 card preset-tonal-warning px-3 py-2 text-sm">
		<TriangleAlertIcon class="size-4 shrink-0" />
		<span>{text}</span>
	</div>
{/if}
