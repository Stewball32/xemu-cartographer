// Per-field validation annotations for the admin debug page.
//
// Each field gets a three-state mark — "ok", "broken", "unknown" — that the
// user flips during a manual validation walk. State is persisted in
// localStorage so it survives reloads and tab navigation. Keyed by an
// arbitrary path string (typically `<instance>:<tab>:<group>.<field>`); the
// caller is responsible for picking a stable shape.
//
// The harness is the manual half of M19's runtime offset validation: M19
// will compare runtime probe results against these manual marks to flag
// drifted offsets. Until M19 lands, the export-as-JSON button on the debug
// page is the only readout.

export type FieldMark = 'ok' | 'broken' | 'unknown';

const STORAGE_KEY = 'xc:field-annotations';

function readStorage(): Record<string, FieldMark> {
	if (typeof localStorage === 'undefined') return {};
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return {};
		const parsed = JSON.parse(raw) as Record<string, FieldMark>;
		return parsed && typeof parsed === 'object' ? parsed : {};
	} catch {
		return {};
	}
}

function writeStorage(map: Record<string, FieldMark>): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
	} catch {
		// localStorage full or disabled — annotations stay in-memory only.
	}
}

function createStore() {
	let map = $state<Record<string, FieldMark>>(readStorage());

	function get(key: string): FieldMark {
		return map[key] ?? 'unknown';
	}

	function set(key: string, mark: FieldMark): void {
		const next = { ...map };
		if (mark === 'unknown') delete next[key];
		else next[key] = mark;
		map = next;
		writeStorage(map);
	}

	// Cycle: unknown → ok → broken → unknown
	function cycle(key: string): void {
		const current = get(key);
		const order: FieldMark[] = ['unknown', 'ok', 'broken'];
		const next = order[(order.indexOf(current) + 1) % order.length];
		set(key, next);
	}

	function clearAll(): void {
		map = {};
		writeStorage(map);
	}

	function clearPrefix(prefix: string): void {
		const next: Record<string, FieldMark> = {};
		for (const [k, v] of Object.entries(map)) {
			if (!k.startsWith(prefix)) next[k] = v;
		}
		map = next;
		writeStorage(map);
	}

	function exportJSON(prefix?: string): string {
		const subset = prefix
			? Object.fromEntries(Object.entries(map).filter(([k]) => k.startsWith(prefix)))
			: map;
		return JSON.stringify(subset, null, 2);
	}

	return {
		get all() {
			return map;
		},
		get,
		set,
		cycle,
		clearAll,
		clearPrefix,
		exportJSON
	};
}

export const fieldAnnotations = createStore();
