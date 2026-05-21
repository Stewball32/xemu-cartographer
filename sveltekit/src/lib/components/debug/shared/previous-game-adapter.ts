// V2 → v1 adapter for the previous_game envelope class. Originally lived
// in debug/postgame/postgame-vm.ts; lifted to debug/shared/ so it can be
// reused by the Overview tab's previous-match section after the postgame
// folder was retired in favour of the envelope-class `previous_game` tab
// (M6c). The new previous_game tab consumes the v2 payload directly and
// has no need for this adapter — it survives only for v1-shaped consumers.

import type { AnyEvent, EnvelopeV2, PreviousGamePayload } from '$lib/types/scraper-v2';
import type { Envelope, PreviousGameInfo } from '$lib/types/scraper';
import { v2ToV1GameData } from './v2-to-v1-game';

// Map a v2 envelope (event class) to the v1 Envelope shape EventTile /
// EventFeed / EventLogSection consume. Field rename: data → payload.
function v2EnvelopeToV1(env: EnvelopeV2): Envelope {
	const innerData = env.data as AnyEvent | undefined;
	return {
		v: env.v,
		// v2 envelopes inside a previous_game.events list are always
		// "event" — the per-game log records live event broadcasts only.
		type: 'event',
		instance: env.instance,
		tick: env.tick,
		payload: (innerData as unknown as Record<string, unknown>) ?? {}
	};
}

// Translate the v2 PreviousGamePayload (game-class snapshot + per-game
// envelope list) into the v1 PreviousGameInfo shape Overview's previous-
// match section still expects. Returns null when the runner hasn't
// published a previous_game envelope yet.
export function v2ToV1PreviousGame(payload: PreviousGamePayload | null): PreviousGameInfo | null {
	if (!payload) return null;
	const gameData = v2ToV1GameData(payload.game, null);
	const events: Envelope[] = (payload.events ?? []).map(v2EnvelopeToV1);
	return {
		game_data: gameData,
		events,
		ended_at: payload.ended_at
	};
}
