import { getContext, setContext } from 'svelte';
import type { ScraperInspect } from '$lib/types/scraper';

export type DebugContext = {
	readonly inspect: ScraperInspect | null;
	readonly inspectAt: number | undefined;
	readonly showAll: boolean;
	readonly now: number;
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
