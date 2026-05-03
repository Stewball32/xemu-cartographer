<script lang="ts">
	import type {
		GameState,
		SnapshotPayload,
		SnapshotPlayer,
		StateInputs,
		TickPayload,
		TickPlayer
	} from '$lib/types/scraper';
	import KvCard from './KvCard.svelte';

	let {
		state,
		snapshot,
		tick,
		tickValue,
		stateInputs,
		showAll
	}: {
		state: GameState | '';
		snapshot: SnapshotPayload | null;
		tick: TickPayload | null;
		tickValue: number | undefined;
		stateInputs: StateInputs | null;
		showAll: boolean;
	} = $props();

	const stateBadgeClass: Record<string, string> = {
		menu: 'preset-tonal',
		lobby: 'preset-filled-secondary-500',
		pregame: 'preset-filled-warning-500',
		in_game: 'preset-filled-success-500',
		postgame: 'preset-filled-tertiary-500',
		'': 'preset-tonal'
	};

	const isTeam = $derived(snapshot?.is_team_game === true);
	const players = $derived(snapshot?.players ?? []);
	const teamScores = $derived(snapshot?.team_scores ?? []);

	const tickByIdx = $derived.by(() => {
		const map = new Map<number, TickPlayer>();
		for (const p of tick?.players ?? []) map.set(p.index, p);
		return map;
	});

	// Sort players by score desc for the FFA leaderboard view. Score now
	// comes from the gametype-specific table (Slayer/Oddball/King/Race) or
	// CTF flag captures, so it's a meaningful ranking across all modes.
	const sortedPlayers = $derived(
		[...players].sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
	);

	// Group players by team for the team-game roster view.
	const playersByTeam = $derived.by(() => {
		const map = new Map<number, SnapshotPlayer[]>();
		for (const p of players) {
			const arr = map.get(p.team) ?? [];
			arr.push(p);
			map.set(p.team, arr);
		}
		return [...map.entries()].sort(([a], [b]) => a - b);
	});

	function teamLabel(team: number): string {
		// Halo CE conventional team mapping: 0=red, 1=blue, others fall back to index.
		if (team === 0) return 'Red';
		if (team === 1) return 'Blue';
		return `Team ${team}`;
	}

	function teamScoreFor(team: number): number {
		return teamScores.find((t) => t.team === team)?.score ?? 0;
	}

	function fmtTickAsTime(ticks: number | undefined): string {
		// Halo CE engine tick rate is 30Hz. The tick is engine-uptime since boot
		// (not match-relative), so this displays "engine time" — useful as a
		// rough sanity check that the game is advancing.
		if (ticks === undefined) return '—';
		const totalSeconds = Math.floor(ticks / 30);
		const h = Math.floor(totalSeconds / 3600);
		const m = Math.floor((totalSeconds % 3600) / 60);
		const s = totalSeconds % 60;
		const pad = (n: number) => n.toString().padStart(2, '0');
		return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
	}

	function fmtLimit(n: number, unit: string = ''): string {
		if (!n || n <= 0) return 'none';
		return unit ? `${n} ${unit}` : String(n);
	}

	function fmtPct(v: number | undefined | null): string {
		if (v === undefined || v === null) return '—';
		return `${Math.round(Math.max(0, Math.min(100, v * 100)))}%`;
	}

	function localBadge(p: SnapshotPlayer): string | null {
		if (!p.is_local) return null;
		return p.local_index !== null && p.local_index !== undefined ? `L${p.local_index}` : 'L';
	}
</script>

<div class="space-y-3">
	<!-- Match config row -->
	<div class="card preset-tonal p-4">
		<div class="mb-3 flex flex-wrap items-center gap-2">
			<span class="badge {stateBadgeClass[state] ?? 'preset-tonal'} text-xs uppercase">
				{state || 'unknown'}
			</span>
			{#if snapshot}
				<span class="badge preset-tonal text-xs">{snapshot.gametype}</span>
				{#if isTeam}
					<span class="badge preset-filled-primary-500 text-xs">Team</span>
				{:else}
					<span class="badge preset-tonal text-xs">FFA</span>
				{/if}
			{/if}
			{#if tickValue !== undefined}
				<span class="ml-auto flex items-baseline gap-1.5">
					<span class="text-surface-700-200 text-xs uppercase">tick</span>
					<span class="font-mono text-lg tabular-nums">{tickValue}</span>
					<span class="text-surface-700-200 font-mono text-xs tabular-nums"
						>({fmtTickAsTime(tickValue)})</span
					>
				</span>
			{/if}
		</div>
		{#if snapshot}
			<dl class="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm sm:grid-cols-[max-content_1fr_max-content_1fr]">
				<dt class="text-surface-700-200 font-mono text-xs">map</dt>
				<dd class="font-mono">{snapshot.map || '—'}</dd>
				<dt class="text-surface-700-200 font-mono text-xs">difficulty</dt>
				<dd class="font-mono tabular-nums">{snapshot.game_difficulty}</dd>
				<dt class="text-surface-700-200 font-mono text-xs">score limit</dt>
				<dd class="font-mono tabular-nums">{fmtLimit(snapshot.score_limit)}</dd>
				<dt class="text-surface-700-200 font-mono text-xs">time limit</dt>
				<dd class="font-mono tabular-nums">{fmtLimit(snapshot.time_limit_ticks, 'ticks')}</dd>
			</dl>
		{:else}
			<div class="text-surface-500-400 text-sm">No snapshot yet — waiting for first read.</div>
		{/if}
	</div>

	<!-- Score block: full-width when only one variant is shown, 2-col when both -->
	{#if snapshot}
		{@const showTeam = isTeam || showAll}
		{@const showFFA = !isTeam || showAll}
		{@const bothShown = showTeam && showFFA}
		<div class="grid gap-3 {bothShown ? 'lg:grid-cols-2' : 'grid-cols-1'}">
			{#if showTeam}
				<div class="card preset-tonal p-4">
					<div
						class="text-surface-700-200 mb-2 flex items-center gap-2 text-xs font-semibold uppercase"
					>
						Team Score
						{#if !isTeam}
							<span class="text-surface-500-400 normal-case font-normal"
								>(unused — FFA match)</span
							>
						{/if}
					</div>
					{#if teamScores.length > 0}
						<div class="grid grid-cols-2 gap-3">
							{#each teamScores as ts}
								{@const isRed = ts.team === 0}
								<div
									class="rounded p-3 text-center {isRed
										? 'bg-error-500/20'
										: 'bg-primary-500/20'}"
								>
									<div class="text-xs uppercase">{teamLabel(ts.team)}</div>
									<div class="text-3xl font-bold tabular-nums">{ts.score}</div>
								</div>
							{/each}
						</div>
					{:else}
						<div class="text-surface-500-400 text-sm">no team scores</div>
					{/if}
				</div>
			{/if}

			{#if showFFA}
				<div class="card preset-tonal p-4">
					<div
						class="text-surface-700-200 mb-2 flex items-center gap-2 text-xs font-semibold uppercase"
					>
						FFA Leaderboard
						{#if isTeam}
							<span class="text-surface-500-400 normal-case font-normal"
								>(unused — team match)</span
							>
						{/if}
					</div>
					{#if sortedPlayers.length > 0}
						<table class="w-full text-sm">
							<thead class="text-surface-700-200 text-xs">
								<tr>
									<th class="px-1 py-1 text-left">#</th>
									<th class="px-1 py-1 text-left">name</th>
									<th class="px-1 py-1 text-right">Score</th>
									<th class="px-1 py-1 text-right">K</th>
									<th class="px-1 py-1 text-right">D</th>
									<th class="px-1 py-1 text-right">A</th>
								</tr>
							</thead>
							<tbody>
								{#each sortedPlayers as p, i}
									<tr class="border-surface-300-700 border-t">
										<td class="px-1 py-1 font-mono text-xs">{i + 1}</td>
										<td class="px-1 py-1">{p.name || '—'}</td>
										<td class="px-1 py-1 text-right font-mono tabular-nums">{p.score}</td>
										<td class="px-1 py-1 text-right font-mono tabular-nums">{p.kills}</td>
										<td class="px-1 py-1 text-right font-mono tabular-nums">{p.deaths}</td>
										<td class="px-1 py-1 text-right font-mono tabular-nums">{p.assists}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					{:else}
						<div class="text-surface-500-400 text-sm">no players</div>
					{/if}
				</div>
			{/if}
		</div>
	{/if}

	<!-- Roster — single table so all per-player columns line up vertically.
	     In team games we insert team-divider rows between team groups. -->
	{#if snapshot && players.length > 0}
		{@const colCount = 9 + (showAll && !isTeam ? 1 : 0)}
		<div class="card preset-tonal p-4">
			<div class="text-surface-700-200 mb-2 text-xs font-semibold uppercase">
				Roster ({players.length})
			</div>
			<table class="w-full table-fixed text-sm">
				<colgroup>
					<col class="w-4" />
					<col />
					{#if showAll && !isTeam}<col class="w-12" />{/if}
					<col class="w-12" />
					<col class="w-10" />
					<col class="w-10" />
					<col class="w-10" />
					<col class="w-10" />
					<col class="w-14" />
					<col class="w-14" />
				</colgroup>
				<thead class="text-surface-700-200 text-xs">
					<tr>
						<th class="px-1 py-1"></th>
						<th class="px-1 py-1 text-left">name</th>
						{#if showAll && !isTeam}
							<th class="px-1 py-1 text-left">team</th>
						{/if}
						<th class="px-1 py-1 text-right">Score</th>
						<th class="px-1 py-1 text-right">K</th>
						<th class="px-1 py-1 text-right">D</th>
						<th class="px-1 py-1 text-right">A</th>
						<th class="px-1 py-1 text-right">KS</th>
						<th class="px-1 py-1 text-right">HP</th>
						<th class="px-1 py-1 text-right">Sh</th>
					</tr>
				</thead>
				<tbody>
					{#if isTeam}
						{#each playersByTeam as [team, members]}
							<tr class="bg-surface-200-800">
								<td class="px-1 py-1">
									<span
										class="block size-3 rounded-sm {team === 0
											? 'bg-error-500'
											: 'bg-primary-500'}"
									></span>
								</td>
								<td colspan={colCount - 2} class="px-1 py-1 text-xs font-semibold">
									{teamLabel(team)}
									<span class="text-surface-700-200 ml-2 font-normal tabular-nums"
										>{teamScoreFor(team)} pts</span
									>
								</td>
								<td class="px-1 py-1"></td>
							</tr>
							{#each members as p}
								{@const t = tickByIdx.get(p.index)}
								<tr class="border-surface-300-700 border-t">
									<td class="px-1 py-1">
										{#if t?.alive === true}
											<span class="bg-success-500 inline-block size-2 rounded-full" title="alive"
											></span>
										{:else if t?.alive === false}
											<span class="bg-error-500 inline-block size-2 rounded-full" title="dead"
											></span>
										{:else}
											<span
												class="bg-surface-500 inline-block size-2 rounded-full"
												title="unknown"
											></span>
										{/if}
									</td>
									<td class="truncate px-1 py-1">
										{p.name || '—'}
										{#if localBadge(p)}
											<span
												class="badge preset-tonal-warning ml-1 text-[10px]"
												title="local player"
											>
												{localBadge(p)}
											</span>
										{/if}
									</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{p.score}</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{p.kills}</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{p.deaths}</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{p.assists}</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{p.kill_streak}</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{fmtPct(t?.health)}</td>
									<td class="px-1 py-1 text-right font-mono tabular-nums">{fmtPct(t?.shields)}</td>
								</tr>
							{/each}
						{/each}
					{:else}
						{#each players as p}
							{@const t = tickByIdx.get(p.index)}
							<tr class="border-surface-300-700 border-t">
								<td class="px-1 py-1">
									{#if t?.alive === true}
										<span class="bg-success-500 inline-block size-2 rounded-full" title="alive"
										></span>
									{:else if t?.alive === false}
										<span class="bg-error-500 inline-block size-2 rounded-full" title="dead"
										></span>
									{:else}
										<span
											class="bg-surface-500 inline-block size-2 rounded-full"
											title="unknown"
										></span>
									{/if}
								</td>
								<td class="truncate px-1 py-1">
									{p.name || '—'}
									{#if localBadge(p)}
										<span
											class="badge preset-tonal-warning ml-1 text-[10px]"
											title="local player"
										>
											{localBadge(p)}
										</span>
									{/if}
								</td>
								{#if showAll}
									<td class="px-1 py-1 font-mono text-xs">{p.team}</td>
								{/if}
								<td class="px-1 py-1 text-right font-mono tabular-nums">{p.score}</td>
								<td class="px-1 py-1 text-right font-mono tabular-nums">{p.kills}</td>
								<td class="px-1 py-1 text-right font-mono tabular-nums">{p.deaths}</td>
								<td class="px-1 py-1 text-right font-mono tabular-nums">{p.assists}</td>
								<td class="px-1 py-1 text-right font-mono tabular-nums">{p.kill_streak}</td>
								<td class="px-1 py-1 text-right font-mono tabular-nums">{fmtPct(t?.health)}</td>
								<td class="px-1 py-1 text-right font-mono tabular-nums">{fmtPct(t?.shields)}</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	{/if}

	<!-- State inputs diagnostic — always visible -->
	<KvCard
		title="State inputs (diagnostic)"
		value={stateInputs}
		emptyMessage="no state inputs reported by plugin"
	/>
</div>
