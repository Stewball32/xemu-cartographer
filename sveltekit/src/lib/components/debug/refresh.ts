import { adminGet, AdminFetchError } from '$lib/utils/admin-api';
import type { ContainerDetail, InstanceState } from '$lib/types/containers';
import type { ScraperInfo, ScraperInspect } from '$lib/types/scraper';

export type DebugRefreshResult = {
	scraper: InstanceState | null;
	inspect: ScraperInspect | null;
	inspectAt: number | undefined;
};

export async function fetchDebugDetail(
	name: string,
	prev: { scraper: InstanceState | null; inspect: ScraperInspect | null }
): Promise<DebugRefreshResult> {
	let scraper = prev.scraper;
	let inspect = prev.inspect;
	let inspectAt: number | undefined;

	try {
		const res = await adminGet<ContainerDetail>(`containers/${encodeURIComponent(name)}/detail`);
		scraper = res.scraper;
	} catch (err) {
		if (err instanceof AdminFetchError && err.status === 404) {
			try {
				const list = await adminGet<ScraperInfo[] | null>('scraper');
				const match = (list ?? []).find((s) => s.name === name);
				scraper = match
					? {
							name: match.name,
							title_id: match.title_id,
							title: match.title,
							xbox_name: match.xbox_name,
							running: true
						}
					: null;
			} catch (listErr) {
				console.warn('scraper list fetch failed', listErr);
			}
		} else {
			console.warn('detail fetch failed', err);
		}
	}

	try {
		inspect = await adminGet<ScraperInspect>(`scraper/${encodeURIComponent(name)}/inspect`);
		inspectAt = Date.now();
	} catch (err) {
		if (err instanceof AdminFetchError && err.status === 404) {
			inspect = null;
		} else {
			console.warn('inspect fetch failed', err);
		}
	}

	return { scraper, inspect, inspectAt };
}
