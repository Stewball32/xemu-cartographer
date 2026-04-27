import { dev } from '$app/environment';
import { PUBLIC_PB_PORT } from '$env/static/public';
import { auth } from '$lib/stores/auth.svelte';

const baseURL = dev ? `http://localhost:${PUBLIC_PB_PORT}` : '';

export class AdminFetchError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
		this.name = 'AdminFetchError';
	}
}

async function request(method: string, path: string, body?: unknown): Promise<Response> {
	if (!auth.token) {
		throw new AdminFetchError(401, 'not authenticated');
	}
	const headers: Record<string, string> = {
		Authorization: auth.token
	};
	if (body !== undefined) {
		headers['Content-Type'] = 'application/json';
	}
	const res = await fetch(`${baseURL}/api/admin/${path}`, {
		method,
		headers,
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		let msg = `HTTP ${res.status}`;
		try {
			const data = await res.clone().json();
			if (data && typeof data === 'object' && 'error' in data && typeof data.error === 'string') {
				msg = data.error;
			}
		} catch {
			// non-JSON body; keep default
		}
		throw new AdminFetchError(res.status, msg);
	}
	return res;
}

export async function adminGet<T>(path: string): Promise<T> {
	const res = await request('GET', path);
	if (res.status === 204) {
		return undefined as T;
	}
	return (await res.json()) as T;
}

export async function adminPost<T>(path: string, body?: unknown): Promise<T> {
	const res = await request('POST', path, body);
	if (res.status === 204) {
		return undefined as T;
	}
	return (await res.json()) as T;
}

export async function adminDelete(path: string): Promise<void> {
	await request('DELETE', path);
}
