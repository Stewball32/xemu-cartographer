// Resolves the PocketBase HTTP / WebSocket base URL.
//
// Dev: SvelteKit runs on Vite (typically :5173) while PocketBase runs on
// :PUBLIC_PB_PORT — different ports, so the API host has to be named
// explicitly. We use `window.location.hostname` rather than 'localhost' so
// that opening the dev page from a phone on the LAN (e.g. http://192.168.x.y:5173)
// hits the same machine's :8090 instead of trying to reach the phone itself.
//
// Prod: SvelteKit's static build is served by PocketBase, so everything is
// same-origin — return '' (relative) for HTTP and the current host for WS.
import { dev } from '$app/environment';
import { PUBLIC_PB_PORT } from '$env/static/public';

export function apiBaseURL(): string {
	if (!dev) return '';
	if (typeof window === 'undefined') return `http://localhost:${PUBLIC_PB_PORT}`;
	return `http://${window.location.hostname}:${PUBLIC_PB_PORT}`;
}

export function wsBaseURL(): string {
	if (typeof window === 'undefined') return `ws://localhost:${PUBLIC_PB_PORT}`;
	const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
	if (dev) return `${proto}://${window.location.hostname}:${PUBLIC_PB_PORT}`;
	return `${proto}://${window.location.host}`;
}
