<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import { XIcon, DownloadIcon, HeartIcon } from '@lucide/svelte';

	interface Photo {
		id: number;
		title: string;
		photographer: string;
		src: string;
		category: string;
	}

	const categories = ['All', 'Nature', 'Urban', 'Abstract', 'Portrait'];
	let activeCat = $state('All');

	const photos: Photo[] = Array.from({ length: 12 }, (_, i) => {
		const cats = ['Nature', 'Urban', 'Abstract', 'Portrait'];
		const titles = [
			'Mountain Dawn',
			'City Lights',
			'Color Study',
			'The Gaze',
			'Forest Path',
			'Neon Alley',
			'Fluid Forms',
			'Quiet Moment',
			'Alpine Lake',
			'Rooftop View',
			'Pattern No. 7',
			'Window Light'
		];
		const names = ['A. Rivera', 'S. Okafor', 'L. Tanaka', 'M. Dubois'];
		return {
			id: i + 1,
			title: titles[i],
			photographer: names[i % names.length],
			src: `https://picsum.photos/seed/gallery-${i + 1}/600/400`,
			category: cats[i % 4]
		};
	});

	let filtered = $derived(
		activeCat === 'All' ? photos : photos.filter((p) => p.category === activeCat)
	);
</script>

<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<div>
		<h1 class="h2">Gallery</h1>
		<p class="text-sm opacity-70">Click any photo to open the lightbox.</p>
	</div>
	<div class="flex flex-wrap gap-2">
		{#each categories as cat (cat)}
			<button
				class="chip {activeCat === cat
					? 'preset-filled-primary-500'
					: 'preset-outlined-surface-200-800'}"
				onclick={() => (activeCat = cat)}
			>
				{cat}
			</button>
		{/each}
	</div>
</div>

<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
	{#each filtered as photo (photo.id)}
		<Dialog>
			<Dialog.Trigger class="group block overflow-hidden card rounded-lg p-0 text-left card-hover">
				<div class="aspect-square overflow-hidden">
					<img
						src={photo.src}
						alt={photo.title}
						class="size-full object-cover transition group-hover:scale-105"
						loading="lazy"
					/>
				</div>
				<div class="p-3">
					<p class="truncate text-sm font-semibold">{photo.title}</p>
					<p class="truncate text-xs opacity-60">{photo.photographer}</p>
				</div>
			</Dialog.Trigger>

			<Portal>
				<Dialog.Backdrop class="fixed inset-0 z-50 bg-surface-950/80 backdrop-blur-sm" />
				<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
					<Dialog.Content class="max-w-3xl overflow-hidden card p-0 shadow-xl">
						<div class="relative">
							<img src={photo.src} alt={photo.title} class="w-full object-contain" />
							<Dialog.CloseTrigger
								class="absolute top-3 right-3 btn-icon btn-icon-sm preset-filled"
								aria-label="Close"
							>
								<XIcon class="size-4" />
							</Dialog.CloseTrigger>
						</div>
						<div class="flex items-start justify-between gap-4 p-4">
							<div>
								<Dialog.Title class="h4">{photo.title}</Dialog.Title>
								<Dialog.Description class="text-sm opacity-70">
									By {photo.photographer} · {photo.category}
								</Dialog.Description>
							</div>
							<div class="flex gap-2">
								<button class="btn-icon preset-tonal" aria-label="Like">
									<HeartIcon class="size-4" />
								</button>
								<button class="btn-icon preset-tonal" aria-label="Download">
									<DownloadIcon class="size-4" />
								</button>
							</div>
						</div>
					</Dialog.Content>
				</Dialog.Positioner>
			</Portal>
		</Dialog>
	{/each}
</div>
