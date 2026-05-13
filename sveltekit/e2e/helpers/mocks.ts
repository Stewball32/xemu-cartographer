// Playwright mock-API helper. Intercepts the /api/admin/* endpoints the /pod/*
// routes consume and returns realistic fixture JSON so every screenshot has
// populated, deterministic data.
//
// Three containers in the fixture: one Live Halo: CE match (so debug + probe
// pages render their richest state), one Ready instance (between matches),
// one stopped. Workers should not modify this file.

import type { Page, Route } from '@playwright/test';

const NOW = '2026-05-12T07:30:00Z';
const STARTED = '2026-05-12T07:00:00Z';

const CONTAINER_LIVE = {
	name: 'live-host',
	index: 0,
	ports: {
		xemu_http: 3100,
		xemu_https: 3101,
		xemu_ws: 3102,
		browser_web: 3103,
		browser_vnc: 3104
	},
	created: STARTED
};
const CONTAINER_READY = {
	name: 'ready-host',
	index: 1,
	ports: {
		xemu_http: 3105,
		xemu_https: 3106,
		xemu_ws: 3107,
		browser_web: 3108,
		browser_vnc: 3109
	},
	created: STARTED
};
const CONTAINER_STOPPED = {
	name: 'stopped-host',
	index: 2,
	ports: {
		xemu_http: 3110,
		xemu_https: 3111,
		xemu_ws: 3112,
		browser_web: 3113,
		browser_vnc: 3114
	},
	created: STARTED
};
const CONTAINERS = [CONTAINER_LIVE, CONTAINER_READY, CONTAINER_STOPPED];

const STATUS_BY_NAME: Record<string, string> = {
	'live-host': 'running',
	'ready-host': 'running',
	'stopped-host': 'exited'
};

const SCRAPER_BASE = {
	sock: '/tmp/qmp/live-host.sock',
	title_id: 0x4d530102,
	title: 'Halo: Combat Evolved',
	xbox_name: 'Whicker',
	serial_number: '712998856516',
	mac_address: '0050f296b66f',
	video_standard: 'NTSC-M',
	xbe_title_name: 'H1 Performance Build (server)',
	xbe_version: 8,
	xbe_game_region: 1,
	xbe_allowed_media: 127,
	kernel_system_time: NOW,
	kernel_boot_time: STARTED,
	kernel_uptime_ns: 1800000000000,
	tick: 5907,
	ticks: 23868,
	started_at: STARTED
};

const SCRAPER_INFOS = [
	{ ...SCRAPER_BASE, name: 'live-host' },
	{
		...SCRAPER_BASE,
		name: 'ready-host',
		sock: '/tmp/qmp/ready-host.sock',
		xbox_name: 'Cupid'
	}
];

const GAME_DATA_LIVE = {
	map: 'levels\\test\\temple\\temple',
	gametype: 'slayer',
	variant_name: 'TS TRAINING',
	is_team_game: true,
	score_limit: 50,
	time_limit_ticks: 0,
	team_scores: [
		{ team: 0, score: 12 },
		{ team: 1, score: 7 }
	],
	players: [
		{
			index: 0,
			name: 'Howard',
			team: 0,
			armor_color: 0,
			score: 5,
			kills: 5,
			deaths: 2,
			assists: 1,
			ctf_score: 0,
			team_kills: 0,
			suicides: 0,
			kill_streak: 3,
			multikill: 1,
			shots_fired: 120,
			shots_hit: 48,
			is_local: true,
			local_index: 0,
			machine_index: 0,
			controller_index: 0
		},
		{
			index: 1,
			name: 'Goat',
			team: 0,
			armor_color: 1,
			score: 7,
			kills: 7,
			deaths: 3,
			assists: 0,
			ctf_score: 0,
			team_kills: 0,
			suicides: 1,
			kill_streak: 2,
			multikill: 0,
			shots_fired: 90,
			shots_hit: 38,
			is_local: false,
			local_index: null,
			machine_index: 1,
			controller_index: 0
		},
		{
			index: 2,
			name: 'Penguin',
			team: 1,
			armor_color: 2,
			score: 7,
			kills: 7,
			deaths: 5,
			assists: 2,
			ctf_score: 0,
			team_kills: 0,
			suicides: 0,
			kill_streak: 1,
			multikill: 0,
			shots_fired: 110,
			shots_hit: 44,
			is_local: false,
			local_index: null,
			machine_index: 2,
			controller_index: 0
		}
	],
	power_item_spawns: null,
	machines: [
		{ index: 0, name: 'Whicker', is_local: true },
		{ index: 1, name: 'Cupid', is_local: false },
		{ index: 2, name: 'Crazy', is_local: false }
	],
	game_difficulty: 1,
	player_spawns: [
		{
			index: 0,
			x: 6.5,
			y: -2.2,
			z: -4.5,
			facing: 1.57,
			team_index: 0,
			bsp_index: 0,
			unk_0: 0,
			gametype_0: 13,
			gametype_1: 0,
			gametype_2: 0,
			gametype_3: 0
		},
		{
			index: 1,
			x: -6.7,
			y: 2.0,
			z: -4.5,
			facing: -1.58,
			team_index: 1,
			bsp_index: 0,
			unk_0: 0,
			gametype_0: 13,
			gametype_1: 0,
			gametype_2: 0,
			gametype_3: 0
		}
	],
	fog: {
		color_r: 0,
		color_g: 0,
		color_b: 0,
		max_density: 0,
		atmo_min_dist: 1024,
		atmo_max_dist: 2048
	},
	object_types: [
		{ type_index: 0, name: 'biped', datum_size: 1232 },
		{ type_index: 1, name: 'vehicle', datum_size: 1604 },
		{ type_index: 2, name: 'weapon', datum_size: 824 }
	],
	tag_cache: {
		game_state_base: 2147880960,
		game_state_size: 3428352,
		tag_cache_base: 2151309312,
		tag_cache_size: 23068672,
		texture_cache_base: 2191392768,
		texture_cache_size: 23068672,
		sound_cache_base: 2187198464,
		sound_cache_size: 4194304
	}
};

const TICK_PAYLOAD = {
	players: [
		{
			index: 0,
			alive: true,
			respawn_in_ticks: null,
			x: 1.5,
			y: 0.2,
			z: -4.5,
			vx: 0,
			vy: 0,
			vz: 0,
			aim_x: 1,
			aim_y: 0,
			aim_z: 0,
			zoom_level: -1,
			crouchscale: 0,
			health: 1,
			shields: 1,
			has_camo: false,
			has_overshield: false,
			frags: 2,
			plasmas: 1,
			selected_weapon_slot: 0,
			is_crouching: false,
			is_jumping: false,
			is_firing: false,
			is_shooting: false,
			is_flashlight_on: false,
			is_throwing_grenade: false,
			is_meleeing: false,
			is_pressing_action: false,
			is_holding_action: false,
			weapons: [
				{
					slot: 0,
					object_id: 38,
					tag: 'weapons\\assault rifle\\assault rifle',
					ammo_pack: 180,
					ammo_mag: 60,
					is_energy: false
				}
			]
		}
	],
	power_items: null,
	game_globals: {
		map_loaded: 1,
		active: 1,
		players_are_double_speed: 0,
		game_loading_in_progress: 0,
		precache_map_status: 1,
		game_difficulty_level: 1,
		stored_global_random: 52072
	},
	player_count: 3,
	local_count: 1,
	locals: [],
	network: {
		client: {
			machine_index: 0,
			ping_target_ip: 0,
			packets_sent: 0,
			packets_received: 0,
			average_ping: 0,
			ping_active: 0,
			seconds_to_game_start: -1
		},
		server: { countdown_active: 1, countdown_paused: 0, countdown_adjusted_time: 0 },
		game_data: { maximum_player_count: 16, machine_count: 3, player_count: 3 },
		machines: [
			{ index: 0, name: 'Whicker' },
			{ index: 1, name: 'Cupid' }
		],
		network_players: []
	},
	ctf_flags: null,
	objects: null,
	projectiles: null
};

const EVENTS = [
	{
		type: 'event',
		instance: 'live-host',
		tick: 100,
		payload: { event_type: 'kill', killer: 0, victim: 2, cause: 'pistol' }
	},
	{
		type: 'event',
		instance: 'live-host',
		tick: 50,
		payload: { event_type: 'spawn', player: 0, x: 6.5, y: -2.2, z: -4.5 }
	},
	{
		type: 'event',
		instance: 'live-host',
		tick: 3,
		payload: { event_type: 'game_start', gametype: 'slayer', map: 'temple' }
	}
];

const SCORE_PROBE_LIVE: Record<string, unknown> = {
	team_0_score_at_0x80123400: 12,
	team_1_score_at_0x80123404: 7,
	gametype_engine_ptr: 0x80456000,
	player_0_kills_at_0x80789000: 5,
	player_0_deaths_at_0x80789004: 2
};

const STATE_INPUTS_LIVE: Record<string, unknown> = {
	main_menu: false,
	initialized: true,
	active: true,
	paused: false,
	engine_running: true,
	game_can_score: true,
	game_engine_globals_ptr: 0x80456000,
	game_time_globals_ptr: 0x80456100
};

function inspectFor(name: string): unknown {
	if (name === 'live-host') {
		return {
			...SCRAPER_INFOS[0],
			running: true,
			phase: 'live',
			last_read_at: NOW,
			state_inputs: STATE_INPUTS_LIVE,
			score_probe: SCORE_PROBE_LIVE,
			game_data: GAME_DATA_LIVE,
			latest_tick: TICK_PAYLOAD,
			recent_events: EVENTS,
			previous_game: null
		};
	}
	if (name === 'ready-host') {
		return {
			...SCRAPER_INFOS[1],
			running: true,
			phase: 'ready',
			last_read_at: NOW,
			state_inputs: { ...STATE_INPUTS_LIVE, active: false, game_can_score: false },
			score_probe: {},
			game_data: null,
			latest_tick: null,
			recent_events: [],
			previous_game: null
		};
	}
	return null;
}

function statusFor(name: string): { status: string } {
	return { status: STATUS_BY_NAME[name] ?? 'unknown' };
}

function detailFor(name: string): unknown {
	const info =
		name === 'live-host'
			? CONTAINER_LIVE
			: name === 'ready-host'
				? CONTAINER_READY
				: CONTAINER_STOPPED;
	const scraper =
		name === 'live-host'
			? {
					name,
					title_id: SCRAPER_BASE.title_id,
					title: SCRAPER_BASE.title,
					xbox_name: 'Whicker',
					running: true
				}
			: name === 'ready-host'
				? {
						name,
						title_id: SCRAPER_BASE.title_id,
						title: SCRAPER_BASE.title,
						xbox_name: 'Cupid',
						running: true
					}
				: null;
	return { info, status: STATUS_BY_NAME[name] ?? 'unknown', scraper };
}

function json(body: unknown, status = 200) {
	return {
		status,
		contentType: 'application/json',
		body: typeof body === 'string' ? body : JSON.stringify(body)
	};
}

function extractName(url: string): string {
	// Pull the {name} segment out of /api/admin/containers/{name}/... or
	// /api/admin/scraper/{name}/... regardless of any query string.
	const u = new URL(url);
	const parts = u.pathname.split('/').filter(Boolean);
	// ['api', 'admin', 'containers' | 'scraper', '<name>', ...]
	return parts[3] ?? '';
}

export async function installPodMocks(page: Page): Promise<void> {
	await page.route('**/api/admin/containers', (route: Route) => {
		if (route.request().method() === 'POST') {
			return route.fulfill(json(CONTAINER_LIVE, 201));
		}
		return route.fulfill(json(CONTAINERS));
	});

	await page.route('**/api/admin/containers/cleanup', (route) =>
		route.fulfill(json({ deleted: [] }))
	);

	await page.route('**/api/admin/containers/*/start', (route) => route.fulfill({ status: 204 }));
	await page.route('**/api/admin/containers/*/stop', (route) => route.fulfill({ status: 204 }));
	await page.route('**/api/admin/containers/*/files', (route) => route.fulfill({ status: 204 }));
	await page.route('**/api/admin/containers/*/detail', (route) => {
		const name = extractName(route.request().url());
		return route.fulfill(json(detailFor(name)));
	});
	await page.route('**/api/admin/containers/*/logs**', (route) =>
		route.fulfill(json({ logs: '(mock logs)\n', which: 'xemu' }))
	);
	// Catch-all for /api/admin/containers/{name} (status GET + DELETE).
	// Must come after the more-specific patterns above.
	await page.route('**/api/admin/containers/*', (route: Route) => {
		const method = route.request().method();
		const name = extractName(route.request().url());
		if (method === 'DELETE') return route.fulfill({ status: 204 });
		return route.fulfill(json(statusFor(name)));
	});

	await page.route('**/api/admin/scraper', (route) => route.fulfill(json(SCRAPER_INFOS)));
	await page.route('**/api/admin/scraper/*/inspect', (route) => {
		const name = extractName(route.request().url());
		const inspect = inspectFor(name);
		if (inspect === null) {
			return route.fulfill(json({ error: 'not found' }, 404));
		}
		return route.fulfill(json(inspect));
	});
	await page.route('**/api/admin/scraper/*/start', (route) => route.fulfill({ status: 204 }));
	await page.route('**/api/admin/scraper/*/stop', (route) => route.fulfill({ status: 204 }));

	// Ad-hoc xemu probe endpoints — small canned JSON so the /pod/probe page's
	// "Run" buttons return something useful for screenshots.
	await page.route('**/api/admin/xemu/probe**', (route) =>
		route.fulfill(
			json({
				sock: '/tmp/qmp/live-host.sock',
				pid: 12345,
				base_hva: 0x7f0000000000,
				title_id: 0x4d530102
			})
		)
	);
	await page.route('**/api/admin/xemu/probe-title**', (route) =>
		route.fulfill(
			json({
				sock: '/tmp/qmp/live-host.sock',
				samples: [{ tick: 0, value: 0x4d530102, magic: 'XBEH' }]
			})
		)
	);
	await page.route('**/api/admin/xemu/sample-deltas**', (route) =>
		route.fulfill(json({ sock: '/tmp/qmp/live-host.sock', deltas: [] }))
	);
	await page.route('**/api/admin/xemu/scan-string**', (route) =>
		route.fulfill(json({ sock: '/tmp/qmp/live-host.sock', matches: [] }))
	);
}

// Re-exported for specs that need to refer to canonical seeded names.
export const MOCK_INSTANCE_LIVE = 'live-host';
export const MOCK_INSTANCE_READY = 'ready-host';
export const MOCK_INSTANCE_STOPPED = 'stopped-host';
