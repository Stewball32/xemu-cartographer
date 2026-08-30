<script lang="ts">
	// Discord-style drop / drag / zoom reframe stage. The ONE drop zone of its
	// host surface: drop (or click to browse) any image, drag to pan, wheel or
	// the slider to zoom; the parent calls exportBlob() to get the fixed-size
	// crop (e.g. 600×100 banner, square map graphic) on save.
	//
	// An existing crop (src) displays as-is — reframing starts when a NEW image
	// lands. dirty() tells the parent whether there's anything to export.
	//
	// `overlay` renders INSIDE the stage, pointer-events-none — the Nameplates
	// editor uses it to draw the real plate chrome over the live crop so what
	// you line up is exactly what the overlay draws.
	import { UploadIcon } from '@lucide/svelte';
	import type { Snippet } from 'svelte';

	let {
		src = '',
		targetW,
		targetH,
		placeholder = 'Drop an image — or browse',
		radius = '8px',
		onimage,
		overlay
	}: {
		src?: string;
		targetW: number;
		targetH: number;
		placeholder?: string;
		radius?: string;
		/** fires when a NEW image lands (the parent's dirty tracking). */
		onimage?: () => void;
		/** rendered inside the stage, above the crop, pointer-events-none. */
		overlay?: Snippet;
	} = $props();

	let img = $state<HTMLImageElement | null>(null);
	let zoom = $state(1); // 1 = cover fit
	let panX = $state(0.5); // 0..1 — fraction of the pannable overflow
	let panY = $state(0.5);
	let dragOver = $state(false);
	let stageEl = $state<HTMLDivElement | null>(null);
	let stageW = $state(0);
	let stageH = $state(0);
	let fileInput: HTMLInputElement;

	export function dirty(): boolean {
		return img !== null;
	}

	export function reset(): void {
		img = null;
		zoom = 1;
		panX = panY = 0.5;
	}

	/** The fixed-size crop as a PNG blob, or null when no new image landed. */
	export async function exportBlob(): Promise<Blob | null> {
		if (!img) return null;
		const canvas = document.createElement('canvas');
		canvas.width = targetW;
		canvas.height = targetH;
		const ctx = canvas.getContext('2d')!;
		const g = geometry(targetW, targetH);
		ctx.drawImage(img, g.x, g.y, g.w, g.h);
		return await new Promise((resolve) => canvas.toBlob(resolve, 'image/png'));
	}

	/** Draw geometry for a frame of (fw, fh): cover-fit × zoom, panned. */
	function geometry(fw: number, fh: number) {
		const iw = img!.naturalWidth;
		const ih = img!.naturalHeight;
		const scale = Math.max(fw / iw, fh / ih) * zoom;
		const w = iw * scale;
		const h = ih * scale;
		return { x: -(w - fw) * panX, y: -(h - fh) * panY, w, h };
	}

	const stageStyle = $derived.by(() => {
		if (!img || !stageW || !stageH) return '';
		const g = geometry(stageW, stageH);
		return (
			`background-image:url(${img.src});background-repeat:no-repeat;` +
			`background-size:${g.w}px ${g.h}px;background-position:${g.x}px ${g.y}px`
		);
	});

	function acceptFile(f: File | undefined | null) {
		if (!f || !f.type.startsWith('image/')) return;
		const url = URL.createObjectURL(f);
		const el = new Image();
		el.onload = () => {
			img = el;
			zoom = 1;
			panX = panY = 0.5;
			onimage?.();
		};
		el.src = url;
	}

	// Pointer drag → pan (fractions of the current overflow, so zooming keeps
	// the framing stable).
	let dragging = false;
	let lastX = 0;
	let lastY = 0;
	function onPointerDown(e: PointerEvent) {
		if (!img) return;
		dragging = true;
		lastX = e.clientX;
		lastY = e.clientY;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function onPointerMove(e: PointerEvent) {
		if (!dragging || !img || !stageW || !stageH) return;
		const g = geometry(stageW, stageH);
		const overX = g.w - stageW;
		const overY = g.h - stageH;
		if (overX > 0) panX = clamp01(panX - (e.clientX - lastX) / overX);
		if (overY > 0) panY = clamp01(panY - (e.clientY - lastY) / overY);
		lastX = e.clientX;
		lastY = e.clientY;
	}
	function onPointerUp() {
		dragging = false;
	}
	function onWheel(e: WheelEvent) {
		if (!img) return;
		e.preventDefault();
		zoom = Math.min(4, Math.max(1, zoom * (e.deltaY < 0 ? 1.08 : 1 / 1.08)));
	}
	const clamp01 = (v: number) => Math.min(1, Math.max(0, v));
</script>

<div class="flex flex-col gap-2">
	<div
		bind:this={stageEl}
		bind:clientWidth={stageW}
		bind:clientHeight={stageH}
		class="relative w-full overflow-hidden border bg-black/20 select-none
			{dragOver ? 'border-primary-500/60' : 'border-surface-500/30'}
			{img ? 'cursor-grab active:cursor-grabbing' : 'cursor-pointer'}"
		style="aspect-ratio:{targetW}/{targetH}; border-radius:{radius}; {stageStyle}"
		role="button"
		tabindex="0"
		aria-label={img ? 'Drag to reframe the crop' : placeholder}
		ondragover={(e) => {
			e.preventDefault();
			dragOver = true;
		}}
		ondragleave={() => (dragOver = false)}
		ondrop={(e) => {
			e.preventDefault();
			dragOver = false;
			acceptFile(e.dataTransfer?.files?.[0]);
		}}
		onpointerdown={onPointerDown}
		onpointermove={onPointerMove}
		onpointerup={onPointerUp}
		onwheel={onWheel}
		onclick={() => {
			if (!img) fileInput.click();
		}}
		onkeydown={(e) => {
			if ((e.key === 'Enter' || e.key === ' ') && !img) fileInput.click();
		}}
	>
		{#if !img}
			{#if src}
				<img {src} alt="" class="absolute inset-0 h-full w-full object-cover" draggable="false" />
			{:else}
				<div
					class="absolute inset-0 flex flex-col items-center justify-center gap-1.5 border border-dashed border-surface-500/40 text-center"
					style="border-radius:{radius}"
				>
					<UploadIcon class="size-5 opacity-60" />
					<span class="px-4 text-xs opacity-70">{placeholder}</span>
				</div>
			{/if}
		{/if}
		{#if overlay}
			<div class="pointer-events-none absolute inset-0">
				{@render overlay()}
			</div>
		{/if}
	</div>
	<div class="flex items-center gap-2">
		<button class="btn preset-tonal btn-sm" onclick={() => fileInput.click()}>
			<UploadIcon class="size-4" /><span>{img || src ? 'Replace image' : 'Browse'}</span>
		</button>
		{#if img}
			<input
				type="range"
				class="max-w-40 flex-1"
				min="1"
				max="4"
				step="0.01"
				bind:value={zoom}
				aria-label="Zoom"
			/>
			<span class="text-xs tabular-nums opacity-60">{zoom.toFixed(2)}×</span>
		{/if}
	</div>
	<input
		bind:this={fileInput}
		type="file"
		accept="image/*"
		class="hidden"
		onchange={(e) => acceptFile((e.currentTarget as HTMLInputElement).files?.[0])}
	/>
</div>
