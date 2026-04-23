import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getFileURL } from './files';

// Mock the PocketBase client
vi.mock('$lib/pocketbase', () => {
	return {
		default: {
			files: {
				getURL: vi.fn(
					(record: { collectionId: string; id: string }, filename: string, params?: object) =>
						`http://localhost:8090/api/files/${record.collectionId}/${record.id}/${filename}${params ? '?' + new URLSearchParams(params as Record<string, string>).toString() : ''}`
				)
			}
		}
	};
});

const mockRecord = {
	id: 'abc123',
	collectionId: 'users',
	collectionName: 'users',
	avatar: 'photo.png'
};

describe('getFileURL', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('returns a URL for a valid file field', () => {
		const url = getFileURL(mockRecord, 'avatar');
		expect(url).toContain('photo.png');
		expect(url).toContain('abc123');
	});

	it('returns null when the field is empty', () => {
		const record = { ...mockRecord, avatar: '' };
		expect(getFileURL(record, 'avatar')).toBeNull();
	});

	it('returns null when the field is missing', () => {
		const record = { id: 'abc123', collectionId: 'users', collectionName: 'users' };
		expect(getFileURL(record, 'avatar')).toBeNull();
	});

	it('passes query params through to the SDK', () => {
		const url = getFileURL(mockRecord, 'avatar', { thumb: '100x100' });
		expect(url).toContain('thumb=100x100');
	});
});
