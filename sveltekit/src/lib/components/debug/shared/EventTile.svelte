<script lang="ts">
	// One event mini-card. Renders the play-by-play summary (player names tinted
	// by armor color in FFA) plus a per-bucket footer with whatever extras the
	// payload carries — coordinates for spawns, item tags for pickups, K/D/A for
	// score events, etc. Falls back to bare tick number when the payload is
	// uninteresting. Click → events tab.

	import {
		ActivityIcon,
		FlagIcon,
		PackageIcon,
		SwordIcon,
		UserCircle2Icon,
		ZapIcon
	} from '@lucide/svelte';
	import type { Envelope } from '$lib/types/scraper';
	import { armorLabel, armorTextClass } from './util';
	import { eventBucket, eventTypeOf, summarizeEvent, type EventRosterRef } from './events-vm';
	import StatTile from './StatTile.svelte';

	let {
		env,
		players,
		isTeamGame = false,
		latestTick
	}: {
		env: Envelope;
		players: EventRosterRef[];
		isTeamGame?: boolean;
		latestTick: number | undefined;
	} = $props();

	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';

	const bucket = $derived(eventBucket(env));
	const eventType = $derived(eventTypeOf(env) || env.type);
	const summary = $derived(summarizeEvent(env, players));

	const statusKind = $derived<StatusKind>(
		bucket === 'combat'
			? 'off'
			: bucket === 'pickup'
				? 'warn'
				: bucket === 'powerup'
					? 'on'
					: bucket === 'match'
						? 'info'
						: bucket === 'player'
							? 'pointer'
							: 'neutral'
	);

	function fmtCoord(v: unknown): string {
		if (typeof v !== 'number' || !Number.isFinite(v)) return '?';
		return v.toFixed(1);
	}

	// Per-bucket footer text. Surfaces whatever extras the payload carries that
	// the summary line doesn't already show (coordinates, item tags, K/D/A
	// breakdowns). Returns the raw tick when there's nothing better — keeps the
	// footer bar visually consistent across heterogeneous events.
	const footerText = $derived.by(() => {
		const t = eventTypeOf(env);
		const p = (env.payload ?? {}) as Record<string, unknown>;
		switch (t) {
			case 'spawn':
				return `@ (${fmtCoord(p.x)}, ${fmtCoord(p.y)}, ${fmtCoord(p.z)})`;
			case 'kill':
			case 'team_kill':
				return p.cause !== undefined ? `cause: ${String(p.cause)}` : `tick #${env.tick}`;
			case 'damage':
				return p.amount !== undefined ? `dmg ${String(p.amount)}` : `tick #${env.tick}`;
			case 'score':
				return `K${p.kills ?? 0} / D${p.deaths ?? 0} / A${p.assists ?? 0}`;
			case 'multikill':
				return `×${p.count ?? '?'}`;
			case 'kill_streak':
				return `streak ${p.count ?? '?'}`;
			case 'item_picked_up':
			case 'item_dropped':
			case 'item_spawned':
			case 'item_depleted':
				return p.tag !== undefined ? String(p.tag) : `tick #${env.tick}`;
			case 'powerup_picked_up':
			case 'powerup_expired':
				return String(p.tag ?? p.kind ?? `tick #${env.tick}`);
			case 'team_score':
				return `team ${p.team ?? '?'} → ${p.score ?? '?'}`;
			case 'game_start':
			case 'game_end':
				return `${p.gametype ?? '—'} on ${p.map ?? '—'}`;
			default:
				return `tick #${env.tick}`;
		}
	});

	// Time-ago chip. Engine ticks at 30Hz, so divide by 30 for seconds. Hidden
	// when latestTick isn't known or the event is from the future (clock skew /
	// stale snapshot during phase transitions).
	const trend = $derived.by<{ delta: string; direction: 'flat' } | undefined>(() => {
		if (latestTick === undefined || latestTick <= 0) return undefined;
		const dTicks = latestTick - env.tick;
		if (dTicks < 0) return undefined;
		const dSec = dTicks / 30;
		let label: string;
		if (dSec < 1) label = 'now';
		else if (dSec < 60) label = `${dSec.toFixed(0)}s`;
		else if (dSec < 3600) label = `${(dSec / 60).toFixed(0)}m`;
		else label = `${(dSec / 3600).toFixed(1)}h`;
		return { delta: label, direction: 'flat' };
	});
</script>

{#snippet bucketIcon()}
	{#if bucket === 'combat'}
		<SwordIcon class="size-3.5" />
	{:else if bucket === 'pickup'}
		<PackageIcon class="size-3.5" />
	{:else if bucket === 'powerup'}
		<ZapIcon class="size-3.5" />
	{:else if bucket === 'match'}
		<FlagIcon class="size-3.5" />
	{:else if bucket === 'player'}
		<UserCircle2Icon class="size-3.5" />
	{:else}
		<ActivityIcon class="size-3.5" />
	{/if}
{/snippet}

{#snippet summaryValue()}
	{#each summary as part, i (i)}{#if part.kind === 'text'}{part.text}{:else if isTeamGame}{part.name}{:else}<span
				class="font-semibold {armorTextClass(part.armorColor)}"
				title={armorLabel(part.armorColor)}>{part.name}</span
			>{/if}{/each}
{/snippet}

<StatTile
	label={eventType}
	{statusKind}
	title="{eventType} @ tick {env.tick}"
	icon={bucketIcon}
	value={summaryValue}
	footer={footerText}
	{trend}
	href="#events"
/>
