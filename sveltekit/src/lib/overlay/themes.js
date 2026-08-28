// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
// Norcal Halo overlay themes — one accent per surface; star mark is never recolored.
// Redesign deltas: layouts[2] puts BOTH bars at their viewport's bottom (was
// book-ended); ordinals exported for rank labels (CL-15/16).
export const themes = {
	ffa: {
		panel: 'rgba(11, 14, 26, 0.93)',
		accent: '#3d62e0',
		border: 'rgba(61, 98, 224, 0.4)',
		glow: 'rgba(61, 98, 224, 0.3)',
		tagText: 'NORCAL HALO',
		tagColor: '#5d82ff',
		scoreColor: '#f7941d'
	},
	red: {
		panel: 'rgba(44, 11, 11, 0.93)',
		accent: '#e05252',
		border: 'rgba(224, 82, 82, 0.45)',
		glow: 'rgba(224, 82, 82, 0.32)',
		tagText: 'RED TEAM',
		tagColor: '#ff8f85',
		scoreColor: '#e8ecf5'
	},
	blue: {
		panel: 'rgba(10, 16, 44, 0.93)',
		accent: '#3d62e0',
		border: 'rgba(61, 98, 224, 0.45)',
		glow: 'rgba(61, 98, 224, 0.32)',
		tagText: 'BLUE TEAM',
		tagColor: '#7d9cff',
		scoreColor: '#e8ecf5'
	}
};

export const ORANGE = '#f7941d'; // Selection Orange — best-of-stat / leader highlight
export const TEAM_HEX = { red: '#e05252', blue: '#3d62e0' };

// Armor-color chip tint (row background + edge), e.g. tint('#6ec8e8').
export const tint = (hex) => ({ bg: hex + '26', edge: hex + '4D' });

// Ordinal rank labels (CL-15/16) — team chips, FFA rows, post-game.
const ORDINALS = ['1ST', '2ND', '3RD', '4TH', '5TH', '6TH', '7TH', '8TH'];
export const ordinal = (i) => ORDINALS[i] ?? '—';

// POV card anchor per split layout, canvas 1440x1080. Bars are 820×72; quadrant
// cards render at 68%. 2-up: both bars sit at the BOTTOM of their own viewport
// (top seat just above the split, bottom seat at the screen edge).
export const layouts = {
	1: [{ left: '50%', bottom: '24px', tf: 'translateX(-50%)', scale: 1 }],
	2: [
		{ left: '50%', bottom: '556px', tf: 'translateX(-50%)', scale: 1 },
		{ left: '50%', bottom: '24px', tf: 'translateX(-50%)', scale: 1 }
	],
	3: [
		{ left: '50%', bottom: '556px', tf: 'translateX(-50%)', scale: 1 },
		{ left: '360px', bottom: '14px', tf: 'translateX(-50%)', scale: 0.68, origin: 'bottom center' },
		{ left: '1080px', bottom: '14px', tf: 'translateX(-50%)', scale: 0.68, origin: 'bottom center' }
	],
	4: [
		{
			left: '360px',
			bottom: '552px',
			tf: 'translateX(-50%)',
			scale: 0.68,
			origin: 'bottom center'
		},
		{
			left: '1080px',
			bottom: '552px',
			tf: 'translateX(-50%)',
			scale: 0.68,
			origin: 'bottom center'
		},
		{ left: '360px', bottom: '14px', tf: 'translateX(-50%)', scale: 0.68, origin: 'bottom center' },
		{ left: '1080px', bottom: '14px', tf: 'translateX(-50%)', scale: 0.68, origin: 'bottom center' }
	]
};

// Center of each viewport, for the respawn ring.
export const viewportCenters = {
	1: [{ x: 720, y: 540 }],
	2: [
		{ x: 720, y: 270 },
		{ x: 720, y: 810 }
	],
	3: [
		{ x: 720, y: 270 },
		{ x: 360, y: 810 },
		{ x: 1080, y: 810 }
	],
	4: [
		{ x: 360, y: 270 },
		{ x: 1080, y: 270 },
		{ x: 360, y: 810 },
		{ x: 1080, y: 810 }
	]
};

// Kept for callers outside the overlay suite; the redesigned graphics no
// longer zero-pad (CL-08 revised — alignment comes from fixed columns).
export const pad2 = (n) => String(n ?? 0).padStart(2, '0');
