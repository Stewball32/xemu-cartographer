import pb from '$lib/pocketbase';

export interface FileQueryParams {
	thumb?: string; // e.g. '100x100' or '0x80'
	token?: string; // for protected files
	download?: boolean;
}

/**
 * Returns the full URL for a file stored in a PocketBase record field.
 * Returns null if the field is empty/missing.
 *
 * @example
 * const avatarUrl = getFileURL(userRecord, 'avatar', { thumb: '100x100' });
 */
export function getFileURL(
	record: { id: string; collectionId: string; collectionName: string; [key: string]: unknown },
	field: string,
	params?: FileQueryParams
): string | null {
	const filename = record[field];
	if (!filename) return null;
	return pb.files.getURL(record, filename as string, params);
}
