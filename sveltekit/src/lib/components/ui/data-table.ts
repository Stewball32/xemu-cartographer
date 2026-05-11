import type { Snippet } from 'svelte';

export type SortDir = 'asc' | 'desc';

export type SortState = { key: string; dir: SortDir } | null;

export type Density = 'compact' | 'comfortable';

export type DataColumn<T> = {
	key: string;
	label?: string;
	align?: 'left' | 'right';
	width?: string;
	format?: (row: T) => unknown;
	cell?: Snippet<[{ row: T; value: unknown }]>;
	sortable?: boolean;
	sortAccessor?: (row: T) => string | number | boolean | null | undefined;
	comparator?: (a: T, b: T) => number;
};

export type DataColumnGroup<T> = {
	label?: string;
	columns: DataColumn<T>[];
};
