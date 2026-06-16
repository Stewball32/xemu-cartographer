import { describe, it, expect } from 'vitest';
import { reducePlayState } from './play-match';

describe('reducePlayState', () => {
	it('treats a resolved container as a match', () => {
		expect(reducePlayState({ container: 'pod-a' })).toEqual({
			phase: 'matched',
			container: 'pod-a'
		});
	});

	it('treats null container as idle', () => {
		expect(reducePlayState({ container: null })).toEqual({ phase: 'idle', container: null });
	});

	it('treats an empty-string container as idle', () => {
		expect(reducePlayState({ container: '' })).toEqual({ phase: 'idle', container: null });
	});

	it('treats a missing container field as idle', () => {
		expect(reducePlayState({})).toEqual({ phase: 'idle', container: null });
	});

	it('degrades a null/undefined response to idle', () => {
		expect(reducePlayState(null)).toEqual({ phase: 'idle', container: null });
		expect(reducePlayState(undefined)).toEqual({ phase: 'idle', container: null });
	});
});
