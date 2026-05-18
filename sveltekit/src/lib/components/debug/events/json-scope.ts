// Tree-View node collection + JSON-scoping helpers for the Events debug tab's
// JSON view. Mirrors xbox/json-scope.ts but the payload type is AnyEvent and
// `scopeAtPath` walks a single event envelope (selected from the tree).
//
// Kept as a pure-TS module (no Svelte/Skeleton imports) so the scoping rules
// can be unit-tested in isolation from the rendering layer.

import type { AnyEvent } from '$lib/types/scraper-v2';

export interface XNode {
	id: string;
	name: string;
	children?: XNode[];
}

// Top-level keys every AnyEvent carries via the embedded EventCommon shape
// plus the discriminator. Render these as leaves at the envelope root so the
// operator can scope the JSON view to one well-known scalar quickly.
const EVENT_SCALAR_KEYS = ['seq', 'tick', 'at', 'event_type'] as const;

/** Root node for the Skeleton TreeView when scoped to a single event
 * envelope. Same shape as xbox/json-scope.ts so the rendering layer can
 * stay verbatim. */
export function buildTreeRoot(env: AnyEvent | null): XNode {
	const root: XNode = { id: '', name: 'event', children: [] };
	if (!env) return root;
	const record = env as unknown as Record<string, unknown>;
	const handled = new Set<string>(EVENT_SCALAR_KEYS);
	for (const k of EVENT_SCALAR_KEYS) {
		if (k in record) root.children!.push({ id: k, name: k });
	}
	// Walk any other top-level keys (player/victim/killer/dealer refs, pos
	// vectors, kind/cause discriminators, item/vehicle refs) so the tree
	// covers everything actually present on this event payload.
	for (const [k, v] of Object.entries(record)) {
		if (handled.has(k)) continue;
		const node: XNode = { id: k, name: k };
		if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
			node.children = walkPayload(v, k);
		}
		root.children!.push(node);
	}
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

/** Scope a single event envelope by the Tree's selected dot-path id,
 * preserving the full event-rooted path as nested wrappers so the operator
 * can still tell where the value lives.
 *  - empty selection         → full event envelope
 *  - any other id            → nested wrappers down to the selected node,
 *                              e.g. selecting 'victim.name' yields
 *                              { victim: { name: '...' } }
 *                              regardless of whether the selected node is
 *                              a branch or a leaf. JSON.stringify renders
 *                              this with standard pretty-print.
 */
export function scopeAtPath(env: AnyEvent | null, selectedValue: string[]): unknown {
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
