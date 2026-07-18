// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
// Shared query-param parsing for overlay routes.
// ?ws=ws://localhost:8765&names=STEW:Stewart,CARTO:Carter
export function overlayParams(url) {
	return {
		ws: url.searchParams.get('ws') ?? 'ws://localhost:8765',
		layout: Number(url.searchParams.get('layout') ?? 1),
		names: Object.fromEntries(
			(url.searchParams.get('names') ?? '')
				.split(',')
				.filter(Boolean)
				.map((pair) => pair.split(':'))
		)
	};
}
