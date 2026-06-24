// Offline pre-bake of spectator meshes for the visualizers — the headless twin
// of the /bsp-editor "Save to cartographer" button.
//
// It reuses the EXACT editor logic ($lib/utils/bsp-edit.ts + floor-bands.ts —
// pure, IO-free, type-only imports) by running this TS file directly under Node's
// type-stripping, so the baked asset is byte-for-byte what the browser editor
// would produce from the same auto-tags. For each map it:
//   1. reads the raw extracted mesh (static/game-geometry/<game>/<key>.json),
//   2. auto-classifies with the material-name priors (schema-3 caches) and folds
//      in any existing hand-edited <key>.tags.json sidecar (authoritative),
//   3. drops the off-render tags (ceiling/inaccessible/clutter) + applies an
//      optional per-map clip box, then writes <key>.spectator.json + the tag
//      sidecar and patches the manifest's spectator_file so loadBspMesh serves it.
//
// Run:  node --experimental-strip-types scripts/game-geometry/bake_spectator.ts [key...]
// (defaults to chillout prisoner bloodgulch). Static/offline — reads only the
// already-extracted JSON cache; never touches xemu or the .map files.

import { promises as fs } from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import {
  buildTriMeta,
  autoClassify,
  applyTagOverrides,
  exportSpectatorMesh,
  buildTagSidecar,
  degenerateTriangleIndices,
  selectBeyondPlane,
  taggedFloorZs,
  tagCounts,
  editStats,
  defaultTagRender,
  TAG_LEGEND,
  type SurfaceTag,
  type TagOverride,
  type TagRenderMap,
  type CullAxis,
  type SpectatorBand,
} from "../../sveltekit/src/lib/utils/bsp-edit.ts";
import { computeFloorBands } from "../../sveltekit/src/lib/utils/floor-bands.ts";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const GAME = "haloce";
const CACHE = path.resolve(
  HERE,
  "..",
  "..",
  "sveltekit",
  "static",
  "game-geometry",
  GAME,
);

/** One clip-plane cut, in Halo world units — equivalent to one editor "Remove
 *  beyond" press. Applied in sequence to carve out-of-map shells / stray bits. */
interface Clip {
  axis: CullAxis;
  offset: number;
  sign: 1 | -1;
  mode?: "centroid" | "whole" | "any";
}

/** Per-map bake recipe. `clips` are extra scalpels beyond the tag render rules;
 *  empty means "tags alone decide" (the pure default bake). */
interface Recipe {
  clips?: Clip[];
  note?: string;
}

const RECIPES: Record<string, Recipe> = {
  // First pass: tags alone (the priors now bin overhead lights as `ceiling`, which
  // default render drops). Any residual lower-tunnel clutter is handled with a
  // targeted clip below, decided from the baked screenshots.
  chillout: {},
  prisoner: {},
  bloodgulch: {},
};

interface RawMesh {
  scenario?: string;
  source_map?: string;
  bounds: {
    minX: number;
    maxX: number;
    minY: number;
    maxY: number;
    minZ: number;
    maxZ: number;
  };
  positions: number[];
  colors?: number[];
  indices: number[];
  materials?: string[];
  material_semantic?: string[];
  triangle_material?: number[];
}

function fmtCounts(c: Record<SurfaceTag, number>): string {
  return TAG_LEGEND.map((t) => `${t}=${c[t]}`).join(" ");
}

async function readSidecar(
  key: string,
): Promise<{ overrides?: TagOverride[]; render?: TagRenderMap } | null> {
  try {
    return JSON.parse(
      await fs.readFile(path.join(CACHE, `${key}.tags.json`), "utf8"),
    );
  } catch {
    return null;
  }
}

async function bake(key: string): Promise<void> {
  const rawPath = path.join(CACHE, `${key}.json`);
  let raw: RawMesh;
  try {
    raw = JSON.parse(await fs.readFile(rawPath, "utf8"));
  } catch (e) {
    console.error(`! ${key}: cannot read ${rawPath} — ${e}`);
    return;
  }

  const mesh = {
    game: GAME,
    scenario: raw.scenario ?? key,
    sourceMap: raw.source_map ?? "",
    bounds: raw.bounds,
    positions: raw.positions,
    colors: raw.colors,
    indices: raw.indices,
    materials: raw.materials,
    materialSemantic: raw.material_semantic,
    triangleMaterial: raw.triangle_material,
  };

  // Geometry-only vs material-prior auto-tags — for the before/after report.
  const metaGeom = buildTriMeta({
    positions: mesh.positions,
    indices: mesh.indices,
  });
  const geomCounts = tagCounts(autoClassify(metaGeom));
  const meta = buildTriMeta(mesh);
  const autoTags = autoClassify(meta);
  const priorCounts = tagCounts(autoTags);

  // Hand-edited sidecar wins over both the geometry and the priors.
  const sidecar = await readSidecar(key);
  let tags = applyTagOverrides(meta, autoTags, sidecar?.overrides);
  const tagRender = sidecar?.render
    ? { ...defaultTagRender(), ...sidecar.render }
    : defaultTagRender();

  // Degenerate (zero-area) triangles → removed: stray-line BSP artifacts in the
  // ramp/tunnel areas. Same seed the editor applies on load, so editor-save and
  // this offline bake agree.
  const removed = new Uint8Array(meta.length);
  const degenerate = degenerateTriangleIndices(meta);
  for (const t of degenerate) removed[t] = 1;

  // Manual clips → removed triangles (mirrors the editor's "Remove beyond").
  const recipe = RECIPES[key] ?? {};
  let clipped = 0;
  for (const clip of recipe.clips ?? []) {
    for (const t of selectBeyondPlane(
      meta,
      clip.axis,
      clip.offset,
      clip.sign,
      clip.mode,
    )) {
      if (!removed[t]) {
        removed[t] = 1;
        clipped++;
      }
    }
  }

  const bands: SpectatorBand[] = computeFloorBands(
    taggedFloorZs(meta, tags),
    5,
  ).bands.map((b) => ({
    index: b.index,
    minZ: b.minZ,
    maxZ: b.maxZ,
    midZ: b.midZ,
  }));

  const spec = exportSpectatorMesh(
    mesh,
    { removed, tags },
    {
      meta,
      autoTags,
      tagRender,
      sourceMesh: `${key}.json`,
      bands,
      generatedAt: new Date().toISOString(),
    },
  );

  const stats = editStats(mesh, removed, tags, tagRender);

  // Write the baked mesh + the re-applicable tag sidecar.
  const specFile = `${key}.spectator.json`;
  const tagsFile = `${key}.tags.json`;
  await fs.writeFile(path.join(CACHE, specFile), JSON.stringify(spec));
  await fs.writeFile(
    path.join(CACHE, tagsFile),
    JSON.stringify(buildTagSidecar(meta, tags, autoTags, tagRender), null, 2),
  );

  // Patch the manifest so loadBspMesh prefers the baked mesh (same fields the
  // dev save endpoint writes).
  const manPath = path.join(CACHE, "manifest.json");
  type ManEntry = Record<string, unknown>;
  let manifest: { meshes?: Record<string, ManEntry>; [k: string]: unknown } = {
    meshes: {},
  };
  try {
    manifest = JSON.parse(await fs.readFile(manPath, "utf8"));
  } catch {
    /* fresh manifest */
  }
  manifest.meshes ??= {};
  const entry = (manifest.meshes[key] ??= { file: `${key}.json` });
  entry.spectator_file = specFile;
  entry.tags_file = tagsFile;
  entry.spectator_vertex_count = spec.vertex_count;
  entry.spectator_triangle_count = spec.triangle_count;
  await fs.writeFile(manPath, JSON.stringify(manifest, null, 2));

  console.log(
    `\n=== ${key} ===  (${meta.length} tris${recipe.note ? ` — ${recipe.note}` : ""})`,
  );
  console.log(`  geometry-only : ${fmtCounts(geomCounts)}`);
  console.log(`  with priors   : ${fmtCounts(priorCounts)}`);
  if (sidecar?.overrides?.length)
    console.log(`  + ${sidecar.overrides.length} sidecar overrides`);
  if (degenerate.length)
    console.log(`  + ${degenerate.length} degenerate stray-line tris culled`);
  if (clipped) console.log(`  + clip removed ${clipped} tris`);
  console.log(
    `  baked → ${specFile}: ${stats.keptTris} kept / ${stats.removedTris} dropped, ${spec.vertex_count} verts`,
  );
}

const keys = process.argv.slice(2);
const targets = keys.length ? keys : ["chillout", "prisoner", "bloodgulch"];
for (const key of targets) await bake(key);
console.log(
  "\nDone. Reload a visualizer (or the editor) to pick up the baked meshes.",
);
