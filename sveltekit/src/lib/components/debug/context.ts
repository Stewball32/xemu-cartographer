import { getContext, setContext } from 'svelte';
import type { ScraperInspect } from '$lib/types/scraper';

/** Tabs that support both formatted and raw views read viewMode from this
 * context so the choice persists across tabs (an operator who toggles JSON
 * on the xbox tab sees JSON when they later open the scenario tab). The
 * page owns the state + localStorage write; tabs only call setViewMode. */
export type ViewMode = 'pretty' | 'json';

export type DebugContext = {
	readonly inspect: ScraperInspect | null;
	readonly inspectAt: number | undefined;
	readonly now: number;
	/** Optional so pages that don't host Pretty/JSON-capable tabs (e.g.
	 * /pod/probe/) can skip wiring these up; tab consumers default to
	 * 'pretty' + a no-op setter when absent. */
	readonly viewMode?: ViewMode;
	setViewMode?: (next: ViewMode) => void;
	relativeTime: (ts: number | undefined) => string;
};

const KEY = Symbol('debug-context');

export function setDebugContext(ctx: DebugContext): void {
	setContext(KEY, ctx);
}

export function useDebugContext(): DebugContext {
	const ctx = getContext<DebugContext | undefined>(KEY);
	if (!ctx) throw new Error('useDebugContext: no DebugContext in scope');
	return ctx;
}
