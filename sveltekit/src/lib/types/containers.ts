// Types mirroring internal/podman/{state.go,ports.go} JSON tags.

export interface Ports {
	xemu_http: number;
	xemu_https: number;
	xemu_ws: number;
	browser_web: number;
	browser_vnc: number;
}

export interface ContainerInfo {
	name: string;
	index: number;
	ports: Ports;
	created: string; // RFC3339
}

// Possible values returned by GET /api/admin/containers/{name}.
// "unknown" is returned by the handler when the container was deleted out from
// under the state file; the rest come from `podman inspect`.
export type ContainerStatus =
	| 'running'
	| 'exited'
	| 'created'
	| 'paused'
	| 'stopping'
	| 'stopped'
	| 'unknown';

export interface ContainerStatusResponse {
	status: ContainerStatus | string;
}

// Mirrors internal/guards/interfaces/scraper.InstanceState. Empty/zero values
// mean the scraper hasn't attached or hasn't resolved that field yet.
export interface InstanceState {
	name: string;
	title_id: number;
	title: string;
	xbox_name: string;
	running: boolean;
}

// GET /api/admin/containers/{name}/detail
export interface ContainerDetail {
	info: ContainerInfo;
	status: ContainerStatus | string;
	scraper: InstanceState | null;
}

export type LogsWhich = 'xemu' | 'browser';

export interface LogsResponse {
	logs: string;
	which: LogsWhich;
}
