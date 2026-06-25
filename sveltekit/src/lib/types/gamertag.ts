// Types for the gamertag identity system — per-user CE + H2 player profiles,
// the shared organizer-curated gametype library, and the game (XBE) uploads.
// Mirror the PocketBase collections ce_profiles / h2_profiles / gametypes /
// game_titles and the saveartifact.Info JSON written by the generate-on-save
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

export interface CeProfileRecord extends RecordBase {
	user: string;
	gamertag: string;
	settings: Record<string, unknown>;
	save_bundle: string;
	save_info: SaveInfo | DeferredInfo | null;
}

export interface H2ProfileRecord extends RecordBase {
	user: string;
	gamertag: string;
	appearance: Record<string, number>;
	save_bundle: string;
	save_info: SaveInfo | null;
}

export interface GametypeSettings {
	teams?: boolean;
	radar?: boolean;
	score_limit?: number;
	time_minutes?: number;
	time_limit?: number;
	time_limit2?: number;
	options?: number;
	scoring_subtype?: number;
	option2?: number;
	respawn?: number;
	engine_union?: number;
}

export interface UserRef {
	id: string;
	username: string;
}

export interface GametypeRecord extends RecordBase {
	title: 'ce' | 'h2';
	engine: string;
	name: string;
	settings: GametypeSettings;
	save_bundle: string;
	save_info: SaveInfo | null;
	created_by: string;
	expand?: { created_by?: UserRef };
}

export interface GameTitleRecord extends RecordBase {
	name: string;
	description: string;
	file: string;
	created_by: string;
	expand?: { created_by?: UserRef };
}
