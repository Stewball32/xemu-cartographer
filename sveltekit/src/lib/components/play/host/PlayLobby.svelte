<script lang="ts">
	// Phase: lobby. The box is up and parked in the menus. The player picks the
	// map + gametype (read LIVE off the disc), readies up, and watches boxes /
	// players join over System Link. The host-runner drives the actual menu
	// navigation + presses start once the native preconditions pass (2+ boxes,
	// 2+ teams) — the player never touches noVNC here.
	import {
		formatCountdown,
		isPlayerControllable,
		type OptionsResponse,
		type PlayStatus,
		type ReapView
	} from '$lib/utils/play-hosting';
	import type { GamePayload } from '$lib/types/scraper-v2';
	import { buildScoreboard } from '$lib/utils/overlay-view';
	import {
		AlertTriangleIcon,
		CheckCircle2Icon,
		LoaderIcon,
		LockIcon,
		MonitorIcon,
		PowerIcon,
		UsersIcon
	} from '@lucide/svelte';

	interface Props {
		instance: string;
		status: PlayStatus | null;
		options: OptionsResponse | null;
		game: GamePayload | null;
		reap: ReapView | null;
		busy: boolean;
		onselect: (map: string, gametype: string) => void;
		onready: (ready: boolean) => void;
		onteardown: () => void;
	}

	let { instance, status, options, game, reap, busy, onselect, onready, onteardown }: Props =
		$props();

	// Seed the pickers once from the confirmed selection; the parent re-keys this
	// component on `instance` so a new box resets the local state.
	let seeded = $state(false);
	let pickMap = $state('');
	let pickGametype = $state('');
	$effect(() => {
		if (!seeded && (status || options)) {
			pickMap = status?.selected_map ?? options?.selected_map ?? '';
			pickGametype = status?.selected_gametype ?? options?.selected_gametype ?? '';
			seeded = true;
		}
	});

	const controllable = $derived(isPlayerControllable(status));
	const enumerable = $derived(options?.available === true);
	// The gametype list is only "ready" once the async host-side custom-variant
	// read has finished — until then it's the built-ins-only intermediate, which we
	// hide behind a "reading gametypes…" waiting state (the page re-polls options
	// live, so this clears on its own without a refresh).
	const gametypesReady = $derived(enumerable && options?.gametypes_pending !== true);

	// CE gametype display names are NOT unique (a modded disc can list "Practice
	// Mode" 3×). The /selection API resolves a pick by NAME (first match), so the
	// duplicate rows aren't independently selectable — dedupe by name for a clean
	// picker, keeping the first (lowest-steps) occurrence the backend resolves to.
	// Maps have unique names, so they're used as-is.
	const gametypeOptions = $derived.by(() => {
		const seen = new Set<string>();
		return (options?.gametypes ?? []).filter((g) => {
			if (seen.has(g.name)) return false;
			seen.add(g.name);
			return true;
		});
	});
	const selected = $derived(status?.selected === true);
	const ready = $derived(status?.ready === true);

	// Roster view off the live game feed (scores muted in the lobby).
	const roster = $derived(buildScoreboard(game, null));
	const machines = $derived(game?.machines ?? []);

	const selectionChanged = $derived(
		pickMap !== (status?.selected_map ?? '') || pickGametype !== (status?.selected_gametype ?? '')
	);
	const canSave = $derived(
		controllable && !busy && pickMap.trim().length > 0 && pickGametype.trim().length > 0
	);

	function save() {
		if (!canSave) return;
		onselect(pickMap.trim(), pickGametype.trim());
	}

	// Native-start readiness (the runner needs a 2nd box + 2 teams before it can
	// press start). Surfaced so the host knows what they're waiting on.
	const needBox = $derived((status?.machine_count ?? 0) < 2);
	const needTeams = $derived((status?.team_count ?? 0) < 2);
</script>

<div class="flex flex-col gap-4">
	<!-- Box header -->
	<div class="flex flex-wrap items-center gap-2">
		<MonitorIcon class="size-5" />
		<span class="font-semibold">Your box</span>
		<code class="text-xs text-surface-600-400">{instance}</code>
		{#if status}
			{#if status.authority === 'runner'}
				<span class="preset-tonal-success-500 ml-auto badge text-xs">Hosting</span>
			{:else if status.authority === 'admin'}
				<span class="preset-tonal-warning-500 ml-auto badge text-xs">Admin control</span>
			{:else}
				<span class="preset-tonal-error-500 ml-auto badge text-xs">Hosting off</span>
			{/if}
		{/if}
	</div>

	<!-- Idle-out heads-up -->
	{#if reap}
		<div
			class="card p-3 text-sm {reap.warning ? 'preset-tonal-error' : 'preset-tonal-warning'}"
			role="status"
		>
			<div class="flex items-center gap-2">
				<AlertTriangleIcon class="size-4 shrink-0" />
				<span>
					This box is idle and will be released in <b>{formatCountdown(reap.seconds_remaining)}</b>.
					Start a game to keep it alive.
				</span>
			</div>
		</div>
	{/if}

	<!-- Admin / disabled lockout -->
	{#if status && !controllable}
		<div class="flex items-center gap-2 card preset-tonal p-3 text-sm">
			<LockIcon class="size-4 shrink-0" />
			<span>
				{#if status.authority === 'admin'}
					An admin is currently controlling this box. Your controls are paused.
				{:else}
					Auto-hosting is disabled on this box.
				{/if}
			</span>
		</div>
	{/if}

	<!-- Map / gametype picker -->
	<div class="flex flex-col gap-3 card preset-tonal p-4">
		<h3 class="text-sm font-semibold tracking-wide uppercase">Game setup</h3>

		<div class="grid gap-3 sm:grid-cols-2">
			<!-- Dropdowns ONLY — populated from the live enumeration. There is no
			     type-in fallback: a free-text name the runner can't match to a live
			     carousel card never drives selection, so when the list isn't readable
			     we show a disabled, clearly-labelled empty state instead of a dead
			     text box. -->
			<label class="label">
				<span class="label-text text-xs">Map</span>
				<select class="select" bind:value={pickMap} disabled={!enumerable || !controllable || busy}>
					{#if enumerable}
						<option value="" disabled>Choose a map…</option>
						{#each options?.maps ?? [] as m (m.steps)}
							<option value={m.name}>{m.name}</option>
						{/each}
					{:else}
						<option value="" disabled>Map list not readable yet</option>
					{/if}
				</select>
			</label>

			<label class="label">
				<span class="label-text text-xs">Gametype</span>
				<!-- gametypeOptions is deduped by name (see script); key by the unique
				     carousel index (steps) so a keyed-each duplicate key can never
				     corrupt this dropdown the way raw duplicate names did (the map
				     list has unique names, so it rendered fine). -->
				<select class="select" bind:value={pickGametype} disabled={!gametypesReady || !controllable || busy}>
					{#if gametypesReady}
						<option value="" disabled>Choose a gametype…</option>
						{#each gametypeOptions as g (g.steps)}
							<option value={g.name}>{g.name}</option>
						{/each}
					{:else if enumerable}
						<option value="" disabled>Reading gametypes…</option>
					{:else}
						<option value="" disabled>Gametype list not readable yet</option>
					{/if}
				</select>
			</label>
		</div>

		{#if !enumerable}
			<p class="text-xs text-surface-500">
				This box's map / gametype lists aren't readable yet (the create-game
				carousels load once the box reaches the front-end). The pickers enable
				automatically as soon as they're readable.
			</p>
		{/if}

		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				class="btn preset-filled-primary-500 btn-sm"
				disabled={!canSave || !selectionChanged}
				onclick={save}
			>
				{#if busy}<LoaderIcon class="size-4 animate-spin" />{/if}
				{selected ? 'Update setup' : 'Set game'}
			</button>
			{#if selected && status?.selected_map}
				<span class="text-xs text-surface-600-400">
					Selected: <b>{status.selected_map}</b> · <b>{status.selected_gametype}</b>
				</span>
			{/if}
		</div>

		<!-- Live readback: what the SCRAPER reads as the box's currently-loaded map /
		     gametype (distinct from the picked intent above) — the double-check that
		     the runner's selection actually took on the box. Blank until the box
		     settles in the lobby with a map/gametype loaded. -->
		{#if status?.map || status?.gametype}
			<span class="text-xs text-surface-500">
				On the box now: <b>{status.map || '—'}</b> · <b>{status.gametype || '—'}</b>
			</span>
		{/if}
	</div>

	<!-- Ready + start readiness -->
	<div class="flex flex-col gap-3 card preset-tonal p-4">
		<div class="flex flex-wrap items-center gap-3">
			<h3 class="text-sm font-semibold tracking-wide uppercase">Start</h3>
			{#if ready}
				<button
					type="button"
					class="ml-auto btn preset-tonal-error btn-sm"
					disabled={!controllable || busy}
					onclick={() => onready(false)}
				>
					Cancel ready
				</button>
			{:else}
				<button
					type="button"
					class="ml-auto btn preset-filled-success-500 btn-sm"
					disabled={!controllable || busy || !selected}
					onclick={() => onready(true)}
				>
					{#if busy}<LoaderIcon class="size-4 animate-spin" />{/if}
					Ready up
				</button>
			{/if}
		</div>

		{#if !selected}
			<p class="text-sm text-surface-600-400">Pick a map and gametype above first.</p>
		{:else if status?.countdown_active}
			<p class="flex items-center gap-2 text-sm">
				<LoaderIcon class="size-4 animate-spin" /> Match starting…
			</p>
		{:else if ready}
			<div class="flex flex-col gap-1 text-sm">
				<p class="flex items-center gap-2 text-success-600-400">
					<CheckCircle2Icon class="size-4" /> You're ready — the match starts automatically when:
				</p>
				<ul class="ml-6 list-disc text-xs text-surface-600-400">
					<li class:line-through={!needBox}>a second box joins over System Link</li>
					<li class:line-through={!needTeams}>players are split across at least two teams</li>
				</ul>
			</div>
		{:else}
			<p class="text-sm text-surface-600-400">
				Press <b>Ready up</b> when your friends are ready to go.
			</p>
		{/if}
	</div>

	<!-- Who's here -->
	<div class="flex flex-col gap-3 card preset-tonal p-4">
		<div class="flex items-center gap-2">
			<UsersIcon class="size-4" />
			<h3 class="text-sm font-semibold tracking-wide uppercase">In the lobby</h3>
		</div>

		<div class="flex flex-col gap-1">
			<span class="text-xs text-surface-500">Boxes ({machines.length || 1})</span>
			<div class="flex flex-wrap gap-1.5">
				{#if machines.length > 0}
					{#each machines as m (m.index)}
						<span class="chip preset-tonal text-xs">
							{m.name || `machine ${m.index}`}{m.is_local ? ' (you)' : ''}
						</span>
					{/each}
				{:else}
					<span class="chip preset-tonal text-xs">{instance} (you)</span>
				{/if}
			</div>
			{#if machines.length < 2}
				<span class="text-xs text-surface-500">Waiting for another box to join…</span>
			{/if}
		</div>

		{#if roster.playerCount > 0}
			<div class="flex flex-col gap-1">
				<span class="text-xs text-surface-500">Players ({roster.playerCount})</span>
				<div class="flex flex-wrap gap-1.5">
					{#each roster.isTeamGame ? roster.teams.flatMap((t) => t.players) : roster.players as p (p.index)}
						<span class="chip preset-tonal text-xs" title={p.name}>{p.name}</span>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- Teardown -->
	<div class="flex flex-col gap-1">
		<button
			type="button"
			class="btn self-start preset-tonal-error btn-sm"
			disabled={busy}
			onclick={onteardown}
		>
			<PowerIcon class="size-4" /> End session
		</button>
		<p class="text-xs text-surface-500">
			Resets the lobby so the next player can pick a game. The box is fully released after it sits
			idle.
		</p>
	</div>
</div>
