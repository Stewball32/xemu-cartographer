<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { ArrowLeftIcon, ChevronDownIcon } from '@lucide/svelte';
	import { Tabs, Accordion, SegmentedControl, Switch } from '@skeletonlabs/skeleton-svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { adminGet, AdminFetchError } from '$lib/utils/admin-api';
	import type { ContainerDetail, InstanceState } from '$lib/types/containers';
	import type {
		ScraperInfo,
		ScraperInspect,
		SnapshotPlayer,
		TickPlayer,
		Envelope
	} from '$lib/types/scraper';
	import JsonTree from '$lib/components/JsonTree.svelte';
	import OverviewCard from '$lib/components/debug/OverviewCard.svelte';
	import KvCard from '$lib/components/debug/KvCard.svelte';
	import PlayerListItem from '$lib/components/debug/PlayerListItem.svelte';
	import PlayerDetailPanel from '$lib/components/debug/PlayerDetailPanel.svelte';
	import ColGroupedTable from '$lib/components/debug/ColGroupedTable.svelte';
	import type { ColGroup } from '$lib/components/debug/col-grouped-table';

	let { data } = $props();
	const name = $derived(data.name);

	let scraper = $state<InstanceState | null>(null);
	let inspect = $state<ScraperInspect | null>(null);
	let inspectAt = $state<number | undefined>(undefined);
	let now = $state(Date.now());

	// Persisted "show all data" toggle. Off (default) hides sections that
	// don't apply to the current state/gametype.
	let showAll = $state(false);
	let topTab = $state('overview');
	let tickTab = $state('players');
	let eventFilter = $state<'all' | 'match' | 'player' | 'combat' | 'pickup' | 'powerup'>('all');
	let selectedPlayerIdx = $state<number | null>(null);

	let pollTimer: ReturnType<typeof setInterval> | null = null;
	let nowTimer: ReturnType<typeof setInterval> | null = null;

	async function refreshDetail() {
		try {
			const res = await adminGet<ContainerDetail>(
				`containers/${encodeURIComponent(name)}/detail`
			);
			scraper = res.scraper;
		} catch (err) {
			if (err instanceof AdminFetchError && err.status === 404) {
				try {
					const list = await adminGet<ScraperInfo[] | null>('scraper');
					const match = (list ?? []).find((s) => s.name === name);
					scraper = match
						? {
								name: match.name,
								title_id: match.title_id,
								game_title: match.game_title,
								xbox_name: match.xbox_name,
								running: true
							}
						: null;
				} catch (listErr) {
					console.warn('scraper list fetch failed', listErr);
				}
			} else {
				console.warn('detail fetch failed', err);
			}
		}

		try {
			inspect = await adminGet<ScraperInspect>(`scraper/${encodeURIComponent(name)}/inspect`);
			inspectAt = Date.now();
		} catch (err) {
			if (err instanceof AdminFetchError && err.status === 404) {
				inspect = null;
			} else {
				console.warn('inspect fetch failed', err);
			}
		}
	}

	onMount(() => {
		// Restore showAll preference.
		try {
			const saved = localStorage.getItem('debug.showAll');
			if (saved !== null) showAll = saved === 'true';
		} catch {
			// localStorage unavailable; keep default.
		}
		if (auth.token) {
			scraperWS.connect(auth.token);
		}
		refreshDetail();
		pollTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			refreshDetail();
		}, 3000);
		nowTimer = setInterval(() => {
			now = Date.now();
		}, 1000);
	});

	$effect(() => {
		try {
			localStorage.setItem('debug.showAll', String(showAll));
		} catch {
			// ignore
		}
	});

	onDestroy(() => {
		scraperWS.disconnect();
		if (pollTimer) clearInterval(pollTimer);
		if (nowTimer) clearInterval(nowTimer);
	});

	function relativeTime(ts: number | undefined): string {
		if (!ts) return 'never';
		const diffMs = now - ts;
		if (diffMs < 1000) return 'just now';
		if (diffMs < 60_000) return `${Math.floor(diffMs / 1000)}s ago`;
		if (diffMs < 3_600_000) return `${Math.floor(diffMs / 60_000)}m ago`;
		return `${Math.floor(diffMs / 3_600_000)}h ago`;
	}

	const snapshot = $derived(inspect?.latest_snapshot ?? scraperWS.snapshots[name] ?? null);
	const tick = $derived(scraperWS.ticks[name] ?? inspect?.latest_tick ?? null);
	const events = $derived(scraperWS.events[name] ?? inspect?.recent_events ?? []);
	const stateInputs = $derived(inspect?.state_inputs ?? null);
	const scoreProbe = $derived(inspect?.score_probe ?? null);
	const snapshotAt = $derived(
		inspect?.latest_snapshot ? inspectAt : scraperWS.snapshotsAt[name]
	);
	const tickAt = $derived(scraperWS.ticksAt[name]);

	const currentState = $derived(inspect?.current_state ?? '');
	const isMatchState = $derived(
		currentState === 'in_game' || currentState === 'pregame' || currentState === 'postgame'
	);
	const runnerAttached = $derived(!!inspect?.running || !!scraper?.running);
	const isTeamGame = $derived(snapshot?.is_team_game === true);
	const gametype = $derived(snapshot?.gametype ?? '');
	const isCTF = $derived(gametype === 'ctf');

	// Engine tick: prefer the WS value (refreshes ~30Hz in_game) over the
	// 3s HTTP poll, otherwise the counter visibly stutters.
	const tickValue = $derived(scraperWS.tickNumbers[name] ?? inspect?.tick);

	// Auto-select first player when tick arrives.
	$effect(() => {
		if (selectedPlayerIdx === null && tick?.players && tick.players.length > 0) {
			selectedPlayerIdx = tick.players[0].index;
		}
	});

	function snapshotPlayerFor(idx: number): SnapshotPlayer | null {
		return snapshot?.players?.find((p) => p.index === idx) ?? null;
	}

	const selectedTickPlayer = $derived<TickPlayer | null>(
		tick?.players?.find((p) => p.index === selectedPlayerIdx) ?? null
	);
	const selectedSnapPlayer = $derived(
		selectedPlayerIdx !== null ? snapshotPlayerFor(selectedPlayerIdx) : null
	);

	// Object table column groups (visual grouping of related fields).
	const objectGroups: ColGroup[] = [
		{
			label: 'Identity',
			columns: [
				{ key: 'object_id', label: 'id' },
				{ key: 'tag' },
				{ key: 'type' }
			]
		},
		{ label: 'Flags', columns: [{ key: 'flags' }] },
		{
			label: 'Position',
			columns: [{ key: 'x' }, { key: 'y' }, { key: 'z' }]
		},
		{
			label: 'Angular Velocity',
			columns: [
				{ key: 'ang_vel_x', label: 'ω_x' },
				{ key: 'ang_vel_y', label: 'ω_y' },
				{ key: 'ang_vel_z', label: 'ω_z' }
			]
		},
		{
			label: 'Damage / Refs',
			columns: [
				{ key: 'unk_damage_1', label: 'dmg_1' },
				{ key: 'time_existing', label: 'age' },
				{ key: 'owner_unit_ref', label: 'owner_unit' },
				{ key: 'owner_object_ref', label: 'owner_obj' },
				{ key: 'ultimate_parent', label: 'parent' }
			]
		}
	];

	const projectileGroups: ColGroup[] = [
		{
			label: 'Identity',
			columns: [
				{ key: 'object_id', label: 'id' },
				{ key: 'tag' }
			]
		},
		{
			label: 'Position',
			columns: [{ key: 'x' }, { key: 'y' }, { key: 'z' }]
		},
		{
			label: 'State',
			columns: [
				{ key: 'flags' },
				{ key: 'action' },
				{ key: 'hit_material_type', label: 'hit_mat' },
				{ key: 'ignore_object_index', label: 'ignore_obj' },
				{ key: 'target_object_index', label: 'target_obj' }
			]
		},
		{
			label: 'Timers',
			columns: [
				{ key: 'detonation_timer', label: 'det_t' },
				{ key: 'detonation_timer_delta', label: 'det_Δ' },
				{ key: 'arming_time_delta', label: 'arm_Δ' },
				{ key: 'deceleration_timer', label: 'dec_t' },
				{ key: 'deceleration_timer_delta', label: 'dec_Δ' },
				{ key: 'deceleration', label: 'dec' }
			]
		},
		{
			label: 'Trajectory',
			columns: [
				{ key: 'distance_traveled', label: 'dist' },
				{ key: 'maximum_damage_distance', label: 'max_dmg_dist' },
				{ key: 'rotation_axis_x', label: 'rot_x' },
				{ key: 'rotation_axis_y', label: 'rot_y' },
				{ key: 'rotation_axis_z', label: 'rot_z' },
				{ key: 'rotation_sine', label: 'sin' },
				{ key: 'rotation_cosine', label: 'cos' }
			]
		}
	];

	// Event filter buckets.
	const matchEvents = new Set(['game_start', 'game_end', 'team_score']);
	const playerEvents = new Set([
		'player_joined',
		'player_left',
		'player_team_changed',
		'player_quit',
		'spawn'
	]);
	const combatEvents = new Set([
		'kill',
		'death',
		'damage',
		'melee',
		'team_kill',
		'multikill',
		'kill_streak',
		'score',
		'grenade_thrown',
		'vehicle_entered',
		'vehicle_exited'
	]);
	const pickupEvents = new Set(['item_picked_up', 'item_dropped', 'item_spawned', 'item_depleted']);
	const powerupEvents = new Set(['powerup_picked_up', 'powerup_expired']);

	function eventBucket(ev: Envelope): string {
		const innerType = (ev.payload as { type?: string } | undefined)?.type ?? ev.type;
		if (matchEvents.has(innerType)) return 'match';
		if (playerEvents.has(innerType)) return 'player';
		if (combatEvents.has(innerType)) return 'combat';
		if (pickupEvents.has(innerType)) return 'pickup';
		if (powerupEvents.has(innerType)) return 'powerup';
		return 'other';
	}

	const filteredEvents = $derived(
		eventFilter === 'all' ? events : events.filter((e) => eventBucket(e) === eventFilter)
	);
</script>

<div class="container mx-auto max-w-7xl p-4">
	<header class="mb-4">
		<a class="anchor mb-2 flex items-center gap-1 text-sm" href={resolve('/admin/debug/')}>
			<ArrowLeftIcon class="size-4" />
			Back to debug
		</a>
		<div class="flex flex-wrap items-center justify-between gap-4">
			<div>
				<h1 class="h2">{name}</h1>
				<p class="text-surface-700-200 text-sm">
					{#if scraper?.running}
						<span class="badge preset-filled-success-500 mr-2 text-xs">Scraper running</span>
					{:else}
						<span class="badge preset-tonal mr-2 text-xs">Scraper idle</span>
					{/if}
					{scraper?.game_title || '—'} · {scraper?.xbox_name || '—'}
				</p>
			</div>
			<div class="flex items-center gap-4">
				<label class="flex items-center gap-2 text-xs">
					<Switch
						checked={showAll}
						onCheckedChange={(d) => (showAll = d.checked)}
						name="debug-show-all"
					>
						<Switch.Control>
							<Switch.Thumb />
						</Switch.Control>
						<Switch.HiddenInput />
					</Switch>
					<span>Show all data</span>
				</label>
				<div class="text-right text-xs">
					<div>
						WS:
						<span class:text-success-500={scraperWS.connected}
							>{scraperWS.connected ? 'connected' : 'disconnected'}</span
						>
					</div>
					{#if scraperWS.lastError}
						<div class="text-error-500">{scraperWS.lastError}</div>
					{/if}
				</div>
			</div>
		</div>
	</header>

	{#if !runnerAttached && !snapshot && !tick}
		<div class="card preset-tonal p-6 text-center">
			<p class="mb-2">No scraper attached for this instance.</p>
			<p class="text-surface-700-200 text-sm">
				Start it from <a class="anchor" href={resolve(`/containers/${name}/`)}
					>/containers/{name}/</a
				>.
			</p>
		</div>
	{:else}
		<Tabs value={topTab} onValueChange={(d) => (topTab = d.value)}>
			<Tabs.List>
				<Tabs.Trigger value="overview">Overview</Tabs.Trigger>
				<Tabs.Trigger value="snapshot">Snapshot</Tabs.Trigger>
				<Tabs.Trigger value="tick">Tick</Tabs.Trigger>
				<Tabs.Trigger value="events">Events</Tabs.Trigger>
				<Tabs.Trigger value="probe">Probe</Tabs.Trigger>
				<Tabs.Trigger value="raw">Raw JSON</Tabs.Trigger>
				<Tabs.Indicator />
			</Tabs.List>

			<!-- OVERVIEW TAB -->
			<Tabs.Content value="overview" class="pt-4">
				<div class="text-surface-700-200 mb-3 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4">
					<div>Snapshot: <span class="font-mono">{relativeTime(snapshotAt)}</span></div>
					<div>Tick: <span class="font-mono">{relativeTime(tickAt)}</span></div>
					<div>State: <span class="font-mono">{currentState || '—'}</span></div>
					<div>Events buffered: <span class="font-mono tabular-nums">{events.length}</span></div>
				</div>
				<OverviewCard
					state={currentState}
					{snapshot}
					{tick}
					{tickValue}
					{stateInputs}
					{showAll}
				/>
			</Tabs.Content>

			<!-- SNAPSHOT TAB -->
			<Tabs.Content value="snapshot" class="pt-4">
				{#if !snapshot}
					<div class="card preset-tonal text-surface-500-400 p-6 text-center">
						No snapshot yet. {isMatchState
							? 'Should appear shortly.'
							: 'Snapshots populate during pregame, in-game, and postgame.'}
					</div>
				{:else}
					<Accordion value={['match-config', 'players']} multiple>
						<Accordion.Item value="match-config">
							<Accordion.ItemTrigger
								class="flex w-full items-center justify-between gap-2 py-2 text-left"
							>
								<span class="font-semibold">Match Config</span>
								<Accordion.ItemIndicator>
									<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
								</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-3">
								{@const matchScalars = {
									game_state: snapshot.game_state,
									map: snapshot.map,
									gametype: snapshot.gametype,
									is_team_game: snapshot.is_team_game,
									score_limit: snapshot.score_limit,
									time_limit_ticks: snapshot.time_limit_ticks,
									game_difficulty: snapshot.game_difficulty,
									...(isTeamGame || showAll
										? {
												team_scores:
													snapshot.team_scores
														?.map((t) => `team ${t.team}: ${t.score}`)
														.join(', ') ?? '—'
											}
										: {})
								}}
								<KvCard value={matchScalars} />
							</Accordion.ItemContent>
						</Accordion.Item>

						<hr class="hr" />

						<Accordion.Item value="players">
							<Accordion.ItemTrigger
								class="flex w-full items-center justify-between gap-2 py-2 text-left"
							>
								<span class="font-semibold"
									>Players <span class="text-surface-700-200 font-normal"
										>({snapshot.players?.length ?? 0})</span
									></span
								>
								<Accordion.ItemIndicator>
									<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
								</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-3">
								<JsonTree value={snapshot.players ?? []} depth={1} />
							</Accordion.ItemContent>
						</Accordion.Item>

						<hr class="hr" />

						<Accordion.Item value="map-layout">
							<Accordion.ItemTrigger
								class="flex w-full items-center justify-between gap-2 py-2 text-left"
							>
								<span class="font-semibold">Map Layout</span>
								<Accordion.ItemIndicator>
									<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
								</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-3">
								<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
									<div>
										<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
											Player Spawns ({snapshot.player_spawns?.length ?? 0})
										</div>
										<JsonTree value={snapshot.player_spawns ?? []} depth={1} />
									</div>
									<div>
										<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
											Power Item Spawns ({snapshot.power_item_spawns?.length ?? 0})
										</div>
										<JsonTree value={snapshot.power_item_spawns ?? []} depth={1} />
									</div>
								</div>
							</Accordion.ItemContent>
						</Accordion.Item>

						<hr class="hr" />

						<Accordion.Item value="diagnostic">
							<Accordion.ItemTrigger
								class="flex w-full items-center justify-between gap-2 py-2 text-left"
							>
								<span class="font-semibold">Diagnostic (fog, object types, tag cache)</span>
								<Accordion.ItemIndicator>
									<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
								</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-3">
								<div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
									<KvCard
										title="fog"
										value={snapshot.fog as unknown as Record<string, unknown> | null}
									/>
									<div>
										<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
											object_types ({snapshot.object_types?.length ?? 0})
										</div>
										<JsonTree value={snapshot.object_types ?? []} depth={1} />
									</div>
									<KvCard
										title="tag_cache"
										value={snapshot.tag_cache as unknown as Record<string, unknown> | null}
									/>
								</div>
							</Accordion.ItemContent>
						</Accordion.Item>
					</Accordion>
				{/if}
			</Tabs.Content>

			<!-- TICK TAB -->
			<Tabs.Content value="tick" class="pt-4">
				{#if !tick}
					<div class="card preset-tonal text-surface-500-400 p-6 text-center">
						No tick data — only emitted while in a match.
						{#if currentState}
							<div class="text-surface-700-200 mt-2 text-xs">
								Current state: <code>{currentState}</code>
							</div>
						{/if}
					</div>
				{:else}
					<Tabs value={tickTab} onValueChange={(d) => (tickTab = d.value)}>
						<Tabs.List>
							<Tabs.Trigger value="players"
								>Players <span class="text-surface-700-200 ml-1 text-xs"
									>({tick.players?.length ?? 0})</span
								></Tabs.Trigger
							>
							<Tabs.Trigger value="network">Network</Tabs.Trigger>
							<Tabs.Trigger value="objects"
								>Objects <span class="text-surface-700-200 ml-1 text-xs"
									>({tick.objects?.length ?? 0})</span
								></Tabs.Trigger
							>
							<Tabs.Trigger value="projectiles"
								>Projectiles <span class="text-surface-700-200 ml-1 text-xs"
									>({tick.projectiles?.length ?? 0})</span
								></Tabs.Trigger
							>
							{#if isCTF || showAll}
								<Tabs.Trigger value="ctf"
									>CTF Flags <span class="text-surface-700-200 ml-1 text-xs"
										>({tick.ctf_flags?.length ?? 0})</span
									></Tabs.Trigger
								>
							{/if}
							<Tabs.Trigger value="locals"
								>Locals <span class="text-surface-700-200 ml-1 text-xs"
									>({tick.locals?.length ?? 0})</span
								></Tabs.Trigger
							>
							<Tabs.Trigger value="misc">Misc</Tabs.Trigger>
							<Tabs.Indicator />
						</Tabs.List>

						<!-- Players sub-tab: master-detail -->
						<Tabs.Content value="players" class="pt-3">
							{#if !tick.players || tick.players.length === 0}
								<div class="card preset-tonal text-surface-500-400 p-3 text-sm">no players</div>
							{:else}
								<div class="grid grid-cols-1 gap-3 lg:grid-cols-[18rem_1fr]">
									<div class="space-y-2">
										{#each tick.players as p (p.index)}
											<PlayerListItem
												tickPlayer={p}
												snapshotPlayer={snapshotPlayerFor(p.index)}
												selected={selectedPlayerIdx === p.index}
												teamGame={isTeamGame}
												onSelect={() => (selectedPlayerIdx = p.index)}
											/>
										{/each}
									</div>
									<div>
										{#if selectedTickPlayer}
											<PlayerDetailPanel
												tickPlayer={selectedTickPlayer}
												snapshotPlayer={selectedSnapPlayer}
											/>
										{:else}
											<div class="card preset-tonal text-surface-500-400 p-6 text-center text-sm">
												Select a player from the list to inspect.
											</div>
										{/if}
									</div>
								</div>
							{/if}
						</Tabs.Content>

						<!-- Network sub-tab -->
						<Tabs.Content value="network" class="pt-3">
							{#if !tick.network}
								<div class="card preset-tonal text-surface-500-400 p-3 text-sm">
									no network data (singleplayer or unreplicated)
								</div>
							{:else}
								<div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
									<KvCard
										title="client"
										value={tick.network.client as unknown as Record<string, unknown> | null}
										emptyMessage="no client data"
									/>
									<KvCard
										title="server"
										value={tick.network.server as unknown as Record<string, unknown> | null}
										emptyMessage="no server data"
									/>
									<KvCard
										title="game_data"
										value={tick.network.game_data as unknown as Record<string, unknown> | null}
										emptyMessage="no game_data"
									/>
								</div>
								<div class="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
									<div>
										<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
											Machines ({tick.network.machines?.length ?? 0})
										</div>
										<JsonTree value={tick.network.machines ?? []} depth={1} />
									</div>
									<div>
										<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
											Network Players ({tick.network.network_players?.length ?? 0})
										</div>
										<JsonTree value={tick.network.network_players ?? []} depth={1} />
									</div>
								</div>
							{/if}
						</Tabs.Content>

						<!-- Objects sub-tab -->
						<Tabs.Content value="objects" class="pt-3">
							<ColGroupedTable
								rows={(tick.objects ?? []) as unknown as Record<string, unknown>[]}
								groups={objectGroups}
								stickyFirst
								emptyMessage="no objects"
							/>
						</Tabs.Content>

						<!-- Projectiles sub-tab -->
						<Tabs.Content value="projectiles" class="pt-3">
							<ColGroupedTable
								rows={(tick.projectiles ?? []) as unknown as Record<string, unknown>[]}
								groups={projectileGroups}
								stickyFirst
								emptyMessage="no projectiles"
							/>
						</Tabs.Content>

						<!-- CTF sub-tab -->
						{#if isCTF || showAll}
							<Tabs.Content value="ctf" class="pt-3">
								{#if !isCTF && showAll}
									<div class="text-surface-500-400 mb-2 text-xs">(unused — non-CTF gametype)</div>
								{/if}
								<JsonTree value={tick.ctf_flags ?? []} depth={1} />
							</Tabs.Content>
						{/if}

						<!-- Locals sub-tab -->
						<Tabs.Content value="locals" class="pt-3">
							{#if !tick.locals || tick.locals.length === 0}
								<div class="card preset-tonal text-surface-500-400 p-3 text-sm">no locals</div>
							{:else}
								<div class="space-y-4">
									{#each tick.locals as local}
										<div class="card preset-tonal p-3">
											<div class="mb-2 text-sm font-semibold">
												local_index: {local.local_index}
												<span class="text-surface-700-200 ml-2 font-mono text-xs">
													look {local.look_yaw_rate.toFixed(3)} / {local.look_pitch_rate.toFixed(3)}
												</span>
											</div>
											<div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
												<KvCard
													title="fp_weapon"
													value={local.fp_weapon as unknown as Record<string, unknown> | null}
													emptyMessage="—"
												/>
												<KvCard
													title="observer_cam"
													value={local.observer_cam as unknown as Record<string, unknown> | null}
													emptyMessage="—"
												/>
												<KvCard
													title="player_control"
													value={local.player_control as unknown as Record<string, unknown> | null}
													emptyMessage="—"
												/>
												<KvCard
													title="ias"
													value={local.ias as unknown as Record<string, unknown> | null}
													emptyMessage="—"
												/>
												<KvCard
													title="gamepad"
													value={local.gamepad as unknown as Record<string, unknown> | null}
													emptyMessage="—"
												/>
												<KvCard
													title="ui"
													value={local.ui as unknown as Record<string, unknown> | null}
													emptyMessage="—"
												/>
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</Tabs.Content>

						<!-- Misc sub-tab -->
						<Tabs.Content value="misc" class="pt-3">
							<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
								<KvCard
									title="game_globals"
									value={tick.game_globals as unknown as Record<string, unknown> | null}
								/>
								<KvCard
									title="data_queue"
									value={tick.data_queue as unknown as Record<string, unknown> | null}
								/>
								<KvCard
									title="counts"
									value={{
										player_count: tick.player_count,
										local_count: tick.local_count
									}}
								/>
								<div>
									<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
										Power Items ({tick.power_items?.length ?? 0})
									</div>
									<JsonTree value={tick.power_items ?? []} depth={1} />
								</div>
							</div>
						</Tabs.Content>
					</Tabs>
				{/if}
			</Tabs.Content>

			<!-- EVENTS TAB -->
			<Tabs.Content value="events" class="pt-4">
				<div class="mb-3 flex items-center justify-between gap-2">
					<SegmentedControl
						value={eventFilter}
						onValueChange={(d) =>
							(eventFilter = d.value as
								| 'all'
								| 'match'
								| 'player'
								| 'combat'
								| 'pickup'
								| 'powerup')}
					>
						<SegmentedControl.Indicator />
						{#each ['all', 'match', 'player', 'combat', 'pickup', 'powerup'] as bucket}
							<SegmentedControl.Item value={bucket}>
								<SegmentedControl.ItemText>{bucket}</SegmentedControl.ItemText>
								<SegmentedControl.ItemHiddenInput />
							</SegmentedControl.Item>
						{/each}
					</SegmentedControl>
					<span class="text-surface-700-200 text-xs"
						>{filteredEvents.length} of {events.length} (newest first)</span
					>
				</div>
				{#if filteredEvents.length === 0}
					<div class="card preset-tonal text-surface-500-400 p-3 text-sm">
						no events {eventFilter === 'all' ? '' : `in '${eventFilter}'`} bucket
					</div>
				{:else}
					<JsonTree value={filteredEvents} label="Events" depth={1} />
				{/if}
			</Tabs.Content>

			<!-- PROBE TAB — diagnostic dump of every gametype/score address -->
			<Tabs.Content value="probe" class="space-y-4 pt-4">
				<div class="card preset-tonal p-3 text-sm">
					<p class="text-surface-700-200">
						Raw values from every candidate address the Halo: CE plugin reads for
						gametype detection, team scores, score limits, and per-player scores. Use
						this to identify which addresses match what the game actually shows so we
						can fix the canonical readers.
					</p>
				</div>
				{#if !scoreProbe}
					<div class="card preset-tonal text-surface-500-400 p-3 text-sm">
						No probe data yet — wait for the next inspect poll (~3s) once a scraper is
						attached.
					</div>
				{:else}
					<div class="space-y-4">
						{#each Object.entries(scoreProbe) as [section, value]}
							<div>
								<div class="text-surface-700-200 mb-1 text-xs font-semibold uppercase">
									{section}
								</div>
								<JsonTree {value} label={section} depth={1} />
							</div>
						{/each}
					</div>
				{/if}
			</Tabs.Content>

			<!-- RAW JSON TAB -->
			<Tabs.Content value="raw" class="space-y-4 pt-4">
				<div>
					<div class="text-surface-700-200 mb-1 text-xs">
						Snapshot · received {relativeTime(snapshotAt)}
					</div>
					{#if snapshot}
						<JsonTree value={snapshot} label="Snapshot" />
					{:else}
						<div class="card preset-tonal text-surface-500-400 p-3 text-sm">no snapshot yet</div>
					{/if}
				</div>
				<div>
					<div class="text-surface-700-200 mb-1 text-xs">
						Latest tick · received {relativeTime(tickAt)}
					</div>
					{#if tick}
						<JsonTree value={tick} label="Latest tick" defaultOpen={false} />
					{:else}
						<div class="card preset-tonal text-surface-500-400 p-3 text-sm">no tick yet</div>
					{/if}
				</div>
				<div>
					<div class="text-surface-700-200 mb-1 text-xs">
						Recent events · {events.length} buffered
					</div>
					{#if events.length > 0}
						<JsonTree value={events} label="Recent events" defaultOpen={false} />
					{:else}
						<div class="card preset-tonal text-surface-500-400 p-3 text-sm">no events yet</div>
					{/if}
				</div>
			</Tabs.Content>
		</Tabs>
	{/if}
</div>
