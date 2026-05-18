// Tree-View node collection + JSON-scoping helpers for the Tick debug tab's
// JSON view. Kept as a pure-TS module (no Svelte/Skeleton imports) so the
// scoping rules can be unit-tested in isolation from the rendering layer.

import type { EnvelopeV2, TickPayloadV2 } from '$lib/types/scraper-v2';

export interface XNode {
	id: string;
	name: string;
	children?: XNode[];
}

const ENVELOPE_SCALAR_KEYS = ['v', 'type', 'instance', 'seq', 'tick', 'ts'] as const;

/** Root node for the Skeleton TreeView: envelope-level scalars as leaves
 * plus a `data` branch that walks the payload. Node `id` is the dot-path
 * from the envelope root so the Tree's selectedValue round-trips cleanly
 * into scopeAtPath without a parallel lookup table. */
export function buildTreeRoot(env: EnvelopeV2<TickPayloadV2> | null): XNode {
	const root: XNode = { id: '', name: 'envelope', children: [] };
	if (!env) return root;
	for (const k of ENVELOPE_SCALAR_KEYS) {
		root.children!.push({ id: k, name: k });
	}
	root.children!.push({
		id: 'data',
		name: 'data',
		children: walkPayload(env.data as unknown, 'data')
	});
	return root;
}

function walkPayload(value: unknown, path: string): XNode[] {
	if (value === null || typeof value !== 'object' || Array.isArray(value)) return [];
	return Object.entries(value as Record<string, unknown>).map(([k, v]) => {
		const id = `${path}.${k}`;
		const node: XNode = { id, name: k };
		if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
			node.children = walkPayload(v, id);
		}
		return node;
	});
}

/** Scope an envelope by the Tree's selected dot-path id, preserving the
 * full envelope-rooted path as nested wrappers so the operator can still
 * tell where the value lives.
 *  - empty selection         → full envelope
 *  - any other id            → nested wrappers down to the selected node,
 *                              e.g. selecting 'data.game_globals.active' yields
 *                              { data: { game_globals: { active: 1 } } }
 *                              regardless of whether the selected node is
 *                              a branch or a leaf. JSON.stringify renders
 *                              this with standard pretty-print.
 */
export function scopeAtPath(
	env: EnvelopeV2<TickPayloadV2> | null,
	selectedValue: string[]
): unknown {
	if (!env) return null;
	const id = selectedValue[0];
	if (!id) return env;

	const segments = id.split('.');
	let cursor: unknown = env;
	for (const seg of segments) {
		if (cursor === null || typeof cursor !== 'object') return undefined;
		cursor = (cursor as Record<string, unknown>)[seg];
	}
	// Wrap the resolved value back up through the path, innermost first.
	let result: unknown = cursor;
	for (let i = segments.length - 1; i >= 0; i--) {
		result = { [segments[i]]: result };
	}
	return result;
}
