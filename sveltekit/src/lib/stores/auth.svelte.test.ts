import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock PocketBase before importing auth store
const mockAuthWithPassword = vi.fn();
const mockAuthWithOAuth2 = vi.fn();
const mockListExternalAuths = vi.fn();
const mockUnlinkExternalAuth = vi.fn();
const mockOnChange = vi.fn();
const mockClear = vi.fn();

vi.mock('$lib/pocketbase', () => {
	return {
		default: {
			authStore: {
				record: null,
				token: '',
				isValid: false,
				onChange: mockOnChange,
				clear: mockClear
			},
			collection: () => ({
				authWithPassword: mockAuthWithPassword,
				authWithOAuth2: mockAuthWithOAuth2,
				listExternalAuths: mockListExternalAuths,
				unlinkExternalAuth: mockUnlinkExternalAuth
			})
		}
	};
});

// Import after mocks are set up
const { auth } = await import('./auth.svelte');

describe('auth store', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('starts logged out', () => {
		expect(auth.user).toBeNull();
		expect(auth.token).toBe('');
		expect(auth.isLoggedIn).toBe(false);
	});

	it('calls authWithPassword on login', async () => {
		mockAuthWithPassword.mockResolvedValueOnce({});
		await auth.login('test@example.com', 'password123');
		expect(mockAuthWithPassword).toHaveBeenCalledWith('test@example.com', 'password123');
	});

	it('calls authWithOAuth2 on OAuth login', async () => {
		mockAuthWithOAuth2.mockResolvedValueOnce({});
		await auth.loginWithOAuth('discord');
		expect(mockAuthWithOAuth2).toHaveBeenCalledWith({ provider: 'discord' });
	});

	it('clears the auth store on logout', () => {
		auth.logout();
		expect(mockClear).toHaveBeenCalled();
	});

	it('calls listExternalAuths with user ID', async () => {
		const mockAuths = [{ id: '1', provider: 'discord', providerId: '123', created: '' }];
		mockListExternalAuths.mockResolvedValueOnce(mockAuths);
		const result = await auth.listExternalAuths('user123');
		expect(mockListExternalAuths).toHaveBeenCalledWith('user123');
		expect(result).toEqual(mockAuths);
	});

	it('calls authWithOAuth2 on linkOAuth', async () => {
		mockAuthWithOAuth2.mockResolvedValueOnce({});
		await auth.linkOAuth('github');
		expect(mockAuthWithOAuth2).toHaveBeenCalledWith({ provider: 'github' });
	});

	it('calls unlinkExternalAuth on unlinkOAuth', async () => {
		mockUnlinkExternalAuth.mockResolvedValueOnce(undefined);
		await auth.unlinkOAuth('user123', 'discord');
		expect(mockUnlinkExternalAuth).toHaveBeenCalledWith('user123', 'discord');
	});
});
