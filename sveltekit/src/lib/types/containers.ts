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
