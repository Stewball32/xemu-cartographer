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

// MCC's post-game Spartan customization stance set — the COMPLETE set decoded
// from the MCC UE4 shell's AnimSequence assets and retargeted onto the H2 Mark VI
// rig: 11 UNARMED stances + 10 ARMED stances that hold a battle rifle prop
// (the MCC `SM_H2_BattleRifle`, attached to the anim's `weapon` bone). 'default'
// is MCC's unlocked-by-default armory idle. The `Pose_Finger` asset is excluded
// (it's a weapon trigger-finger helper, not a selectable stance).
export const SPARTAN_POSES = [
	'default',
	'at_ease',
	'salute',
	'crossed_arms',
	'hands_on_hips',
	'double_flex',
	'kneel',
	'look_back',
	'peace_sign',
	'meditation',
	'yoga',
	// armed (battle-rifle) stances
	'rifle_ready',
	'rifle_at_side',
	'rifle_on_shoulder',
	'rifle_back',
	'rifle_kneel',
	'rifle_sit',
	'rifle_salute',
	'rifle_hero_pose',
	'rifle_buff',
	'rifle_buff_back'
] as const;
export type SpartanPose = (typeof SPARTAN_POSES)[number];

/** The armed subset — these render with the battle-rifle prop. */
export const SPARTAN_ARMED_POSES = [
	'rifle_ready',
	'rifle_at_side',
	'rifle_on_shoulder',
	'rifle_back',
	'rifle_kneel',
	'rifle_sit',
	'rifle_salute',
	'rifle_hero_pose',
	'rifle_buff',
	'rifle_buff_back'
] as const satisfies readonly SpartanPose[];

/** True when the pose holds the battle rifle (armed stance). */
export function isArmedPose(pose: SpartanPose): boolean {
	return (SPARTAN_ARMED_POSES as readonly string[]).includes(pose);
}

/** Human-readable labels for the pose selector (matches MCC's naming). */
export const SPARTAN_POSE_LABELS: Record<SpartanPose, string> = {
	default: 'Default',
	at_ease: 'At Ease',
	salute: 'Salute',
	crossed_arms: 'Crossed Arms',
	hands_on_hips: 'Hands on Hips',
	double_flex: 'Double Flex',
	kneel: 'Kneel',
	look_back: 'Look Back',
	peace_sign: 'Peace Sign',
	meditation: 'Meditation',
	yoga: 'Yoga',
	rifle_ready: 'Rifle Ready',
	rifle_at_side: 'Rifle at Side',
	rifle_on_shoulder: 'Rifle on Shoulder',
	rifle_back: 'Rifle on Back',
	rifle_kneel: 'Rifle Kneel',
	rifle_sit: 'Rifle Sit',
	rifle_salute: 'Rifle Salute',
	rifle_hero_pose: 'Rifle Hero',
	rifle_buff: 'Rifle Flex',
	rifle_buff_back: 'Rifle Flex (Back)'
};

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
	const img = (suffix: string) => `${base}/${pose}${suffix}.png`;
	return (
		maskTag(p, img('_p')) +
		maskTag(s, img('_s')) +
		`<image href="${img('')}" x="0" y="0" width="100" height="100" preserveAspectRatio="xMidYMid meet"/>` +
		`<rect x="0" y="0" width="100" height="100" fill="${primary}" mask="url(#${p})" style="mix-blend-mode:multiply"/>` +
		`<rect x="0" y="0" width="100" height="100" fill="${secondary}" mask="url(#${s})" style="mix-blend-mode:multiply"/>`
	);
}
