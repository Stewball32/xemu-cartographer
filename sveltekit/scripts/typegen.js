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
