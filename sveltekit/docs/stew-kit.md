# stew-kit — motion & theme kit

Reference for the theme + motion + fx layer of the Skeleton v5 / Tailwind v4 / Svelte 5 frontend.

> Ported from `stew-site-template`. This repo keeps its own eight Halo/Xbox
> themes instead of the template's `stew` / `custom` pair — the motion + fx
> layer below is identical. The starcommand components are a separate,
> repo-local kit; see the section at the end.

## Where files live

| File(s)                          | Purpose                                                      |
| -------------------------------- | ------------------------------------------------------------ |
| `src/lib/themes/theme-*.css`     | the eight themes (`data-theme="starcommand"` is the default) |
| `src/lib/styles/motion.css`      | motion/background utility classes                            |
| `src/lib/components/fx/*.svelte` | 13 fx components (listed below)                              |

## Wiring

`src/routes/layout.css` — after the skeleton imports:

```css
@import '../lib/themes/theme-default.css';
@import '../lib/themes/theme-forerunner.css';
@import '../lib/themes/theme-norcal.css';
@import '../lib/themes/theme-xbox.css';
@import '../lib/themes/theme-phosphor.css';
@import '../lib/themes/theme-midnight.css';
@import '../lib/themes/theme-debug.css';
@import '../lib/themes/theme-starcommand.css';
@import '../lib/styles/motion.css';
@import '../lib/styles/starcommand-utilities.css';
```

`src/app.html` — pick the active theme:

```html
<html lang="en" data-theme="starcommand"></html>
```

Themes are additive: registering all eight costs nothing, and you can switch
at runtime with `document.documentElement.setAttribute('data-theme', 'norcal')`.

- Each theme implements the full **247-var Skeleton v5 token contract**. To add
  one, copy an existing `theme-*.css` — not an upstream v4 theme, whose
  typography and body-background tokens use the pre-v5 names.
- The dark-mode body gradient in `layout.css` reads `--color-primary-500` /
  `--color-tertiary-500`, so it re-tints per theme automatically. `xbox` and
  `debug` additionally override the body treatment behind `[data-theme='…']`.
- A theme adding a **fixed full-viewport backdrop** must also set
  `body { background-color: transparent }` under its own `[data-theme='…']`
  selector. Skeleton v5 paints the root background on `html`, so an opaque
  `body` background paints over any negative-`z-index` layer. See CLAUDE.md.

## Motion utilities (`motion.css`)

Compose like any Tailwind/Skeleton class; all honor `prefers-reduced-motion`.

```html
<div class="hover-lift card p-6">…</div>
<button class="btn press icon-slide preset-filled">Get Started <ArrowRightIcon /></button>
<a class="btn preset-outlined-primary-500 glow" href="…">Docs</a>

<!-- loading state: shimmer + Skeleton placeholders -->
<div class="shimmer space-y-3 card p-6">
	<div class="placeholder-circle w-10"></div>
	<div class="placeholder w-3/4"></div>
	<div class="placeholder w-1/2"></div>
</div>
```

## Backgrounds

Four token-tinted treatments — mix or match per section:

```html
<!-- CSS-only, from motion.css -->
<section class="bg-aurora">…</section>
<!-- drifting blurred color blobs -->
<section class="bg-dotgrid">…</section>
<!-- dot grid fading out radially -->
```

```svelte
<!-- JS-driven, inside a position:relative container -->
<ParticleField density={40} />
<!-- floating particles -->
<MouseGlow />
<!-- soft glow that follows the pointer -->
```

## Components

```svelte
<script>
	import ParticleField from '$lib/components/fx/ParticleField.svelte';
	import AnimateIn from '$lib/components/fx/AnimateIn.svelte';
	import CountUp from '$lib/components/fx/CountUp.svelte';
</script>

<!-- exciting background: fills nearest position:relative ancestor -->
<section class="relative">
	<ParticleField density={40} />
	<div class="relative">hero content…</div>
</section>

<AnimateIn stagger={60} class="grid gap-4 sm:grid-cols-3">
	{#each features as f}<div class="card p-6">…</div>{/each}
</AnimateIn>

<p class="text-3xl font-bold"><CountUp value={1234} /></p>
<CountUp value={8.3} decimals={1} prefix="+" suffix="%" />
```

`ParticleField` and `MouseGlow` read `--color-primary-500` / `--color-secondary-500` /
`--color-tertiary-500` from the active theme, re-seeds on `data-theme` or
`.dark` changes, pauses offscreen, and goes static under reduced motion.

## App-page polish components

All live in `src/lib/components/fx/`; import like the others. Every one is
theme-token driven and reduced-motion safe.

```svelte
<!-- +layout.svelte — one wrapper, all routes transition -->
<PageTransition>{@render children()}</PageTransition>
<ScrollProgress />
<!-- or target="#docs-pane" for a container -->

<!-- data loading (PocketBase fetches) -->
<SkeletonBlock loaded={!!records} lines={3} avatar>
	{#each records as r}…{/each}
</SkeletonBlock>

<!-- dashboard metrics -->
<Sparkline data={requests} />
<Sparkline data={churn} color="var(--color-error-500)" fill={false} />

<!-- pricing / gallery cards -->
<TiltCard class="card p-6" max={8}>…</TiltCard>

<!-- hero headline -->
<h1>Build <TypeCycle words={['dashboards', 'inboxes', 'wizards']} class="text-primary-400" /></h1>

<!-- success moments (register done, wizard complete) -->
<ConfettiBurst bind:this={confetti} />
<button onclick={(e) => confetti.fire(e.clientX, e.clientY)}>Finish</button>

<!-- zero-data -->
<EmptyState title="No messages yet" description="When someone writes to you, it lands here.">
	{#snippet action()}<button class="btn preset-filled">New message</button>{/snippet}
</EmptyState>
```

Key props (full docs in each file's header comment):

- `PageTransition` — `duration=220`, `y=8`; View Transitions API, silent no-op fallback
- `SkeletonBlock` — `loaded`, `lines=3`, `avatar`, `media`; children render when loaded
- `ScrollProgress` — `target` selector (empty = window), `height=3`, `zIndex=50`
- `TiltCard` — `max=8` deg, `scale=1.02`, `glare`; off on touch + reduced motion
- `Sparkline` — `data: number[]`, `width/height`, `color`, `fill`, `dot`, `animate`
- `ConfettiBurst` — exported `fire(x?, y?, count=90)`; mount once near the page root
- `TypeCycle` — `words: string[]`, `typeSpeed`, `deleteSpeed`, `hold`, `caret`
- `EmptyState` — `title`, `description`, snippets `icon` / `action`
- `BuildStamp` — `label`, `inline`, `corner`, or wrap children for anchor mode; see wiring below

### BuildStamp — stale-bundle spot check

`svelte.config.js` — bake the git commit into the bundle and enable deploy polling:

```js
import { execSync } from 'node:child_process';

const commit = (() => {
	try {
		return execSync('git rev-parse --short HEAD').toString().trim();
	} catch {
		return `${Date.now()}`;
	}
})();

const config = {
	kit: {
		version: { name: commit, pollInterval: 60_000 }
		// …existing adapter/env config
	}
};
```

Then once in `+layout.svelte`:

```svelte
<BuildStamp />
<!-- fixed bottom-right, 35% opacity -->
<BuildStamp inline label="v1.4.2" />
<!-- or in the sidebar footer -->

<!-- Anchor mode: zero footprint. Wrap the app icon in NavToggle.svelte —
     hover/focus the logo to see the commit; an amber corner dot appears
     only when a newer deploy is detected (card gains a Reload button).
     Already wired here: src/lib/components/layout/NavToggle.svelte. -->
<BuildStamp>
	<a href={resolve('/')} class="flex min-w-0 items-center gap-2" aria-label="{APP_NAME} home">
		<img src={logoUrl} alt="" class="size-8 shrink-0" />
	</a>
</BuildStamp>
```

- **Green dot + commit** — the bundle you are looking at is current (click copies the commit).
- **Amber + “update available”** — SvelteKit's `updated` store found a newer
  `_app/version.json`, i.e. this tab is stale; click hard-reloads.

The Go server's ldflags version (`internal/version`, served at `/api/version`)
can be surfaced too — pass it as `label` to compare server vs. bundle at a glance.

## starcommand kit

Repo-local companion to the `starcommand` theme — the v5 port of the NorCal
Halo "Star Command" UnleashX dashboard skin. Six components in
`src/lib/components/starcommand/` + three utilities in
`src/lib/styles/starcommand-utilities.css` (`.sc-scanlines`, `.sc-vignette`,
`.sc-mono`). All read theme tokens only, so they render under any of the eight
themes — they just look _right_ under `starcommand`.

```svelte
<script>
	import star from '$lib/assets/star.png';
	import Panel from '$lib/components/starcommand/Panel.svelte';
	import Eyebrow from '$lib/components/starcommand/Eyebrow.svelte';
	import StatusDot from '$lib/components/starcommand/StatusDot.svelte';
	import AccentChip from '$lib/components/starcommand/AccentChip.svelte';
	import NorcalLockup from '$lib/components/starcommand/NorcalLockup.svelte';
</script>

<NorcalLockup {star} size={48} />

<Panel class="p-6" accent="secondary">
	<Eyebrow tone="secondary">LAN Ops</Eyebrow>
	<h2 class="h3">System Link</h2>
</Panel>

<StatusDot tone="success" pulse label="Online" />
<AccentChip tone="primary">Event</AccentChip>

<div class="sc-mono text-sm">192.168.1.107 · D: 3.2 GB FREE</div>
```

Buttons stay native Skeleton — the tokens already produce the look:
`btn preset-filled-primary-500` = orange CTA, `btn preset-outlined-surface-200-800`
= quiet secondary.

**House rules.** One accent leads per surface (blue = LAN/ops/stats, orange =
events/CTAs) · green only for status · **never recolor the star** · Orbitron
never at body sizes · motion lives in the background, content stays calm.

### StarfieldBackground — already wired

Mounted once in `+layout.svelte`, outside `PageTransition` (so navigation
doesn't restart its drift animations). Two constraints are enforced in
`layout.css` rather than by conditional mounting, so a runtime `data-theme` or
mode swap works with no JS:

- **Theme-gated** — paints only under `[data-theme='starcommand']`. The other
  seven themes bring their own backdrop.
- **Dark-only** — `html.dark[data-theme='starcommand']`. The theme is
  dark-first; in light mode the navy starfield would sit behind dark body text
  and swallow it. Hidden, `body` stays transparent and html's `light-dark()`
  root-bg gives light mode a proper backdrop.

It is also skipped on `hideLayout` routes (`isLayoutHidden`) — those are the
OBS browser sources, which must composite over a transparent page.

Pass the emblem for hero/screensaver contexts:

```svelte
import star from '$lib/assets/star.png';
<StarfieldBackground logo={star} />
```
