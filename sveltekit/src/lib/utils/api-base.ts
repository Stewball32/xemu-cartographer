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

// directPortHost: in dev, whether to address PocketBase explicitly at
// :PUBLIC_PB_PORT (true) vs. same-origin (false).
//
//   - localhost / a bare LAN IP (task dev, or a phone at http://192.168.x.y:5173)
//     → true: PB is a sibling port on the same host, name it explicitly.
//   - a real domain (e.g. dev.norcal.pro through the cloudflared tunnel) → false:
//     the Vite dev server reverse-proxies /api (+ /_ + realtime + WS) to the dev
//     PB, so the app must use RELATIVE URLs — only that reaches PB through the
//     tunnel (the PB port is never exposed publicly). See vite.config.dev.ts.
function directPortHost(): boolean {
	if (typeof window === 'undefined') return true;
	const h = window.location.hostname;
	return h === 'localhost' || /^[0-9.]+$/.test(h);
}

export function apiBaseURL(): string {
	if (!dev) return '';
	if (typeof window === 'undefined') return `http://localhost:${PUBLIC_PB_PORT}`;
	if (!directPortHost()) return ''; // dev behind a same-origin proxy (dev.norcal.pro)
	const proto = window.location.protocol === 'https:' ? 'https' : 'http';
	return `${proto}://${window.location.hostname}:${PUBLIC_PB_PORT}`;
}

export function wsBaseURL(): string {
	if (typeof window === 'undefined') return `ws://localhost:${PUBLIC_PB_PORT}`;
	const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
	if (dev && directPortHost()) return `${proto}://${window.location.hostname}:${PUBLIC_PB_PORT}`;
	return `${proto}://${window.location.host}`;
}
