// Dev-only save endpoint for the BSP spectator-mesh editor (/bsp-editor).
//
// The editor is a browser tool; it can't write to disk. This Vite plugin adds a
// tiny POST /__bsp-save middleware (DEV ONLY — configureServer never runs in a
// production build) that writes the baked spectator mesh into the served geometry
// cache and patches the manifest so loadBspMesh starts serving it. That closes the
// edit→export→shows-up-in-the-visualizer loop in one click during `task dev`.
//
// It has zero effect on `pnpm build` (adapter-static): no SvelteKit endpoint, no
// prerender hook — just a dev middleware. The editor also offers a plain Download
// fallback for when this isn't available (prod/preview), so persistence never
// depends on it.
import type { Plugin, ViteDevServer } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { promises as fs } from 'node:fs';
import * as path from 'node:path';

const SAFE = /^[a-z0-9_]+$/;

interface SavePayload {
	game?: string;
	key?: string;
	mesh?: {
		positions?: unknown;
		indices?: unknown;
		vertex_count?: number;
		triangle_count?: number;
		bounds?: unknown;
		scenario?: string;
		source_map?: string;
		[k: string]: unknown;
	};
}

function readBody(req: IncomingMessage): Promise<string> {
	return new Promise((resolve, reject) => {
		const chunks: Buffer[] = [];
		req.on('data', (c) => chunks.push(c as Buffer));
		req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
		req.on('error', reject);
	});
}

function json(res: ServerResponse, status: number, body: unknown) {
	res.statusCode = status;
	res.setHeader('content-type', 'application/json');
	res.end(JSON.stringify(body));
}

export function bspSavePlugin(): Plugin {
	return {
		name: 'bsp-editor-save',
		apply: 'serve',
		configureServer(server: ViteDevServer) {
			const root = server.config.root; // the sveltekit/ dir
			server.middlewares.use('/__bsp-save', async (req, res, next) => {
				if (req.method !== 'POST') return next();
				try {
					const payload = JSON.parse(await readBody(req)) as SavePayload;
					const game = payload.game ?? '';
					const key = payload.key ?? '';
					const mesh = payload.mesh;
					if (!SAFE.test(game) || !SAFE.test(key)) {
						return json(res, 400, { error: 'invalid game/key (must be [a-z0-9_])' });
					}
					if (
						!mesh ||
						!Array.isArray(mesh.positions) ||
						!Array.isArray(mesh.indices) ||
						mesh.indices.length < 3
					) {
						return json(res, 400, { error: 'mesh missing positions/indices' });
					}

					const dir = path.join(root, 'static', 'game-geometry', game);
					const fileName = `${key}.spectator.json`;
					const filePath = path.join(dir, fileName);

					// The cache dir must already exist (the extractor created it). Refuse to
					// invent a new game cache from the editor.
					try {
						await fs.access(dir);
					} catch {
						return json(res, 404, {
							error: `cache dir not found: static/game-geometry/${game}/ — run the extractor first`
						});
					}

					await fs.writeFile(filePath, JSON.stringify(mesh));

					// Patch the manifest so loadBspMesh prefers the baked mesh.
					const manPath = path.join(dir, 'manifest.json');
					let manifest: {
						meshes?: Record<string, Record<string, unknown>>;
						[k: string]: unknown;
					} = {};
					try {
						manifest = JSON.parse(await fs.readFile(manPath, 'utf8'));
					} catch {
						manifest = { meshes: {} };
					}
					manifest.meshes ??= {};
					const entry = (manifest.meshes[key] ??= { file: `${key}.json` });
					entry.spectator_file = fileName;
					if (typeof mesh.vertex_count === 'number')
						entry.spectator_vertex_count = mesh.vertex_count;
					if (typeof mesh.triangle_count === 'number')
						entry.spectator_triangle_count = mesh.triangle_count;
					await fs.writeFile(manPath, JSON.stringify(manifest, null, 2));

					server.config.logger.info(
						`[bsp-editor] baked spectator mesh → static/game-geometry/${game}/${fileName}`
					);
					return json(res, 200, { ok: true, file: `game-geometry/${game}/${fileName}` });
				} catch (err) {
					return json(res, 500, { error: String(err) });
				}
			});
		}
	};
}
