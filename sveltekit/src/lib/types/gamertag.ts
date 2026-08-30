// Types for the gamertag identity system — per-user CE + H2 player profiles,
// the shared organizer-curated gametype library, and the game (XBE) uploads.
// Mirror the PocketBase collections ce_profiles / h2_profiles / gametypes /
// the saveartifact.Info JSON written by the generate-on-save
// hooks.

export interface SaveInfoFile {
	name: string;
	size: number;
	sha1: string;
}

export interface DigestStatus {
	mode: string;
	resolved: boolean;
	edited: boolean;
	note: string;
}

/** Generator metadata stored on a record's save_info when a file was built. */
export interface SaveInfo {
	title: string;
	kind: string;
	title_id: string;
	dir_name: string;
	fatx_dir: string;
	files: SaveInfoFile[];
	digest: DigestStatus;
	total_bytes: number;
	warnings?: string[];
}

/** save_info shape for a record whose generation is deferred (CE profiles). */
export interface DeferredInfo {
	deferred: true;
	gamertag: string;
	note: string;
}

export function isDeferred(info: unknown): info is DeferredInfo {
	return !!info && typeof info === 'object' && (info as { deferred?: unknown }).deferred === true;
}

/** Fields every PB record exposes that file URLs need. */
interface RecordBase {
	id: string;
	collectionId: string;
	collectionName: string;
	created: string;
	updated: string;
}

// CE player profile (blam.sav) editable fields — the 2026-08-07 live-verified
// surface. Color is the armor enum (18 colors); button/thumbstick are the
// in-game presets; the rest are the nine Advanced Controls. Mirrors
// saveartifact.CEProfileSettings; keys match the ce_profile_fields schema.
export interface CeProfileSettings {
	color?: number;
	button?: number;
	thumbstick?: number;
	h_sens?: number;
	v_mult?: number;
	invert?: boolean;
	vibration?: boolean;
	rs_deadzone?: number;
	ls_deadzone?: number;
	outer_deadzone?: number;
	deadzone_type?: number;
	response?: number;
}

// Profiles do NOT store the gamertag — it lives on the user record
// (users.gamertag) and is resolved server-side at generation time. CE is a full
// profile (parallel to H2): `settings` holds color + control presets; the
// generated blam.sav is signed.
export interface CeProfileRecord extends RecordBase {
	user: string;
	settings: CeProfileSettings;
	save_bundle: string;
	save_info: SaveInfo | null;
}

export interface H2ProfileRecord extends RecordBase {
	user: string;
	appearance: Record<string, number>;
	save_bundle: string;
	save_info: SaveInfo | null;
}

// Friendly gametype settings persisted in gametypes.settings — the 2026-08-07
// live-verified CE surface. Mirrors saveartifact.GametypeSettings; the generator
// converts these to raw bytes. (Supersedes the pre-2026-08-08 shape whose
// `time_minutes` actually wrote respawn time.)
export interface GametypeSettings {
	teams?: boolean;
	radar?: boolean;
	friend_indicators?: boolean;
	infinite_grenades?: boolean;
	shields_off?: boolean;
	invisible_players?: boolean;
	generic_equipment?: boolean;
	objectives_indicator?: number;
	odd_man_out?: boolean;
	respawn_seconds?: number;
	respawn_growth_seconds?: number;
	suicide_seconds?: number;
	lives?: number;
	max_health?: number;
	score_limit?: number;
	weapon_set?: number;
	nhe_toggles?: number;
	death_bonus_off?: boolean;
	kill_penalty_off?: boolean;
	kill_in_order?: boolean;
	assault?: boolean;
	flag_must_reset?: boolean;
	flag_at_home?: boolean;
	moving_hill?: boolean;
	random_start?: boolean;
	race_any_order?: boolean;
	ctf_single_flag_minutes?: number;
	oddball_speed?: number;
	oddball_trait_with?: number;
	oddball_trait_without?: number;
	oddball_ball_type?: number;
	ball_spawn_count?: number;
	race_scoring?: number;
	options?: number;
	engine_union?: number;
}

export interface UserRef {
	id: string;
	username: string;
}

export interface GametypeRecord extends RecordBase {
	title: 'ce' | 'h2';
	engine: string;
	/** Library name — what rulesets and the organizer list show. */
	name: string;
	/** In-game name — written into the signed save, shown on the pregame lobby
	 * list (CE truncates past 11 chars there). "" on pre-redesign rows. */
	display_name: string;
	settings: GametypeSettings;
	save_bundle: string;
	save_info: SaveInfo | null;
	created_by: string;
	expand?: { created_by?: UserRef };
}
