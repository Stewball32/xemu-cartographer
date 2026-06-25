// Client helpers for the gamertag identity system.
import type { RecordModel } from 'pocketbase';
import pb from '$lib/pocketbase';
import { apiBaseURL } from '$lib/utils/api-base';
import { auth } from '$lib/stores/auth.svelte';
import type { H2AppearanceField } from '$lib/types/lansaves';

/** The minimal record shape the file helpers need (every PB record has these). */
export type FileRecord = Pick<RecordModel, 'id' | 'collectionId' | 'collectionName'>;

/** Resolve the caller's default gamertag (server-side identity) for pre-filling
 *  the profile editors. Returns null if none is set. */
export async function fetchDefaultGamertag(): Promise<string | null> {
	if (!auth.token) return null;
	try {
		const res = await fetch(`${apiBaseURL()}/api/me`, { headers: { Authorization: auth.token } });
		if (!res.ok) return null;
		const data = (await res.json()) as { default_gamertag?: { tag?: string } | null };
		return data.default_gamertag?.tag ?? null;
	} catch {
		return null;
	}
}

/** Trigger a browser download of a record's stored (protected) file field.
 *  Profiles/gametypes are owner/organizer-readable, so the file API needs a
 *  short-lived file token. */
export async function downloadRecordFile(
	record: FileRecord,
	field: string,
	suggestedName?: string
): Promise<void> {
	const filename = (record as Record<string, unknown>)[field] as string | undefined;
	if (!filename) throw new Error('no file on this record yet');
	const token = await pb.files.getToken();
	const url = pb.files.getURL(record as RecordModel, filename, { token, download: true });
	const a = document.createElement('a');
	a.href = url;
	if (suggestedName) a.download = suggestedName;
	document.body.appendChild(a);
	a.click();
	a.remove();
}

/** Friendlier grouping of the provisional H2 appearance/controller byte fields
 *  into Armor / Emblem / Controls sections for the profile editor. Any field
 *  the backend reports that doesn't match a known prefix falls under "Other". */
export interface AppearanceGroup {
	label: string;
	fields: H2AppearanceField[];
}

export function groupAppearanceFields(fields: H2AppearanceField[]): AppearanceGroup[] {
	const armor: H2AppearanceField[] = [];
	const emblem: H2AppearanceField[] = [];
	const controls: H2AppearanceField[] = [];
	const other: H2AppearanceField[] = [];
	for (const f of fields) {
		if (f.key.startsWith('armor')) armor.push(f);
		else if (f.key.startsWith('emblem')) emblem.push(f);
		else if (f.key.startsWith('ctrl')) controls.push(f);
		else other.push(f);
	}
	return [
		{ label: 'Armor', fields: armor },
		{ label: 'Emblem', fields: emblem },
		{ label: 'Controls', fields: controls },
		{ label: 'Other', fields: other }
	].filter((g) => g.fields.length > 0);
}

/** Human-readable byte count for save-info display. */
export function formatBytes(n: number): string {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
	return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}
