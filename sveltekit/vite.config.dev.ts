import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { bspSavePlugin } from './vite-bsp-save';

// DEV-TIER Vite config for dev.norcal.pro — the live-reload-from-repo layer.
//
//   browser ──▶ https://dev.norcal.pro ──(cloudflared)──▶ localhost:19099 (this Vite)
//     • pages / assets / HMR : served by Vite (SvelteKit dev, instant reload)
//     • /api, /api/ws, /api/realtime, /_  : reverse-proxied to the dev PocketBase
//
// The dev PocketBase runs on :19090 (Air hot-reload) and is NEVER exposed
// publicly — only reachable through this proxy, so the app must talk to it
// same-origin (see directPortHost() in src/lib/utils/api-base.ts).
//
// Run via ../run-dev.sh (which also starts the dev PocketBase). Mirrors the base
// vite.config.ts (tailwind + sveltekit + bspSavePlugin + shared root .env).

const DEV_PB = 'http://127.0.0.1:19090';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit(), bspSavePlugin()],
	envDir: '..',
	server: {
		host: true, // bind 0.0.0.0 so cloudflared (localhost) can reach it
		port: 19099,
		strictPort: true,
		allowedHosts: ['dev.norcal.pro', 'localhost', '127.0.0.1'],
		// HMR back-channel travels over the tunnel: wss to the public host on 443.
		hmr: { protocol: 'wss', host: 'dev.norcal.pro', clientPort: 443 },
		proxy: {
			// REST + realtime (SSE) + the custom /api/ws WebSocket → dev PB.
			'/api': { target: DEV_PB, changeOrigin: true, ws: true },
			// PocketBase superuser admin UI.
			'/_': { target: DEV_PB, changeOrigin: true }
		}
	}
});
