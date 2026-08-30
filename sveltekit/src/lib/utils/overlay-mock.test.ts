import { describe, it, expect } from 'vitest';
import {
	mockEvents,
	mockGame,
	mockObjects,
	mockProfiles,
	mockScenario,
	mockTick
} from './overlay-mock';
import type { DeathEvent } from '$lib/types/scraper-v2';

// The mock is a choreography whose whole point is that the overlays' animations
// have something to react to. These assertions pin the properties the motion
// depends on — if a future edit to the kill script breaks one, the preview goes
// quietly static rather than failing loudly, so they're worth holding.

const STEP_FRAMES = 12;
const LOOP_STEPS = 17;
const MATCH_FRAMES = 51 * STEP_FRAMES; // 612
const IDLE_FRAMES = 25;
const CYCLE_FRAMES = MATCH_FRAMES + IDLE_FRAMES; // 637
const LOOP_FRAMES = LOOP_STEPS * STEP_FRAMES; // 204

/** Two full cycles, so wraparound and the match reset are both covered. */
const SWEEP = CYCLE_FRAMES * 2;

describe('mock match cycle', () => {
	it('is live for the whole match and idle for exactly the gap', () => {
		let idle = 0;
		for (let f = 0; f < CYCLE_FRAMES; f++) {
			if (mockGame(f).phase === 'idle') idle++;
		}
		expect(idle).toBe(IDLE_FRAMES);
		// The out-animations latch on live→not-live, so the transition must
		// actually happen inside one cycle, at the match boundary.
		expect(mockGame(MATCH_FRAMES - 1).phase).toBe('live');
		expect(mockGame(MATCH_FRAMES).phase).toBe('idle');
	});

	it('fully resets on the next cycle', () => {
		const first = mockGame(0);
		const second = mockGame(CYCLE_FRAMES);
		expect(second.team_scores).toEqual(first.team_scores);
		expect(second.players.map((p) => p.score)).toEqual(first.players.map((p) => p.score));
		expect(second.engine_tick).toBe(0);
	});

	it('freezes the finished match through the idle gap', () => {
		// The postgame ledger and the scorebug's latched clock both keep reading
		// the final state, so it must not snap back to zero before the reset.
		const final = mockGame(MATCH_FRAMES - 1);
		const duringIdle = mockGame(MATCH_FRAMES + 10);
		expect(duringIdle.team_scores).toEqual(final.team_scores);
	});
});

describe('score bookkeeping', () => {
	it('per-team player scores sum to team_scores on every frame', () => {
		for (let f = 0; f < SWEEP; f += 3) {
			const g = mockGame(f);
			const sums = [0, 0];
			for (const p of g.players) sums[p.team === 1 ? 1 : 0] += p.score;
			const red = g.team_scores.find((t) => t.team === 0)?.score;
			const blue = g.team_scores.find((t) => t.team === 1)?.score;
			expect([red, blue], `frame ${f}`).toEqual(sums);
		}
	});

	it('score equals kills — the ledger prints both columns', () => {
		for (const p of mockGame(400).players) expect(p.score).toBe(p.kills);
	});

	it('red finishes on exactly the score limit', () => {
		const g = mockGame(MATCH_FRAMES - 1);
		const red = g.team_scores.find((t) => t.team === 0)?.score;
		expect(red).toBe(g.config?.score_limit);
	});

	it('hands the lead over exactly once, without blinking through ties', () => {
		// Asserted on sign(red - blue) rather than on "does red lead", because the
		// scorebug highlights NEITHER side at a tie. An earlier tuning satisfied
		// "red takes the lead once" while the orange still blinked off and on six
		// times on the way there — the gap oscillates by 1 as it climbs, and it
		// was climbing right across the tie. Requiring the sign to be monotone
		// catches that: blue ahead → level → red ahead, each state entered once.
		const signs: number[] = [];
		for (let f = 0; f < MATCH_FRAMES; f++) {
			const g = mockGame(f);
			const red = g.team_scores.find((t) => t.team === 0)?.score ?? 0;
			const blue = g.team_scores.find((t) => t.team === 1)?.score ?? 0;
			const sign = Math.sign(red - blue);
			if (!signs.length || signs[signs.length - 1] !== sign) signs.push(sign);
		}
		expect(signs).toEqual([-1, 0, 1]);
	});

	it('spends only a moment level at the crossing', () => {
		// A long stalemate at the handover is the same visual problem in slow
		// motion — no side is highlighted for the duration.
		let level = 0;
		for (let f = 0; f < MATCH_FRAMES; f++) {
			const g = mockGame(f);
			const red = g.team_scores.find((t) => t.team === 0)?.score ?? 0;
			const blue = g.team_scores.find((t) => t.team === 1)?.score ?? 0;
			if (red === blue) level++;
		}
		expect(level).toBeLessThanOrEqual(3 * STEP_FRAMES); // ≤ ~7s
	});
});

describe('respawn timers', () => {
	it('drain strictly, stay positive to the last dead frame, then clear', () => {
		// The ring sweeps its arc off the unrounded seconds — a timer that only
		// changed once a second would step instead of draining.
		const seen: number[] = [];
		let sawDeath = false;
		for (let f = 0; f < LOOP_FRAMES * 2; f++) {
			const gravemind = mockTick(f).players[1];
			if (!gravemind.alive) {
				sawDeath = true;
				const ticks = gravemind.respawn_in_ticks;
				expect(ticks, `frame ${f}`).toBeGreaterThan(0);
				if (seen.length) expect(ticks, `frame ${f}`).toBeLessThan(seen[seen.length - 1]);
				seen.push(ticks as number);
			} else if (seen.length) {
				// The frame after the last dead one: alive, no timer.
				expect(gravemind.respawn_in_ticks).toBeNull();
				expect(seen[0]).toBe(90); // 15 dead frames × 6 ticks
				expect(seen[seen.length - 1]).toBe(6);
				seen.length = 0;
			}
		}
		expect(sawDeath).toBe(true);
	});

	it('nobody is respawning once the match is over', () => {
		for (let f = MATCH_FRAMES; f < CYCLE_FRAMES; f++) {
			for (const p of mockTick(f).players) expect(p.alive).toBe(true);
		}
	});
});

describe('kill script', () => {
	it('exercises every death cause within one loop', () => {
		// Each cause is a distinct render branch in the kill feed and in the
		// respawn ring's plate (named killer vs RESPAWNING).
		const causes = new Set<string>();
		for (let f = 0; f < LOOP_FRAMES; f++) {
			for (const ev of mockEvents(f)) {
				if (ev.event_type === 'death') causes.add((ev as DeathEvent).cause);
			}
		}
		expect([...causes].sort()).toEqual([
			'betrayal',
			'environment',
			'fall',
			'kill',
			'suicide',
			'unknown'
		]);
	});

	it('never has a player get a kill while they are still dead', () => {
		// A victim is out for 15 frames but steps land every 12, so a player who
		// died last step is still down when the next one fires. Check each step at
		// the exact frame it lands: whoever it credits must be on their feet.
		for (let step = 0; step * STEP_FRAMES < MATCH_FRAMES; step++) {
			const f = step * STEP_FRAMES;
			const landed = mockEvents(f)[0] as DeathEvent;
			expect(landed.tick, `step ${step}`).toBe(f * 6);
			if (landed.killer == null) continue;
			expect(mockTick(f).players[landed.killer.index].alive, `step ${step}`).toBe(true);
		}
	});

	it('zeroes a victim streak on the frame they die', () => {
		// The spree badge cascades off this transition.
		const stewballDeathStep = 3; // Regret kills Stewball, 4th step of the loop
		const before = mockGame(stewballDeathStep * STEP_FRAMES - 1).players[0].kill_streak;
		const after = mockGame(stewballDeathStep * STEP_FRAMES).players[0].kill_streak;
		expect(before).toBeGreaterThan(0);
		expect(after).toBe(0);
	});

	it('builds a streak worth showing before it breaks', () => {
		let best = 0;
		for (let f = 0; f < MATCH_FRAMES; f++) {
			for (const p of mockGame(f).players) best = Math.max(best, p.kill_streak);
		}
		expect(best).toBeGreaterThanOrEqual(3); // 3 is where the badge turns orange
	});

	it('keeps the event log newest-first', () => {
		const evs = mockEvents(300);
		expect(evs.length).toBeGreaterThan(1);
		for (let i = 1; i < evs.length; i++) expect(evs[i].tick).toBeLessThan(evs[i - 1].tick);
	});

	it('gives a local both a named killer and an unattributed death each loop', () => {
		// gravemind (seat 1) is the KILLED BY / RESPAWNING demo.
		const causes: Array<string | null> = [];
		for (let f = 0; f < LOOP_FRAMES; f++) {
			for (const ev of mockEvents(f)) {
				const d = ev as DeathEvent;
				if (d.event_type === 'death' && d.victim.index === 1) {
					causes.push(d.killer ? d.killer.name : null);
				}
			}
		}
		expect(causes.some((c) => typeof c === 'string')).toBe(true); // KILLED BY plate
		expect(causes.some((c) => c === null)).toBe(true); // RESPAWNING pill
	});
});

describe('power-up windows', () => {
	it('cloaks a local for a stretch of every loop', () => {
		let camoFrames = 0;
		for (let f = 0; f < LOOP_FRAMES; f++) {
			if (mockTick(f).players[0].has_camo) camoFrames++;
		}
		expect(camoFrames).toBeGreaterThan(10); // long enough to watch the wipe
	});

	it('drains an overshield from above 1 back down to 1', () => {
		// The card's conic rings render nothing at shields ≤ 1, so a mock that
		// clamps to 1 can never preview them.
		const shields: number[] = [];
		for (let f = 0; f < LOOP_FRAMES; f++) {
			const p = mockTick(f).players[0];
			if (p.has_overshield) shields.push(p.shields);
		}
		expect(shields.length).toBeGreaterThan(10);
		expect(Math.max(...shields)).toBeGreaterThan(1.5);
		expect(shields[shields.length - 1]).toBeLessThan(shields[0]);
		expect(Math.min(...shields)).toBeGreaterThanOrEqual(1);
	});

	it('never cloaks or overshields a dead player', () => {
		for (let f = 0; f < SWEEP; f += 2) {
			for (const p of mockTick(f).players) {
				if (!p.alive) {
					expect(p.has_camo).toBe(false);
					expect(p.has_overshield).toBe(false);
				}
			}
		}
	});
});

describe('frame-independent exports', () => {
	it('mockScenario and mockObjects are pure and zero-arg', () => {
		expect(mockScenario()).toEqual(mockScenario());
		expect(mockObjects()).toEqual(mockObjects());
	});

	it('mockProfiles is stable — the store snapshots it exactly once', () => {
		// Anything frame-dependent in here would silently freeze at frame 0.
		expect(mockProfiles()).toEqual(mockProfiles());
	});

	it('previews the plate banner on exactly one seat', () => {
		const withPlate = Object.values(mockProfiles()).filter((p) => p.plate);
		expect(withPlate).toHaveLength(1);
	});

	it('carries each seed armor colour into its player ref', () => {
		// seedRef used to publish the TEAM index as armor_color, which collapsed
		// every red player to palette entry 0.
		const colors = new Set<number>();
		for (const ev of mockEvents(MATCH_FRAMES - 1)) {
			const d = ev as DeathEvent;
			colors.add(d.victim.armor_color);
			if (d.killer) colors.add(d.killer.armor_color);
		}
		expect(colors.size).toBeGreaterThan(2);
	});
});
