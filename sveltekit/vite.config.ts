import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { bspSavePlugin } from './vite-bsp-save';

export default defineConfig({
	// bspSavePlugin: dev-only POST /__bsp-save for the /bsp-editor tool; no-op in
	// production builds (apply: 'serve'). See vite-bsp-save.ts.
	plugins: [tailwindcss(), sveltekit(), bspSavePlugin()],
	envDir: '..'
});
