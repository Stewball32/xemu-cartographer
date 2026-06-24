// Types mirroring internal/halosave + internal/pocketbase/routes/lansaves.
// The LAN-saves endpoints generate Halo CE / Halo 2 UDATA save files
// (gametype variants, H2 player profiles) by template-patching real samples.

export type SaveTitle = 'ce' | 'h2';
export type SaveKind = 'gametype' | 'profile';
export type DownloadFormat = 'tar' | 'zip' | 'payload' | 'savemeta';

/** Flat spec consumed by the /build and /download endpoints. */
export interface BuildRequest {
	title: SaveTitle;
	kind: SaveKind;
	name: string;
	internal_name?: string;
	dir_name?: string;

	// CE gametype (raw mapped fields + friendly helpers)
	engine?: string;
	teams?: boolean;
	radar?: boolean;
	options?: number;
	scoring_subtype?: number;
	time_limit?: number;
	time_minutes?: number;
	time_limit2?: number;
	score_limit?: number;
	option2?: number;
	respawn?: number;
	engine_union?: number;

	// H2 profile appearance/controller bytes, keyed by H2 field key
	appearance?: Record<string, number>;
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
	h2_appearance: H2AppearanceField[];
	h2_gametype_mode: string;
	fatx_cluster: number;
	digest_resolved: boolean;
	digest_note: string;
	formats: DownloadFormat[];
}
