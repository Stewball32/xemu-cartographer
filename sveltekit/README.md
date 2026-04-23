# SvelteKit Frontend

SvelteKit SPA served by PocketBase from `pb_public/`.

## Stack

- **Svelte 5** + **SvelteKit 2** with TypeScript
- **Skeleton UI v4** (cerberus theme) + **Tailwind CSS v4**
- **PocketBase JS SDK** for REST/auth
- **Package manager:** pnpm

## Development

```bash
# From repo root (runs both backend + frontend)
task dev

# Frontend only (from this directory)
pnpm dev
```

The dev server reads `PUBLIC_PB_PORT` from the root `.env` (via `envDir: '..'` in vite.config.ts). The PocketBase client connects to `http://localhost:${PUBLIC_PB_PORT}` in dev mode and uses same-origin in production.

## Build

```bash
# From repo root
task build:frontend
```

adapter-static outputs directly to `pb_public/` with `fallback: 'index.html'` for SPA routing. No copy step needed.

## Layout Config

`src/routes/+layout.ts` sets:

- `ssr = false` — client-side only
- `prerender = true` — static generation for known routes
- `trailingSlash = 'always'` — consistent URL format

## Recreate Scaffold

```sh
pnpm dlx sv@0.13.1 create --template minimal --types ts --add tailwindcss="plugins:none" sveltekit-adapter="adapter:static" prettier eslint --install pnpm .
pnpm add -D @skeletonlabs/skeleton @skeletonlabs/skeleton-svelte pocketbase-typegen
pnpm add pocketbase
```
