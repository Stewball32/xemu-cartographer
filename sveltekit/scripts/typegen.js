#!/usr/bin/env node
/**
 * Thin wrapper around pocketbase-typegen that maps this project's env vars.
 *
 * Usage:
 *   pnpm typegen              (from sveltekit/)
 *   task typegen              (requires task dev:backend running)
 *
 * Env vars (with defaults matching seed data):
 *   PUBLIC_PB_PORT            PocketBase port (default: 8090)
 *   PB_SUPERUSER_EMAIL        Superuser email (default: root@dev.com)
 *   PB_SUPERUSER_PASSWORD     Superuser password (default: root1234)
 */

import { spawnSync } from 'child_process';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const PORT = process.env.PUBLIC_PB_PORT ?? '8090';
const EMAIL = process.env.PB_SUPERUSER_EMAIL;
const PASSWORD = process.env.PB_SUPERUSER_PASSWORD;

if (!EMAIL || !PASSWORD) {
	console.log('typegen: PB_SUPERUSER_EMAIL or PB_SUPERUSER_PASSWORD not set — skipping');
	process.exit(0);
}
const OUT = resolve(__dirname, '../src/lib/types/pocketbase-types.ts');

const TIMEOUT_MS = Number(process.env.PB_TYPEGEN_TIMEOUT_MS ?? 60_000);
const POLL_INTERVAL_MS = 1000;
const PROBE_TIMEOUT_MS = 1000;

async function waitForServer(url) {
	const deadline = Date.now() + TIMEOUT_MS;
	let announced = false;
	while (Date.now() < deadline) {
		const ctrl = new AbortController();
		const t = setTimeout(() => ctrl.abort(), PROBE_TIMEOUT_MS);
		try {
			const res = await fetch(url, { signal: ctrl.signal });
			if (res.ok) return true;
		} catch {
			// not up yet
		} finally {
			clearTimeout(t);
		}
		if (!announced) {
			console.log('typegen: waiting for PocketBase…');
			announced = true;
		}
		await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
	}
	return false;
}

const ready = await waitForServer(`http://localhost:${PORT}/api/health`);
if (!ready) {
	console.error(`typegen: PocketBase did not respond within ${TIMEOUT_MS}ms — aborting`);
	process.exit(1);
}
console.log('typegen: server ready, generating types…');

// Resolve the binary from node_modules.
// On Windows, .cmd wrappers must be invoked via cmd.exe (shell: true).
const isWindows = process.platform === 'win32';
const ext = isWindows ? '.cmd' : '';
const bin = resolve(__dirname, `../node_modules/.bin/pocketbase-typegen${ext}`);

const result = spawnSync(
	bin,
	['--url', `http://localhost:${PORT}`, '--email', EMAIL, '--password', PASSWORD, '--out', OUT],
	{ stdio: 'inherit', shell: isWindows }
);

process.exit(result.status ?? 1);
