<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { TriangleAlertIcon, HomeIcon, ArrowLeftIcon } from '@lucide/svelte';

	const statusMessages: Record<number, string> = {
		400: 'The request was invalid or malformed.',
		401: 'You need to be logged in to access this page.',
		403: "You don't have permission to access this page.",
		404: "The page you're looking for doesn't exist or has been moved.",
		500: 'Something went wrong on our end. Please try again later.'
	};

	let status = $derived(page.status);
	let message = $derived(
		page.error?.message || statusMessages[status] || 'An unexpected error occurred.'
	);
</script>

<div class="flex min-h-[70vh] items-center justify-center">
	<div class="max-w-md space-y-6 text-center">
		<TriangleAlertIcon class="mx-auto size-16 text-error-500" />
		<h1 class="h1 font-bold">{status}</h1>
		<p class="text-lg opacity-70">{message}</p>
		<div class="flex justify-center gap-4">
			<button class="btn preset-tonal" onclick={() => history.back()}>
				<ArrowLeftIcon class="size-4" />
				<span>Go Back</span>
			</button>
			<a href={resolve('/')} class="btn preset-filled">
				<HomeIcon class="size-4" />
				<span>Home</span>
			</a>
		</div>
	</div>
</div>
