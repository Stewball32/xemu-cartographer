const AUTH_PATHS = ['/login/', '/register/', '/forgot-password/', '/reset-password/'];

export function safeRedirectTarget(raw: string | null | undefined): string {
	if (!raw || typeof raw !== 'string') return '/';
	if (!raw.startsWith('/')) return '/';
	if (raw.startsWith('//') || raw.startsWith('/\\')) return '/';
	const pathOnly = raw.split('?')[0].split('#')[0];
	if (AUTH_PATHS.some((p) => pathOnly === p || pathOnly.startsWith(p))) return '/';
	return raw;
}

function buildAuthUrl(base: '/login/' | '/register/', target: string): string {
	const safe = safeRedirectTarget(target);
	if (safe === '/') return base;
	return `${base}?${new URLSearchParams({ redirect: safe }).toString()}`;
}

export function buildLoginUrl(target: string): string {
	return buildAuthUrl('/login/', target);
}

export function buildRegisterUrl(target: string): string {
	return buildAuthUrl('/register/', target);
}
