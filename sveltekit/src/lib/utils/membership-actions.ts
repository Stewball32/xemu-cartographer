// membership-actions.ts — M23d
//
// Small wrappers around the M23d POST endpoints. The bell dropdown,
// /notifications/ page, /settings/ pending sections, and the team page
// all call into these so the success/error handling stays consistent.
//
// Each helper returns the response JSON on success and throws on failure
// (with the server-provided error body when available, otherwise the HTTP
// status text). Callers wrap the call in `toastPromise` to get the
// loading/success/error toasts.

import pb from '$lib/pocketbase';
import { apiBaseURL } from '$lib/utils/api-base';

async function post(path: string, body?: unknown): Promise<unknown> {
	const res = await fetch(`${apiBaseURL()}${path}`, {
		method: 'POST',
		headers: {
			Authorization: pb.authStore.token,
			...(body !== undefined ? { 'Content-Type': 'application/json' } : {})
		},
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		const txt = await res.text().catch(() => '');
		throw new Error(txt || res.statusText);
	}
	if (res.status === 204) return null;
	return res.json();
}

export function acceptRequest(requestId: string): Promise<unknown> {
	return post(`/api/team-membership-requests/${requestId}/accept`);
}

export function declineRequest(requestId: string): Promise<unknown> {
	return post(`/api/team-membership-requests/${requestId}/decline`);
}

export function cancelRequest(requestId: string): Promise<unknown> {
	return post(`/api/team-membership-requests/${requestId}/cancel`);
}

export function leaveRoster(rosterId: string): Promise<unknown> {
	return post(`/api/rosters/${rosterId}/leave`);
}

export function removeRosterMember(rosterId: string): Promise<unknown> {
	return post(`/api/rosters/${rosterId}/remove`);
}

export function inviteToTeam(teamId: string, userId: string): Promise<unknown> {
	return post(`/api/teams/${teamId}/invite`, { user_id: userId });
}

export function requestToJoinTeam(teamId: string): Promise<unknown> {
	return post(`/api/teams/${teamId}/join-requests`);
}
