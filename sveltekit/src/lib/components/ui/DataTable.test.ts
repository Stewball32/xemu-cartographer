import '@testing-library/jest-dom/vitest';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, within } from '@testing-library/svelte';
import DataTable from './DataTable.svelte';
import type { DataColumnGroup } from './data-table';

type Row = { name: string; score: number; team: string };

// `render(DataTable, { props })` doesn't preserve the generic `T` from
// data-table.ts in Svelte 5's typed component signature, so we cast to a
// shape that satisfies the call without erasing our column types at the
// callsite.
function renderTable(props: Record<string, unknown>) {
	return render(DataTable as never, { props } as never);
}

const rows: Row[] = [
	{ name: 'charlie', score: 10, team: 'red' },
	{ name: 'alice', score: 30, team: 'blue' },
	{ name: 'bob', score: 20, team: 'red' }
];

const groups: DataColumnGroup<Row>[] = [
	{ columns: [{ key: 'name' }, { key: 'score' }, { key: 'team' }] }
];

function rowNames(container: HTMLElement): string[] {
	const cells = within(container).getAllByRole('cell');
	const names: string[] = [];
	for (let i = 0; i < cells.length; i += 3) {
		names.push(cells[i].textContent?.trim() ?? '');
	}
	return names;
}

describe('DataTable', () => {
	it('renders rows from props', () => {
		const { container } = renderTable({ rows, groups });
		expect(rowNames(container)).toEqual(['charlie', 'alice', 'bob']);
	});

	it('applies defaultSort on initial render', () => {
		const { container } = renderTable({
			rows,
			groups,
			defaultSort: { key: 'name', dir: 'asc' }
		});
		expect(rowNames(container)).toEqual(['alice', 'bob', 'charlie']);
	});

	it('cycles asc -> desc -> none on header clicks', async () => {
		const { container, getByRole } = renderTable({ rows, groups });
		const nameHeader = getByRole('button', { name: /name/i });

		await fireEvent.click(nameHeader);
		expect(rowNames(container)).toEqual(['alice', 'bob', 'charlie']);

		await fireEvent.click(nameHeader);
		expect(rowNames(container)).toEqual(['charlie', 'bob', 'alice']);

		await fireEvent.click(nameHeader);
		expect(rowNames(container)).toEqual(['charlie', 'alice', 'bob']);
	});

	it('uses secondarySort to break ties', () => {
		const { container } = renderTable({
			rows,
			groups,
			defaultSort: { key: 'team', dir: 'asc' },
			secondarySort: { key: 'name', dir: 'asc' }
		});
		expect(rowNames(container)).toEqual(['alice', 'bob', 'charlie']);
	});

	it('uses comparator when provided', () => {
		const order = ['red', 'blue'];
		const groupsWithCmp: DataColumnGroup<Row>[] = [
			{
				columns: [
					{ key: 'name' },
					{ key: 'score' },
					{
						key: 'team',
						comparator: (a, b) => order.indexOf(a.team) - order.indexOf(b.team)
					}
				]
			}
		];
		const { container } = renderTable({
			rows,
			groups: groupsWithCmp,
			defaultSort: { key: 'team', dir: 'asc' },
			secondarySort: { key: 'name', dir: 'asc' }
		});
		expect(rowNames(container)).toEqual(['bob', 'charlie', 'alice']);
	});

	it('shows loading skeleton when loading and no rows', () => {
		const { container } = renderTable({
			rows: [],
			groups,
			loading: true,
			loadingRows: 3
		});
		const placeholders = container.querySelectorAll('.placeholder');
		expect(placeholders.length).toBe(3 * 3);
	});

	it('shows emptyMessage when not loading and rows empty', () => {
		const { getByText } = renderTable({
			rows: [],
			groups,
			emptyMessage: 'no rows here'
		});
		expect(getByText('no rows here')).toBeInTheDocument();
	});

	it('fires onRowClick on click and on Enter', async () => {
		const onRowClick = vi.fn();
		const { container } = renderTable({ rows, groups, onRowClick });
		const tbody = container.querySelector('tbody');
		const dataRows = tbody?.querySelectorAll('tr') ?? [];
		expect(dataRows.length).toBe(3);

		await fireEvent.click(dataRows[0]);
		expect(onRowClick).toHaveBeenCalledTimes(1);
		expect(onRowClick).toHaveBeenLastCalledWith(rows[0]);

		await fireEvent.keyDown(dataRows[1], { key: 'Enter' });
		expect(onRowClick).toHaveBeenCalledTimes(2);
		expect(onRowClick).toHaveBeenLastCalledWith(rows[1]);
	});

	it('marks non-sortable columns with no header button', () => {
		const groupsMixed: DataColumnGroup<Row>[] = [
			{
				columns: [{ key: 'name' }, { key: 'score' }, { key: 'team', sortable: false }]
			}
		];
		const { getAllByRole } = renderTable({ rows, groups: groupsMixed });
		const buttons = getAllByRole('button');
		expect(buttons.length).toBe(2);
	});
});
