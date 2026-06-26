// Typed accessors over halo-armor-palettes.json — the authoritative, game-exact
// armor/player color palettes for Halo: CE (Xbox) and Halo 2 (Xbox).
//
//   • CE  — c20.reclaimers.net hard-coded multiplayer armor colors. Index order is
//           the "c20 palette order", i.e. the blam.sav profile color enum (u32 @ 0x18).
//   • H2  — extracted from Halo 2 (Xbox) mainmenu.map globals player-color table
//           (float32 RGB, value*255), confirmed byte-identical in shared.map.
//           Index order is e_player_color, matching profile bytes 0x118..0x11B.
//           The SAME 18 colors are used for armor AND emblem colors.
//
// See halo-armor-palettes.json `meta` for full provenance.
import data from './halo-armor-palettes.json';

export interface ArmorColor {
	index: number;
	name: string;
	hex: string;
	rgb: [number, number, number];
	/** Alternate name MCC used at launch (CE only, where applicable). */
	mccName?: string;
}

type RawColor = { name: string; hex: string; rgb: number[]; mccName?: string };

function toArray(obj: Record<string, RawColor>): ArmorColor[] {
	return Object.keys(obj)
		.map((k) => Number(k))
		.sort((a, b) => a - b)
		.map((i) => {
			const c = obj[String(i)];
			const out: ArmorColor = {
				index: i,
				name: c.name,
				hex: c.hex,
				rgb: [c.rgb[0], c.rgb[1], c.rgb[2]]
			};
			if (c.mccName) out.mccName = c.mccName;
			return out;
		});
}

/** Halo: CE armor colors (18), index = blam.sav 0x18 enum / c20 palette order. */
export const CE_ARMOR_COLORS: ArmorColor[] = toArray(data.ce as Record<string, RawColor>);

/** Halo 2 armor/emblem colors (18), index = e_player_color / profile 0x118..0x11B. */
export const H2_ARMOR_COLORS: ArmorColor[] = toArray(data.h2 as Record<string, RawColor>);

/** Provenance + source metadata for both palettes. */
export const PALETTE_META = data.meta;
