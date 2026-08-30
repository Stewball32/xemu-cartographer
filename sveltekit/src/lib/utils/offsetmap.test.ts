import { describe, expect, it } from 'vitest';
import { parseOffsetmap } from './offsetmap';

const valid = JSON.stringify({
	game: 'haloce',
	id: 'nhe-1.2',
	description: 'NHE retune',
	offsets: {
		kda_table: { value: '0x0032A6D8', type: 'u32[3]', confidence: 'verified' },
		player_block: { value: '0x0032A480', type: 'struct' },
		ui_path: { value: 'ui\\shell\\x', type: 'string' }
	}
});

describe('parseOffsetmap', () => {
	it('parses a rig export, name-sorted, tolerating extra fields', () => {
		const p = parseOffsetmap(valid);
		expect(p.game).toBe('haloce');
		expect(p.id).toBe('nhe-1.2');
		expect(p.description).toBe('NHE retune');
		expect(p.entries.map((e) => e.name)).toEqual(['kda_table', 'player_block', 'ui_path']);
		expect(p.entries[0]).toEqual({ name: 'kda_table', value: '0x0032A6D8', type: 'u32[3]' });
	});

	it('rejects non-JSON with a human message', () => {
		expect(() => parseOffsetmap('{nope')).toThrow(/not JSON/);
	});

	it('rejects JSON that is not an offsetmap (missing game/id)', () => {
		expect(() => parseOffsetmap('{"foo": 1}')).toThrow(/missing game\/id/);
		expect(() => parseOffsetmap('{"game": "haloce"}')).toThrow(/missing game\/id/);
	});

	it('tolerates an absent offsets object (zero entries)', () => {
		const p = parseOffsetmap('{"game": "haloce", "id": "empty-set"}');
		expect(p.entries).toEqual([]);
	});
});
