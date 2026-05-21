// Shared Tree-View + JSON-scope helpers for the debug page's Pretty/JSON view.
// Pure-TS (no Svelte imports) so the scoping logic is unit-testable in
// isolation. Used by every envelope-style debug tab through JsonView.svelte,
// and by the events tab through EventsJson.svelte (which wraps a rolling log
// of single events).

export interface XNode {
	id: string;
	name: string;
	children?: XNode[];
}

/** Shape descriptor that lets buildTreeRoot/scopeAtPath share code across the
 * two envelope conventions in use:
 *  - V2 envelopes wrap their payload under `data`; the tree shows data's
 *    children directly and the JSON output prepends the envelope scalars.
 *  - Events have no `data` wrapper; selectable fields live at the top level
 *    alongside the scalars, and selections flatten beside them in the output.
 */
export interface EnvelopeShape {
	dataKey: 'data' | null;
	scalarKeys: readonly string[];
}

export const V2_ENVELOPE_SHAPE: EnvelopeShape = {
	dataKey: 'data',
	scalarKeys: ['v', 'type', 'instance', 'seq', 'tick', 'ts']
};

export const EVENT_SHAPE: EnvelopeShape = {
	dataKey: null,
	scalarKeys: ['seq', 'tick', 'at', 'event_type']
};

const ARRAY_SENTINEL = '[]';

function isPlainObject(v: unknown): v is Record<string, unknown> {
	return v !== null && typeof v === 'object' && !Array.isArray(v);
}

function isArrayOfObjects(v: unknown): v is Record<string, unknown>[] {
	return Array.isArray(v) && v.length > 0 && v.every(isPlainObject);
}

/** Build the TreeView root. The root itself is invisible (the rendering layer
 * iterates `rootNode.children`); its children are the payload keys. For V2
 * that's the keys of `env.data`; for events the top-level keys of the event
 * minus the scalar keys. Scalar keys are intentionally not selectable — they
 * always appear in the output so there's nothing for the operator to toggle. */
export function buildTreeRoot(env: unknown, shape: EnvelopeShape): XNode {
	const rootName = shape.dataKey === 'data' ? 'envelope' : 'event';
	const root: XNode = { id: '', name: rootName, children: [] };
	if (!isPlainObject(env)) return root;

	let basePayload: Record<string, unknown>;
	let basePath: string;
	if (shape.dataKey === 'data') {
		const data = env['data'];
		if (!isPlainObject(data)) return root;
		basePayload = data;
		basePath = 'data';
	} else {
		basePayload = env;
		basePath = '';
	}

	for (const [k, v] of Object.entries(basePayload)) {
		if (shape.scalarKeys.includes(k)) continue;
		const id = basePath ? `${basePath}.${k}` : k;
		root.children!.push(nodeFor(k, v, id));
	}
	return root;
}

function nodeFor(name: string, value: unknown, id: string): XNode {
	if (isPlainObject(value)) {
		return { id, name, children: childrenForObject(value, id) };
	}
	if (isArrayOfObjects(value)) {
		// `[arr]` label signals projection: selecting a child here projects
		// that field across every element of this array.
		return {
			id,
			name: `${name} [arr]`,
			children: childrenForArrayOfObjects(value, `${id}[]`)
		};
	}
	return { id, name };
}

function childrenForObject(obj: Record<string, unknown>, basePath: string): XNode[] {
	return Object.entries(obj).map(([k, v]) => nodeFor(k, v, `${basePath}.${k}`));
}

/** Union-of-keys template for an array of objects: merges every element's
 * keys (and nested object / array-of-objects shapes) so projectable fields
 * aren't hidden when an array is heterogeneous — previous_game.data.events
 * is a mix of death/damage/medal envelopes; first-element keys would drop
 * everything but one event type's fields. */
function childrenForArrayOfObjects(arr: Record<string, unknown>[], basePath: string): XNode[] {
	const template: Record<string, unknown> = {};
	for (const el of arr) {
		for (const [k, v] of Object.entries(el)) {
			if (!(k in template)) {
				template[k] = v;
				continue;
			}
			const existing = template[k];
			if (isPlainObject(existing) && isPlainObject(v)) {
				template[k] = mergeObjects(existing, v);
			} else if (isArrayOfObjects(existing) && isArrayOfObjects(v)) {
				template[k] = [...existing, ...v];
			}
		}
	}
	return childrenForObject(template, basePath);
}

function mergeObjects(
	a: Record<string, unknown>,
	b: Record<string, unknown>
): Record<string, unknown> {
	const out: Record<string, unknown> = { ...a };
	for (const [k, v] of Object.entries(b)) {
		if (!(k in out)) {
			out[k] = v;
			continue;
		}
		const existing = out[k];
		if (isPlainObject(existing) && isPlainObject(v)) {
			out[k] = mergeObjects(existing, v);
		} else if (isArrayOfObjects(existing) && isArrayOfObjects(v)) {
			out[k] = [...existing, ...v];
		}
	}
	return out;
}

/** Parse a tree-node id ("data.players[].name") into walk segments
 * ("data", "players", "[]", "name"). `[]` marks "project the remainder of the
 * path across every element of the array at this point". */
function parsePath(path: string): string[] {
	const out: string[] = [];
	for (const seg of path.split('.')) {
		if (seg.endsWith('[]')) {
			const base = seg.slice(0, -2);
			if (base) out.push(base);
			out.push(ARRAY_SENTINEL);
		} else if (seg) {
			out.push(seg);
		}
	}
	return out;
}

/** Walk segments from `cursor`, rebuilding wrappers innermost-first so the
 * returned value preserves the selected path's nesting. Missing keys and
 * non-descendable cursors emit `null` at that position so the path shape
 * stays visible (and indexes line up when two paths project the same array). */
function walkAndWrap(cursor: unknown, segments: string[]): unknown {
	if (segments.length === 0) return cursor === undefined ? null : cursor;
	const [seg, ...rest] = segments;

	if (seg === ARRAY_SENTINEL) {
		if (!Array.isArray(cursor)) return null;
		return cursor.map((el) => walkAndWrap(el, rest));
	}

	let child: unknown = null;
	if (isPlainObject(cursor)) {
		const v = cursor[seg];
		if (v !== undefined) child = v;
	}
	return { [seg]: walkAndWrap(child, rest) };
}

function deepMerge(target: unknown, source: unknown): unknown {
	if (target === undefined) return source;
	if (source === undefined) return target;
	if (Array.isArray(target) && Array.isArray(source)) {
		const len = Math.max(target.length, source.length);
		const result: unknown[] = [];
		for (let i = 0; i < len; i++) {
			result.push(deepMerge(target[i], source[i]));
		}
		return result;
	}
	if (isPlainObject(target) && isPlainObject(source)) {
		const result: Record<string, unknown> = { ...target };
		for (const [k, v] of Object.entries(source)) {
			result[k] = deepMerge(result[k], v);
		}
		return result;
	}
	return source;
}

/** Scope an envelope by the TreeView's selected dot-path ids. Multi-select is
 * supported: every path is walked and deep-merged into one slice. The output
 * always carries the envelope scalars at the top so the operator sees what
 * envelope they're inspecting regardless of selection.
 *
 *  - empty selection           → full envelope (verbatim)
 *  - V2 paths, e.g. 'data.xbe.title_name'
 *      → { v, type, instance, seq, tick, ts, data: { xbe: { title_name } } }
 *  - V2 array projection, e.g. 'data.players[].name'
 *      → { …scalars, data: { players: [{name:'A'},{name:'B'},…] } }
 *  - event paths, e.g. 'victim.name'
 *      → { seq, tick, at, event_type, victim: { name } }   (no data wrapper)
 */
export function scopeAtPath(env: unknown, selectedValue: string[], shape: EnvelopeShape): unknown {
	if (!isPlainObject(env)) return env ?? null;
	if (selectedValue.length === 0) return env;

	const scalars: Record<string, unknown> = {};
	for (const k of shape.scalarKeys) {
		if (k in env) scalars[k] = env[k];
	}

	const walkRoot: unknown = shape.dataKey === 'data' ? (env['data'] ?? null) : env;
	let slice: unknown = undefined;
	for (const path of selectedValue) {
		const segments = parsePath(path);
		let inner = segments;
		if (shape.dataKey === 'data' && inner[0] === 'data') {
			inner = inner.slice(1);
		}
		const wrapped = inner.length === 0 ? walkRoot : walkAndWrap(walkRoot, inner);
		slice = deepMerge(slice, wrapped);
	}

	if (shape.dataKey === 'data') {
		return { ...scalars, data: slice ?? null };
	}
	if (isPlainObject(slice)) {
		return { ...scalars, ...slice };
	}
	return scalars;
}
