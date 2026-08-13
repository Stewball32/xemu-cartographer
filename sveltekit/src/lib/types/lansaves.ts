// Types mirroring internal/halosave + internal/pocketbase/routes/lansaves.
// The LAN-saves endpoints generate Halo CE / Halo 2 UDATA save files
// (gametype variants, H2 player profiles) by template-patching real samples.

export type SaveTitle = 'ce' | 'h2';
export type SaveKind = 'gametype' | 'profile';
export type DownloadFormat = 'tar' | 'zip' | 'payload' | 'savemeta';

/** Flat spec consumed by the /build and /download endpoints. Mirrors
 * halosave.BuildRequest — friendly CE gametype settings (the backend converts to
 * raw bytes) plus the H2 appearance map. */
export interface BuildRequest {
	title: SaveTitle;
	kind: SaveKind;
	name: string;
	internal_name?: string;
	dir_name?: string;

	// CE gametype — friendly settings (2026-08-07 live-verified map)
	engine?: string;
	teams?: boolean;

	// options bitfield toggles
	radar?: boolean;
	friend_indicators?: boolean;
	infinite_grenades?: boolean;
	shields_off?: boolean;
	invisible_players?: boolean;
	generic_equipment?: boolean;
	options?: number; // raw override

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
	engine_union?: number; // raw override

	// engine_union rule toggles (engine-specific)
	death_bonus_off?: boolean;
	kill_penalty_off?: boolean;
	kill_in_order?: boolean;
	assault?: boolean;
	flag_must_reset?: boolean;
	flag_at_home?: boolean;
	moving_hill?: boolean;
	random_start?: boolean;
	race_any_order?: boolean;

	// engine scratch
	ctf_single_flag_minutes?: number;
	oddball_speed?: number;
	oddball_trait_with?: number;
	oddball_trait_without?: number;
	oddball_ball_type?: number;
	ball_spawn_count?: number;
	race_scoring?: number;

	// H2 profile appearance/controller bytes, keyed by H2 field key
	appearance?: Record<string, number>;

	// CE profile — armor color, controller presets, and the 9 advanced controls
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

/** One value of an enum field in the CE gametype schema. */
export interface CEEnumOption {
	value: number;
	label: string;
}

export type CEFieldKind = 'bool' | 'enum' | 'int' | 'float' | 'seconds' | 'minutes';

/** One editable CE gametype setting, as described by the backend schema. The
 * `key` matches a BuildRequest field. */
export interface CEField {
	key: keyof BuildRequest & string;
	label: string;
	kind: CEFieldKind;
	section: string;
	engines?: string[]; // undefined = all engines
	options?: CEEnumOption[];
	min?: number;
	max?: number;
	step?: number;
	default?: number;
	unit?: string;
	help?: string;
}

export interface CESection {
	id: string;
	label: string;
}

export interface SaveFileMeta {
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

/** Response from POST/GET /build — metadata + re-parsed payload, no bytes. */
export interface BuildResponse {
	title: SaveTitle;
	kind: SaveKind;
	title_id: string;
	dir_name: string;
	fatx_dir: string;
	files: SaveFileMeta[];
	digest: DigestStatus;
	parsed: Record<string, unknown>;
	warnings?: string[];
	total_bytes: number;
	footprint_bytes: number;
	fatx_cluster: number;
}

export interface H2AppearanceField {
	offset: number;
	key: string;
	label: string;
}

export interface LanMeta {
	titles: {
		id: SaveTitle;
		label: string;
		title_id: string;
		kinds: SaveKind[];
		note?: string;
	}[];
	ce_engines: string[];
	ce_gametype_fields: CEField[];
	ce_gametype_sections: CESection[];
	ce_profile_fields: CEField[];
	ce_profile_sections: CESection[];
	ce_score_units: Record<string, string>;
	h2_appearance: H2AppearanceField[];
	h2_gametype_mode: string;
	fatx_cluster: number;
	digest_resolved: boolean;
	digest_note: string;
	formats: DownloadFormat[];
}
