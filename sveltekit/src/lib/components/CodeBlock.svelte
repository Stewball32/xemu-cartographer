<script lang="ts">
	import Copy from '@lucide/svelte/icons/copy';
	import Check from '@lucide/svelte/icons/check';

	interface Props {
		code: string;
		classes?: string;
		preClasses?: string;
		copyable?: boolean;
	}

	const {
		code,
		classes = '',
		preClasses = 'p-4 text-xs font-mono whitespace-pre overflow-x-auto',
		copyable = true
	}: Props = $props();

	let copied = $state(false);
	async function copy() {
		try {
			await navigator.clipboard.writeText(code);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch (err) {
			console.error('clipboard write failed', err);
		}
	}
</script>

<div class="relative {classes}">
	<pre class="preset-tonal-surface rounded-container {preClasses}">{code}</pre>
	{#if copyable}
		<button
			type="button"
			onclick={copy}
			aria-label={copied ? 'Copied' : 'Copy to clipboard'}
			class="btn-icon preset-tonal-surface absolute top-2 right-2 size-8"
		>
			{#if copied}
				<Check class="size-4" />
			{:else}
				<Copy class="size-4" />
			{/if}
		</button>
	{/if}
</div>
