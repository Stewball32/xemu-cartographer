<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
	import { onDestroy } from 'svelte';
	import { CartographerFeed } from '$lib/overlay/cartographer.svelte.js';
	import { layouts, viewportCenters } from '$lib/overlay/themes.js';
	import PlayerCard from '$lib/overlay/PlayerCard.svelte';
	import RespawnRing from '$lib/overlay/RespawnRing.svelte';

	let { data } = $props();

	const feed = new CartographerFeed(data.ws, { nameOverrides: data.names });
	onDestroy(() => feed.destroy());

	const anchors = $derived(layouts[data.layout] ?? layouts[1]);
	const centers = $derived(viewportCenters[data.layout] ?? viewportCenters[1]);
	const players = $derived(feed.players.slice(0, (layouts[data.layout] ?? layouts[1]).length));

	const pos = (a) =>
		`left:${a.left ?? 'auto'}; top:${a.top ?? 'auto'}; bottom:${a.bottom ?? 'auto'}; transform:${a.tf ?? 'none'};`;
</script>

<svelte:head>
	<title>Norcal Halo — POV overlay</title>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Ultra&family=Inter:wght@400;500;600;700&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<div class="canvas">
	{#each players as player, i (player.slot ?? i)}
		<div class="anchor" style={pos(anchors[i])}>
			<PlayerCard {player} scale={anchors[i].scale ?? 1} origin={anchors[i].origin ?? 'center'} />
		</div>
		{#if player.alive === false && (player.respawn ?? 0) > 0}
			<div class="ring-anchor" style="left:{centers[i].x}px; top:{centers[i].y}px">
				<RespawnRing
					seconds={player.respawn}
					max={player.respawnMax ?? 5}
					theme={player.team ?? 'ffa'}
				/>
			</div>
		{/if}
	{/each}
</div>

<style>
	/* OBS browser source: 1440x1080, transparent. */
	:global(html, body) {
		margin: 0;
		background: transparent;
		overflow: hidden;
	}
	.canvas {
		position: relative;
		width: 1440px;
		height: 1080px;
	}
	.anchor {
		position: absolute;
		width: 700px;
		height: 64px;
	}
	.ring-anchor {
		position: absolute;
		transform: translate(-50%, -50%);
	}
</style>
