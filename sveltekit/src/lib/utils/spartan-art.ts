/**
 * Compositor for the REAL posed Halo 2 Mark VI (and Elite) renders — the 3D
 * counterpart of emblem-art.ts. Builds tintable inner-SVG from the offline
 * render passes that ship under the SvelteKit static root:
 *
 *   /spartan/<pose>.png     diffuse-lit base (the extracted Mark VI, posed)
 *   /spartan/<pose>_p.png   primary-armor coverage   (from masterchief_cc R)
 *   /spartan/<pose>_s.png   secondary-armor coverage (from masterchief_cc G)
 *
 * The passes are produced fully on Linux: tools/h2-model decodes the H2 `mode`
 * tag + `masterchief_cc` natively, Blender (headless) poses + renders. At
 * runtime we only tint flat PNGs — same idea as the emblems, in screen space:
 * fill the primary coverage with armor colour 1 and the secondary with colour 2,
 * multiplied over the diffuse so the baked panel detail/shading shows through,
 * then drop the emblem decal on the chest (see CharacterPreview).
 *
 * Mirrors emblem-art.ts's <mask><image/></mask> + filled <rect> technique so it
 * renders identically in any browser (verified headless via chromium on Linux).
 */

export const SPARTAN_ASSET_BASE = '/spartan';

export const SPARTAN_POSES = ['idle', 't_pose', 'salute', 'crouch'] as const;
export type SpartanPose = (typeof SPARTAN_POSES)[number];

function maskTag(id: string, href: string): string {
	return (
		`<mask id="${id}" maskUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">` +
		`<image href="${href}" x="0" y="0" width="100" height="100" preserveAspectRatio="xMidYMid meet"/>` +
		`</mask>`
	);
}

/**
 * Posed Mark VI tinted to the player's two armor colours. `uid` must be unique
 * per on-page instance (mask ids). The result is transparent outside the
 * rendered silhouette, so it drops onto any background like the emblem art.
 */
export function spartanTinted(
	uid: string,
	pose: SpartanPose,
	primary: string,
	secondary: string,
	base = SPARTAN_ASSET_BASE
): string {
	const p = `${uid}-p`,
		s = `${uid}-s`;
	const img = (suffix: string) =>
		`${base}/${pose}${suffix}.png`;
	return (
		maskTag(p, img('_p')) +
		maskTag(s, img('_s')) +
		`<image href="${img('')}" x="0" y="0" width="100" height="100" preserveAspectRatio="xMidYMid meet"/>` +
		`<rect x="0" y="0" width="100" height="100" fill="${primary}" mask="url(#${p})" style="mix-blend-mode:multiply"/>` +
		`<rect x="0" y="0" width="100" height="100" fill="${secondary}" mask="url(#${s})" style="mix-blend-mode:multiply"/>`
	);
}
