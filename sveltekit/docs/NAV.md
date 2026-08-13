# Navigation layout

This doc covers the SvelteKit navigation surface in template-derived repos: how `NavPanel` swaps between modes, and one Skeleton quirk that needs a targeted CSS override every time.

## The 3-mode nav

`NavPanel` (`src/lib/components/layout/NavPanel.svelte`) renders the same nav config three different ways based on viewport + open state:

- **Mobile** (`< sm`): hidden by default; opens as a slide-in overlay drawer using Skeleton's `<Navigation layout="sidebar">`. A separate `NavBar` component renders the fixed bottom bar with `<Navigation layout="bar">`.
- **Tablet / desktop collapsed** (`≥ sm`, `< lg`, or `≥ lg` with nav toggled closed): rail mode — `<Navigation layout="rail">`, icon-only entries.
- **Desktop expanded** (`≥ lg` with nav toggled open): sidebar — `<Navigation layout="sidebar">`, icons + labels + group headers.

Skeleton's `<Navigation.*>` components are headless data-attribute carriers (`data-scope="navigation" data-part="root|content|group|menu|footer|..."`); the actual layout CSS ships from `@skeletonlabs/skeleton-common` (a transitive dep of `@skeletonlabs/skeleton-svelte`).

## Skeleton rail quirk: equal-height menus

In rail mode Skeleton spaces nav **groups** as equal-height stretched panels, with each group's items centered inside its slice. With two or more `Navigation.Group` instances this produces a large empty band between the first group's items and the second group's items.

### Where the behavior comes from

`node_modules/.pnpm/@skeletonlabs+skeleton-common@<ver>/node_modules/@skeletonlabs/skeleton-common/src/components/navigation.css`:

```css
[data-scope='navigation'][data-part='menu'] {
	flex: 1; /* grows to fill */
	display: flex;
	gap: --spacing(2);
	&[data-layout='rail'] {
		flex-direction: column;
		justify-content: center; /* centers items in the grown space */
	}
}

[data-scope='navigation'][data-part='content'][data-layout='rail'],
[data-scope='navigation'][data-part='group'][data-layout='rail'] {
	display: contents; /* invisible to flex layout */
}

[data-scope='navigation'][data-part='root'][data-layout='rail'] {
	display: flex;
	flex-direction: column;
	gap: --spacing(4);
	/* ... padding, width, etc. */
}
```

`display: contents` on `Content` and `Group` means they're skipped by the flex layout — every `Navigation.Menu` becomes a direct flex child of `[data-part='root']`. With `flex: 1` on each menu, two menus split the rail height equally; `justify-content: center` then parks each menu's items in the middle of its slice. The result is the visible "two clusters pushed apart" pattern.

This is fine when each group really is a major section that deserves its own slice (Slack/Discord-style sidebars), but it's wrong for a flat list of icons with one trailing admin section.

## The override

Three edits in `NavPanel.svelte`. All use Tailwind v4's `!` **suffix** (postfix `!important`):

```svelte
<!-- Every Navigation.Menu rendered inside Navigation.Content: -->
<Navigation.Menu class="flex-none! justify-start!">
	<!-- ... TriggerAnchors ... -->
</Navigation.Menu>

<!-- Pin the footer at the bottom: -->
<Navigation.Footer class="mt-auto">
	<Navigation.Menu>
		<!-- ... -->
	</Navigation.Menu>
</Navigation.Footer>
```

### Why `!` is required

Skeleton's selectors look like `[data-scope='navigation'][data-part='menu']` — three attribute selectors give specificity `(0, 3, 0)`. A plain Tailwind class is `(0, 1, 0)` and loses the cascade. The `!` suffix generates `!important`, which wins regardless of specificity.

### What each line does

- `flex-none!` → `flex: none !important;` overrides Skeleton's `flex: 1`; the menu shrinks to its natural content height.
- `justify-start!` → `justify-content: flex-start !important;` overrides the rail-only `justify-content: center`; items sit at the top of the menu.
- `mt-auto` on `Navigation.Footer` → with the menus no longer growing, `mt-auto` (margin-top: auto on a flex item) pushes the footer to the bottom of the root flex column. No `!` needed — Skeleton doesn't set a `[data-part='footer']` rule.
- Footer's inner `Navigation.Menu` keeps `flex: 1` but it's inert: the `<footer>` parent isn't `display: flex`, so flex properties on the menu don't apply.

## When to apply

Apply this override in **any** project that:

1. Uses `@skeletonlabs/skeleton-svelte` v4.x or v5.x (or any version that still ships the `flex: 1` + `justify-content: center` rules on `[data-scope=navigation][data-part=menu]` — re-check on major upgrades; **confirmed still present in v5.0.0**, so the override survived the v5 migration unchanged).
2. Renders `<Navigation layout="rail">` (or `<Navigation>` with a binding that can produce `data-layout="rail"`).
3. Has **two or more** `<Navigation.Group>` instances.

Single-group rails don't show the bug because there's only one menu to stretch.

## Verification after applying

1. Collapse the rail. The icons from every group should pack tightly at the top with the `gap: --spacing(4)` (≈ 16 px) that the root applies between flex children. The footer item sits at the bottom of the rail.
2. Expand to sidebar. Labels reappear; nothing else moves; spacing inside groups (`gap: --spacing(2)` between label and links) is unchanged.
3. Resize to mobile bottom-bar mode. `NavBar` is a separate component instance, unaffected by these overrides.

## Porting to a new repo

1. Copy this `NAV.md` into the new repo's `sveltekit/docs/`.
2. In the new repo's `NavPanel.svelte` (or equivalent), apply the three edits above to every `<Navigation.Menu>` inside `<Navigation.Content>` and add `class="mt-auto"` to `<Navigation.Footer>`.
3. Add a pointer to `NAV.md` in the new repo's `CLAUDE.md` (or whatever doc indexes frontend conventions).

No dependency on a specific `navigation.ts` shape — works whatever groups you have, as long as the layout-mode switching is done with Skeleton's `layout="rail|sidebar|bar"` prop.
