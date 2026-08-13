import adapter from '@sveltejs/adapter-static';
import { execSync } from 'node:child_process';
import { relative, sep } from 'node:path';

// Bake the git commit into the bundle so BuildStamp can show it and SvelteKit's
// `updated` store can detect newer deploys (falls back to a per-build timestamp
// when .git is unavailable, e.g. inside a container build).
const commit = (() => {
	try {
		return execSync('git rev-parse --short HEAD').toString().trim();
	} catch {
		return `${Date.now()}`;
	}
})();

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// defaults to rune mode for the project, except for `node_modules`. Can be removed in svelte 6.
		runes: ({ filename }) => {
			const relativePath = relative(import.meta.dirname, filename);
			const pathSegments = relativePath.toLowerCase().split(sep);
			const isExternalLibrary = pathSegments.includes('node_modules');

			return isExternalLibrary ? undefined : true;
		}
	},
	kit: {
		adapter: adapter({
			pages: '../pb_public',
			assets: '../pb_public',
			fallback: 'index.html'
		}),
		version: { name: commit, pollInterval: 60_000 },
		env: { dir: '..' }
	}
};

export default config;
