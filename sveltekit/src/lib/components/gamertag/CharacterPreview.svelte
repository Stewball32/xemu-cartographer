<script lang="ts">
	// 2D in-game character preview: a stylized Spartan (CE / H2 Master Chief /
	// Spartan) or Elite (Dervish / Elite) bust, tinted by the chosen ARMOR color
	// the way the in-game biped is, with the H2 emblem applied as a chest decal.
	// Deliberately a flat 2D read of the character — not a model render.
	import EmblemPreview from './EmblemPreview.svelte';
	import RawSvg from './RawSvg.svelte';
	import {
		CE_COLORS,
		H2_COLORS,
		colorHex,
		colorName,
		readEmblem,
		DEFAULT_EMBLEM,
		type Appearance
	} from '$lib/utils/emblem';

	let {
		game,
		appearance,
		colorIndex = 0,
		size = 230,
		gamertag = '',
		showName = true
	}: {
		game: 'ce' | 'h2';
		appearance?: Appearance;
		colorIndex?: number;
		size?: number;
		gamertag?: string;
		showName?: boolean;
	} = $props();

	const emblem = $derived(readEmblem(appearance ?? {}, DEFAULT_EMBLEM));

	// Primary / secondary armor hexes + the human color name for the caption.
	const primary = $derived(
		game === 'ce' ? colorHex(CE_COLORS, colorIndex) : colorHex(H2_COLORS, emblem.armorPrimary)
	);
	const secondary = $derived(
		game === 'ce' ? shade(primary, -0.45) : colorHex(H2_COLORS, emblem.armorSecondary)
	);
	const colorLabel = $derived(
		game === 'ce' ? colorName(CE_COLORS, colorIndex) : colorName(H2_COLORS, emblem.armorPrimary)
	);

	// Elite for Dervish(1) / Elite(3); Spartan otherwise. CE is always a Spartan.
	const isElite = $derived(game === 'h2' && (emblem.character === 1 || emblem.character === 3));

	function clampByte(h: string): string {
		return h.length === 4 ? '#' + h[1] + h[1] + h[2] + h[2] + h[3] + h[3] : h;
	}
	// lighten (amt>0 → toward white) / darken (amt<0 → toward black).
	function shade(hex: string, amt: number): string {
		const p = clampByte(hex);
		const to = amt < 0 ? 0 : 255;
		const t = Math.abs(amt);
		const c = [1, 3, 5].map((i) => {
			const v = parseInt(p.slice(i, i + 2), 16);
			return Math.round(v + (to - v) * t);
		});
		return '#' + c.map((v) => v.toString(16).padStart(2, '0')).join('');
	}

	const main = $derived(primary);
	const dark = $derived(shade(primary, -0.38));
	const darker = $derived(shade(primary, -0.6));
	const light = $derived(shade(primary, 0.26));
	const trim = $derived(game === 'ce' ? dark : secondary);
	const under = '#15191e';
	const visor = '#ffcf4d';
	const visorLite = '#fff3c0';

	function spartan(): string {
		return [
			// undersuit neck
			`<path d="M84,108 h32 v22 q-16,8 -32,0 Z" fill="${under}"/>`,
			// shoulders / pauldrons
			`<path d="M26,176 q2,-46 46,-52 l8,30 q-30,4 -34,34 Z" fill="${main}"/>`,
			`<path d="M174,176 q-2,-46 -46,-52 l-8,30 q30,4 34,34 Z" fill="${main}"/>`,
			`<path d="M26,176 q2,-46 46,-52 l3,11 q-34,7 -38,44 Z" fill="${trim}"/>`,
			`<path d="M174,176 q-2,-46 -46,-52 l-3,11 q34,7 38,44 Z" fill="${trim}"/>`,
			// chest plate
			`<path d="M62,128 q38,-22 76,0 l-6,84 q-32,12 -64,0 Z" fill="${main}"/>`,
			`<path d="M62,128 q38,-22 76,0 l-2,18 q-36,-18 -72,0 Z" fill="${light}"/>`,
			// ab shadow + center seam
			`<path d="M70,196 q30,10 60,0 l-3,16 q-27,9 -54,0 Z" fill="${dark}"/>`,
			`<rect x="98" y="150" width="4" height="58" fill="${darker}"/>`,
			// helmet
			`<path d="M62,86 q0,-58 38,-58 q38,0 38,58 q0,26 -38,26 q-38,0 -38,-26 Z" fill="${main}"/>`,
			// helmet top highlight
			`<path d="M70,58 q6,-26 30,-28 q-20,10 -22,34 Z" fill="${light}"/>`,
			// visor
			`<path d="M74,64 q26,-16 52,0 q2,22 -26,30 q-28,-8 -26,-30 Z" fill="${visor}"/>`,
			`<path d="M80,64 q20,-11 40,0 q-2,6 -8,9 q-12,-7 -24,0 q-6,-3 -8,-9 Z" fill="${visorLite}"/>`,
			// brow
			`<path d="M62,86 q38,18 76,0 q-4,12 -38,12 q-34,0 -38,-12 Z" fill="${dark}"/>`
		].join('');
	}

	function elite(): string {
		return [
			`<path d="M86,104 h28 v20 q-14,8 -28,0 Z" fill="${under}"/>`,
			// broad hunched shoulders
			`<path d="M20,186 q6,-54 56,-58 l6,30 q-38,6 -42,46 Z" fill="${main}"/>`,
			`<path d="M180,186 q-6,-54 -56,-58 l-6,30 q38,6 42,46 Z" fill="${main}"/>`,
			`<path d="M20,186 q6,-54 56,-58 l2,12 q-44,9 -48,58 Z" fill="${trim}"/>`,
			`<path d="M180,186 q-6,-54 -56,-58 l-2,12 q44,9 48,58 Z" fill="${trim}"/>`,
			// torso
			`<path d="M64,126 q36,-20 72,0 l-8,86 q-28,12 -56,0 Z" fill="${main}"/>`,
			`<path d="M64,126 q36,-20 72,0 l-3,16 q-33,-16 -66,0 Z" fill="${light}"/>`,
			`<rect x="98" y="150" width="4" height="58" fill="${darker}"/>`,
			// elongated domed head
			`<path d="M82,86 q-6,-60 18,-66 q24,6 18,66 q-18,14 -36,0 Z" fill="${main}"/>`,
			`<path d="M90,30 q10,-8 20,0 q-10,-4 -20,0 Z" fill="${light}"/>`,
			// split mandibles
			`<path d="M84,82 q8,18 16,20 l-2,-26 Z" fill="${dark}"/>`,
			`<path d="M116,82 q-8,18 -16,20 l2,-26 Z" fill="${dark}"/>`,
			`<path d="M92,80 l6,24 -10,-16 Z" fill="${darker}"/>`,
			`<path d="M108,80 l-6,24 10,-16 Z" fill="${darker}"/>`,
			// eyes
			`<circle cx="92" cy="60" r="3" fill="${visor}"/>`,
			`<circle cx="108" cy="60" r="3" fill="${visor}"/>`
		].join('');
	}

	const figure = $derived(isElite ? elite() : spartan());
	const charW = $derived(Math.round((size * 200) / 240));
</script>

<div class="char-wrap">
	<div class="char" style="width:{charW}px;height:{size}px;">
		<RawSvg
			inner={figure}
			viewBox="0 0 200 240"
			title="{game === 'ce' ? 'Halo: CE' : 'Halo 2'} character"
		/>
		{#if game === 'h2'}
			<div class="decal" style="width:{Math.round(charW * 0.3)}px;">
				<EmblemPreview {appearance} size={Math.round(charW * 0.3)} rounded title="Chest emblem" />
			</div>
		{/if}
	</div>
	{#if showName}
		<div class="caption">
			<span class="tag">{gamertag || '—'}</span>
			<span class="meta">{game === 'ce' ? 'Halo: CE' : 'Halo 2'} · {colorLabel}</span>
		</div>
	{/if}
</div>

<style>
	.char-wrap {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.5rem;
	}
	.char {
		position: relative;
		filter: drop-shadow(0 6px 14px rgba(0, 0, 0, 0.45));
	}
	.decal {
		position: absolute;
		left: 50%;
		top: 63%;
		transform: translate(-50%, -50%) rotate(-1deg);
		opacity: 0.96;
	}
	.caption {
		display: flex;
		flex-direction: column;
		align-items: center;
		line-height: 1.1;
	}
	.tag {
		font-weight: 700;
		font-size: 0.95rem;
		letter-spacing: 0.02em;
	}
	.meta {
		font-size: 0.7rem;
		opacity: 0.6;
	}
</style>
