type Mode = 'light' | 'dark';

function createModeStore() {
	const initial: Mode =
		typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
			? 'dark'
			: 'light';

	let current = $state<Mode>(initial);

	function apply(next: Mode) {
		current = next;
		if (typeof document === 'undefined') return;
		document.documentElement.classList.toggle('dark', next === 'dark');
		try {
			localStorage.setItem('mode', next);
		} catch {
			// localStorage may be unavailable (private mode, quota, disabled)
		}
	}

	return {
		get current() {
			return current;
		},
		toggle() {
			apply(current === 'dark' ? 'light' : 'dark');
		},
		set(next: Mode) {
			apply(next);
		}
	};
}

export const mode = createModeStore();
