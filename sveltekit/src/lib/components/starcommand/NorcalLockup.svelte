<!--
	NorcalLockup.svelte — star + stacked wordmark (SS1).
	NORCAL: Orbitron 800 / .08em / Star White. HALO: wide-tracked caps, UI blue.
	The star image is used verbatim — never recolored or filtered.

	Props:
	  star — emblem URL (star.png); null = type-only lockup
	  size — star box in px; type scales from it (default 44)
	  tag  — show the HALO line (default true)
-->
<script lang="ts">
	let {
		star = null,
		size = 44,
		tag = true
	}: { star?: string | null; size?: number; tag?: boolean } = $props();
</script>

<span class="sc-lockup" style="--sz:{size}px;">
	{#if star}<img src={star} alt="" class="star" />{/if}
	<span class="type">
		<span class="norcal">NORCAL</span>
		{#if tag}<span class="halo">HALO</span>{/if}
	</span>
</span>

<style>
	.sc-lockup {
		display: inline-flex;
		align-items: center;
		gap: calc(var(--sz) * 0.26);
	}
	.star {
		width: var(--sz);
		height: var(--sz);
		object-fit: contain;
		display: block;
	}
	.type {
		display: flex;
		flex-direction: column;
		line-height: 1;
	}
	.norcal {
		font-family: 'Orbitron', 'Oswald', sans-serif;
		font-weight: 800;
		font-size: calc(var(--sz) * 0.4);
		letter-spacing: 0.08em;
		color: var(--color-surface-50);
	}
	.halo {
		margin-top: calc(var(--sz) * 0.1);
		font-weight: 700;
		font-size: calc(var(--sz) * 0.185);
		letter-spacing: 0.85em;
		text-indent: 0.06em;
		text-transform: uppercase;
		color: var(--color-secondary-300);
	}
</style>
