import { adminGet } from '$lib/utils/admin-api';
import type { LogsResponse, LogsWhich } from '$lib/types/containers';

export const LOGS_TAIL = 200;

// Duplicated from /containers/[name]/+page.svelte (rather than extracted) to
// keep that page's tightly-coupled VNC + kiosk + logs state self-contained.
export async function fetchContainerLogs(name: string, which: LogsWhich): Promise<string> {
	const res = await adminGet<LogsResponse>(
		`containers/${encodeURIComponent(name)}/logs?which=${which}&tail=${LOGS_TAIL}`
	);
	return res.logs;
}
