import '@testing-library/jest-dom/vitest';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import PlayCatalog from './PlayCatalog.svelte';
import PlayLobby from './PlayLobby.svelte';
import PlayScoreboard from './PlayScoreboard.svelte';
import type { GamePayload, ScenarioPayload } from '$lib/types/scraper-v2';
import type { IsoOption, OptionsResponse, PlayStatus, ReapView } from '$lib/utils/play-hosting';

// Svelte 5's typed component signature doesn't survive `render()` cleanly for
// components with rich prop types, so cast the call like DataTable.test.ts does.
function renderC(Component: unknown, props: Record<string, unknown>) {
	return render(Component as never, { props } as never);
}

// --- Mock factories -------------------------------------------------------

function mockGame(over: Partial<GamePayload> = {}): GamePayload {
	return {
		phase: 'live',
		started_at: '',
		last_read_at: '',
		engine_tick: 900, // 30s in
		iterations: 0,
		config: {
			gametype: 'team_slayer',
			variant_name: 'Team Slayer',
			is_team_game: true,
			score_limit: 50,
			time_limit_ticks: 0
		},
		team_scores: [
			{ team: 0, score: 12 },
			{ team: 1, score: 9 }
		],
		players: [
			mkPlayer({
				index: 0,
				name: 'Stew',
				team: 0,
				kills: 8,
				deaths: 3,
				assists: 1,
				score: 8,
				is_local: true
			}),
			mkPlayer({ index: 1, name: 'Guest1', team: 1, kills: 5, deaths: 6, assists: 2, score: 5 })
		],
		machines: [
			{ index: 0, name: 'play-abc', is_local: true },
			{ index: 1, name: 'guest-console', is_local: false }
		],
		network: null,
		...over
	};
}

function mkPlayer(o: Partial<GamePayload['players'][number]>): GamePayload['players'][number] {
	return {
		index: 0,
		name: '',
		team: 0,
		armor_color: 0,
		score: 0,
		kills: 0,
		deaths: 0,
		assists: 0,
		ctf_score: 0,
		team_kills: 0,
		suicides: 0,
		kill_streak: 0,
		multikill: 0,
		shots_fired: 0,
		shots_hit: 0,
		is_local: null,
		local_index: null,
		machine_index: null,
		controller_index: null,
		...o
	};
}

function mockStatus(over: Partial<PlayStatus> = {}): PlayStatus {
	return {
		instance: 'play-abc',
		present: true,
		authority: 'runner',
		tick: 0,
		machine_count: 1,
		team_count: 1,
		countdown_active: false,
		ready_to_start: false,
		selected: false,
		ready: false,
		...over
	};
}

function mockOptions(over: Partial<OptionsResponse> = {}): OptionsResponse {
	return {
		instance: 'play-abc',
		available: true,
		maps: [
			{ name: 'Blood Gulch', steps: 0 },
			{ name: 'Sidewinder', steps: 1 }
		],
		gametypes: [
			{ name: 'Team Slayer', steps: 0 },
			{ name: 'CTF', steps: 1 }
		],
		selected_map: '',
		selected_gametype: '',
		...over
	};
}

const scenario: ScenarioPayload = {
	map: 'levels\\test\\bloodgulch\\bloodgulch'
} as ScenarioPayload;

// --- catalog phase --------------------------------------------------------

describe('PlayCatalog (catalog phase)', () => {
	const isos: IsoOption[] = [
		{ id: 'iso1', name: 'Halo: Combat Evolved', title_id: '4d530004', description: 'The classic.' },
		{ id: 'iso2', name: 'Halo 2', title_id: '4d53006e', description: '' }
	];

	it('renders the game grid and fires onrequest with the picked id', async () => {
		const onrequest = vi.fn();
		const { getByRole, getByText } = renderC(PlayCatalog, {
			isos,
			loading: false,
			error: null,
			requestingId: null,
			onrequest
		});
		expect(getByText('Halo: Combat Evolved')).toBeInTheDocument();
		await fireEvent.click(getByRole('button', { name: /Halo: Combat Evolved/i }));
		expect(onrequest).toHaveBeenCalledWith('iso1');
	});

	it('shows a loading state', () => {
		const { getByText } = renderC(PlayCatalog, {
			isos: [],
			loading: true,
			error: null,
			requestingId: null,
			onrequest: vi.fn()
		});
		expect(getByText(/Loading the game library/i)).toBeInTheDocument();
	});

	it('shows the empty state when no games are available', () => {
		const { getByText } = renderC(PlayCatalog, {
			isos: [],
			loading: false,
			error: null,
			requestingId: null,
			onrequest: vi.fn()
		});
		expect(getByText(/No games are available/i)).toBeInTheDocument();
	});
});

// --- lobby phase ----------------------------------------------------------

describe('PlayLobby (lobby phase)', () => {
	function base(over: Record<string, unknown> = {}) {
		return {
			instance: 'play-abc',
			status: mockStatus(),
			options: mockOptions(),
			game: mockGame({ phase: 'idle' }),
			reap: null as ReapView | null,
			busy: false,
			onselect: vi.fn(),
			onteardown: vi.fn(),
			...over
		};
	}

	it('renders the map/gametype picker and the box name', () => {
		const { getByText, getByRole } = renderC(PlayLobby, base());
		expect(getByText('Game setup')).toBeInTheDocument();
		expect(getByText('play-abc')).toBeInTheDocument();
		// The live map list is rendered as options.
		expect(getByRole('option', { name: 'Blood Gulch' })).toBeInTheDocument();
	});

	it('End session calls onteardown', async () => {
		const props = base();
		const { getByRole } = renderC(PlayLobby, props);
		await fireEvent.click(getByRole('button', { name: /End session/i }));
		expect(props.onteardown).toHaveBeenCalledOnce();
	});

	it('surfaces the idle-out reap heads-up', () => {
		const reap: ReapView = {
			reap_at: '2026-07-12T00:00:00Z',
			seconds_remaining: 120,
			warning: true
		};
		const { getByText } = renderC(PlayLobby, base({ reap }));
		expect(getByText(/will be released in/i)).toBeInTheDocument();
		expect(getByText('2m')).toBeInTheDocument();
	});

	it('shows an admin-lockout notice when not runner-controllable', () => {
		const { getByText } = renderC(PlayLobby, base({ status: mockStatus({ authority: 'admin' }) }));
		expect(getByText(/admin is currently controlling/i)).toBeInTheDocument();
	});

	// The "Ready up" gate was REMOVED (2026-08): the runner never presses start —
	// players start the match on the box. The lobby now shows what the box is DOING.
	it('has no Ready up gate', () => {
		const { queryByRole } = renderC(PlayLobby, base({ status: mockStatus({ selected: false }) }));
		expect(queryByRole('button', { name: /Ready up/i })).toBeNull();
	});

	it("surfaces the runner's live activity as the box status", () => {
		const { getByText } = renderC(
			PlayLobby,
			base({ status: mockStatus({ last_reason: 'parked at map-select — awaiting player pick' }) })
		);
		expect(getByText(/parked at map-select/i)).toBeInTheDocument();
	});

	// The scraper reports CE's menu shell scenario ("ui") as the loaded map at the
	// front end; the readback must not show it as a map.
	it('does not show the UI menu scenario as the loaded map', () => {
		const { queryByText } = renderC(PlayLobby, base({ status: mockStatus({ map: 'ui' }) }));
		expect(queryByText(/On the box now/i)).toBeNull();
	});
});

// --- live + postgame phases ----------------------------------------------

describe('PlayScoreboard (live + postgame phases)', () => {
	it('renders team scores, players and K/D for a live match', () => {
		const { getByText } = renderC(PlayScoreboard, {
			game: mockGame(),
			tick: null,
			scenario,
			final: false
		});
		expect(getByText('Team Slayer')).toBeInTheDocument();
		expect(getByText('Stew')).toBeInTheDocument();
		expect(getByText('Guest1')).toBeInTheDocument();
		// K/D/A of the local player.
		expect(getByText('8/3/1')).toBeInTheDocument();
	});

	it('renders a FINAL treatment + back-to-lobby in postgame', async () => {
		const onback = vi.fn();
		const { getByText, getByRole } = renderC(PlayScoreboard, {
			game: mockGame({ phase: 'idle' }),
			tick: null,
			scenario,
			final: true,
			onback
		});
		expect(getByText('FINAL')).toBeInTheDocument();
		await fireEvent.click(getByRole('button', { name: /Back to lobby/i }));
		expect(onback).toHaveBeenCalledOnce();
	});

	it('degrades to a waiting state with no players', () => {
		const { getByText } = renderC(PlayScoreboard, {
			game: mockGame({ players: [], team_scores: [] }),
			tick: null,
			scenario,
			final: false
		});
		expect(getByText(/Waiting for players/i)).toBeInTheDocument();
	});
});
