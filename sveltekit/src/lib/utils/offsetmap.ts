// Pure offsetmap-export parsing (no stores, no env) — shared by the Offsets
// page's client-side import preview and detail table. The shape mirrors what
// the server validates on import: {game, id, offsets:{Name:{value,type}}};
// extra rig fields (confidence/notes/source) are tolerated and ignored.

/** One entry of a parsed offsetmap export. */
export interface OffsetEntry {
	name: string;
	value: string;
	type: string;
}

export interface ParsedOffsetmap {
	game: string;
	id: string;
	description: string;
	entries: OffsetEntry[];
}

/** Parse an offsetmap JSON export; throws with a human message when the text
 * isn't one. Entries come back name-sorted. */
export function parseOffsetmap(text: string): ParsedOffsetmap {
	let doc: unknown;
	try {
		doc = JSON.parse(text);
	} catch {
		throw new Error('not JSON — expected an offsetmap export from the hunting rig');
	}
	const o = doc as {
		game?: unknown;
		id?: unknown;
		description?: unknown;
		offsets?: Record<string, { value?: unknown; type?: unknown }>;
	};
	if (typeof o.game !== 'string' || !o.game || typeof o.id !== 'string' || !o.id) {
		throw new Error('missing game/id — not an offsetmap export');
	}
	const entries: OffsetEntry[] = Object.entries(o.offsets ?? {}).map(([name, en]) => ({
		name,
		value: typeof en?.value === 'string' ? en.value : String(en?.value ?? ''),
		type: typeof en?.type === 'string' ? en.type : ''
	}));
	entries.sort((a, b) => a.name.localeCompare(b.name));
	return {
		game: o.game,
		id: o.id,
		description: typeof o.description === 'string' ? o.description : '',
		entries
	};
}
