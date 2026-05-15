<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import {
		ActivityIcon,
		AlertTriangleIcon,
		ChevronDownIcon,
		ClockIcon,
		DatabaseIcon,
		DiscIcon,
		FileTextIcon,
		Gamepad2Icon,
		GlobeIcon,
		HardDriveIcon,
		HashIcon,
		ListIcon,
		MapIcon,
		MonitorIcon,
		PlayIcon,
		PowerIcon,
		RepeatIcon,
		ServerIcon,
		SwordsIcon,
		TagIcon,
		TimerIcon,
		TrophyIcon,
		WifiIcon,
		ZapIcon
	} from '@lucide/svelte';
	import { scraperWS } from '$lib/stores/scraper-ws.svelte';
	import { useDebugContext } from '../context.js';
	import StatTile, { type StatRow } from '../shared/StatTile.svelte';
	import { buildXboxVm } from './xbox-vm';
	import { buildRuntimeVm } from './runtime-vm';

	let { name }: { name: string } = $props();

	const ctx = useDebugContext();
	const xboxVm = $derived.by(() => buildXboxVm(name, scraperWS, ctx.inspect));
	const runtimeVm = $derived.by(() => buildRuntimeVm(name, scraperWS, ctx));

	type SectionId =
		| 'identity'
		| 'xbe_cert'
		| 'eeprom'
		| 'kernel_clock'
		| 'connection'
		| 'lifecycle'
		| 'summary'
		| 'freshness';

	let collapsedSections = $state(new SvelteSet<string>());

	function loadCollapsed() {
		try {
			const raw = localStorage.getItem('debug.system.collapsed');
			if (raw) collapsedSections = new SvelteSet<string>(JSON.parse(raw));
		} catch {
			// localStorage unavailable; default to all open.
		}
	}

	function saveCollapsed() {
		try {
			localStorage.setItem('debug.system.collapsed', JSON.stringify([...collapsedSections]));
		} catch {
			// ignore
		}
	}

	const visibleSections = $derived.by<SectionId[]>(() => {
		const visible: SectionId[] = ['identity'];
		if (xboxVm.xbeCert) visible.push('xbe_cert');
		if (xboxVm.eeprom) visible.push('eeprom');
		if (xboxVm.kernelClock) visible.push('kernel_clock');
		visible.push('connection', 'lifecycle');
		if (runtimeVm.summary) visible.push('summary');
		visible.push('freshness');
		return visible;
	});

	const openSections = $derived(visibleSections.filter((id) => !collapsedSections.has(id)));

	function onAccordionChange(next: string[]) {
		const open = new Set(next);
		const newCollapsed = new SvelteSet<string>();
		for (const id of visibleSections) {
			if (!open.has(id)) newCollapsed.add(id);
		}
		collapsedSections = newCollapsed;
		saveCollapsed();
	}

	onMount(loadCollapsed);

	type StatusKind = 'on' | 'off' | 'warn' | 'info' | 'neutral' | 'pointer' | 'none';

	function asStr(v: unknown): string {
		if (v === null || v === undefined) return '—';
		const s = String(v);
		return s.length > 0 ? s : '—';
	}
	function present(v: unknown): boolean {
		return asStr(v) !== '—';
	}
	function kindFor(v: unknown): StatusKind {
		return present(v) ? 'on' : 'none';
	}
	function phaseKind(phase: string | undefined): StatusKind {
		if (phase === 'live') return 'on';
		if (phase === 'ready') return 'warn';
		return 'neutral';
	}

	// Grouped sub-rows for tiles that read naturally as one labelled card with
	// multiple related values — time zone (bias/std/dlt) and last-read pairs
	// (timestamp + age).
	const tzRows = $derived.by<StatRow[]>(() => {
		const bias = asStr(xboxVm.eeprom?.time_zone);
		const std = asStr(xboxVm.eeprom?.time_zone_std_name);
		const dlt = asStr(xboxVm.eeprom?.time_zone_dlt_name);
		return [
			{ label: 'bias', display: bias, statusKind: bias !== '—' ? 'on' : 'none' },
			{ label: 'std', display: std, statusKind: std !== '—' ? 'on' : 'neutral' },
			{ label: 'dlt', display: dlt, statusKind: dlt !== '—' ? 'on' : 'neutral' }
		];
	});

	const lastReadRows = $derived.by<StatRow[]>(() => [
		{
			label: 'at',
			display: asStr(runtimeVm.lifecycle?.last_read_at),
			statusKind: kindFor(runtimeVm.lifecycle?.last_read_at)
		},
		{
			label: 'age',
			display: runtimeVm.lastReadStr,
			statusKind: runtimeVm.lastReadStr !== 'never' ? 'on' : 'none'
		}
	]);

	const lastSummaryReadRows = $derived.by<StatRow[]>(() =>
		runtimeVm.summary
			? [
					{
						label: 'at',
						display: asStr(runtimeVm.summary.last_successful_read_at),
						statusKind: kindFor(runtimeVm.summary.last_successful_read_at)
					},
					{
						label: 'age',
						display: runtimeVm.hostSummaryReadStr,
						statusKind: runtimeVm.hostSummaryReadStr !== 'never' ? 'on' : 'none'
					}
				]
			: []
	);
</script>

{#snippet trigger(title: string, subtitle?: string)}
	<span
		class="text-surface-700-200 inline-flex items-baseline gap-2 text-xs font-semibold tracking-wide uppercase"
	>
		{title}
		{#if subtitle}
			<span class="text-surface-500-400 font-normal normal-case">{subtitle}</span>
		{/if}
	</span>
{/snippet}

{#snippet titleIcon()}<Gamepad2Icon class="size-3.5" />{/snippet}
{#snippet hashIcon()}<HashIcon class="size-3.5" />{/snippet}
{#snippet fileIcon()}<FileTextIcon class="size-3.5" />{/snippet}
{#snippet serverIcon()}<ServerIcon class="size-3.5" />{/snippet}
{#snippet clockIcon()}<ClockIcon class="size-3.5" />{/snippet}
{#snippet globeIcon()}<GlobeIcon class="size-3.5" />{/snippet}
{#snippet discIcon()}<DiscIcon class="size-3.5" />{/snippet}
{#snippet hardDriveIcon()}<HardDriveIcon class="size-3.5" />{/snippet}
{#snippet wifiIcon()}<WifiIcon class="size-3.5" />{/snippet}
{#snippet monitorIcon()}<MonitorIcon class="size-3.5" />{/snippet}
{#snippet powerIcon()}<PowerIcon class="size-3.5" />{/snippet}
{#snippet timerIcon()}<TimerIcon class="size-3.5" />{/snippet}
{#snippet activityIcon()}<ActivityIcon class="size-3.5" />{/snippet}
{#snippet alertIcon()}<AlertTriangleIcon class="size-3.5" />{/snippet}
{#snippet playIcon()}<PlayIcon class="size-3.5" />{/snippet}
{#snippet repeatIcon()}<RepeatIcon class="size-3.5" />{/snippet}
{#snippet mapIcon()}<MapIcon class="size-3.5" />{/snippet}
{#snippet swordsIcon()}<SwordsIcon class="size-3.5" />{/snippet}
{#snippet trophyIcon()}<TrophyIcon class="size-3.5" />{/snippet}
{#snippet tagIcon()}<TagIcon class="size-3.5" />{/snippet}
{#snippet databaseIcon()}<DatabaseIcon class="size-3.5" />{/snippet}
{#snippet zapIcon()}<ZapIcon class="size-3.5" />{/snippet}
{#snippet listIcon()}<ListIcon class="size-3.5" />{/snippet}

<Accordion value={openSections} onValueChange={(e) => onAccordionChange(e.value)} multiple>
	<Accordion.Item value="identity">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Identity', 'title / xbe / console name')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{#if !xboxVm.identity}
				<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
					no current_state envelope yet — waiting for first read
				</div>
			{:else}
				{@const id = xboxVm.identity}
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
					<StatTile
						label="title"
						display={asStr(id.title)}
						statusKind={kindFor(id.title)}
						title={asStr(id.title)}
						icon={titleIcon}
					/>
					<StatTile
						label="title_id"
						display={asStr(id.title_id)}
						statusKind={kindFor(id.title_id)}
						title={asStr(id.title_id)}
						icon={hashIcon}
					/>
					<StatTile
						label="xbe_title_name"
						display={asStr(id.xbe_title_name)}
						statusKind={kindFor(id.xbe_title_name)}
						title={asStr(id.xbe_title_name)}
						icon={fileIcon}
					/>
					<StatTile
						label="xbox_name"
						display={asStr(id.xbox_name)}
						statusKind={kindFor(id.xbox_name)}
						title={asStr(id.xbox_name)}
						icon={serverIcon}
					/>
					<StatTile
						label="started_at"
						display={asStr(id.started_at)}
						statusKind={kindFor(id.started_at)}
						title={asStr(id.started_at)}
						icon={clockIcon}
					/>
				</div>
			{/if}
		</Accordion.ItemContent>
	</Accordion.Item>

	{#if xboxVm.xbeCert}
		{@const cert = xboxVm.xbeCert}
		<Accordion.Item value="xbe_cert">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('XBE certificate', 'on-disk image metadata')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
					<StatTile
						label="xbe_version"
						display={asStr(cert.xbe_version)}
						statusKind={kindFor(cert.xbe_version)}
						title={asStr(cert.xbe_version)}
						icon={hashIcon}
					/>
					<StatTile
						label="xbe_game_region"
						display={asStr(cert.xbe_game_region)}
						statusKind={kindFor(cert.xbe_game_region)}
						title={asStr(cert.xbe_game_region)}
						icon={globeIcon}
					/>
					<StatTile
						label="xbe_disk_number"
						display={asStr(cert.xbe_disk_number)}
						statusKind={kindFor(cert.xbe_disk_number)}
						title={asStr(cert.xbe_disk_number)}
						icon={discIcon}
					/>
					<StatTile
						label="xbe_allowed_media"
						display={asStr(cert.xbe_allowed_media)}
						statusKind={kindFor(cert.xbe_allowed_media)}
						title={asStr(cert.xbe_allowed_media)}
						icon={hardDriveIcon}
					/>
				</div>
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	{#if xboxVm.eeprom}
		{@const eep = xboxVm.eeprom}
		<Accordion.Item value="eeprom">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('EEPROM', 'serial / mac / region / time zone')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
					<StatTile
						label="serial_number"
						display={asStr(eep.serial_number)}
						statusKind={kindFor(eep.serial_number)}
						title={asStr(eep.serial_number)}
						icon={hashIcon}
					/>
					<StatTile
						label="mac_address"
						display={asStr(eep.mac_address)}
						statusKind={kindFor(eep.mac_address)}
						title={asStr(eep.mac_address)}
						icon={wifiIcon}
					/>
					<StatTile
						label="video_standard"
						display={asStr(eep.video_standard)}
						statusKind={kindFor(eep.video_standard)}
						title={asStr(eep.video_standard)}
						icon={monitorIcon}
					/>
					<StatTile label="time zone" rows={tzRows} icon={globeIcon} />
				</div>
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	{#if xboxVm.kernelClock}
		{@const kc = xboxVm.kernelClock}
		<Accordion.Item value="kernel_clock">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Kernel clock', 'KeSystemTime / KeInterruptTime')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<div class="grid grid-cols-1 gap-2 lg:grid-cols-3">
					<StatTile
						label="kernel_system_time"
						display={asStr(kc.kernel_system_time)}
						statusKind={kindFor(kc.kernel_system_time)}
						title={asStr(kc.kernel_system_time)}
						icon={clockIcon}
					/>
					<StatTile
						label="kernel_boot_time"
						display={asStr(kc.kernel_boot_time)}
						statusKind={kindFor(kc.kernel_boot_time)}
						title={asStr(kc.kernel_boot_time)}
						icon={powerIcon}
					/>
					<StatTile
						label="kernel_uptime"
						display={asStr(kc.kernel_uptime)}
						statusKind={kindFor(kc.kernel_uptime)}
						title={asStr(kc.kernel_uptime)}
						icon={timerIcon}
					/>
				</div>
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	<Accordion.Item value="connection">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Connection', 'WebSocket transport')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				<StatTile
					label="ws_connected"
					display={runtimeVm.wsConnected ? 'connected' : 'disconnected'}
					statusKind={runtimeVm.wsConnected ? 'on' : 'off'}
					icon={wifiIcon}
				/>
				<StatTile
					label="last_error"
					display={runtimeVm.lastError ?? '—'}
					statusKind={runtimeVm.lastError ? 'off' : 'none'}
					title={runtimeVm.lastError ?? undefined}
					icon={alertIcon}
				/>
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>

	<Accordion.Item value="lifecycle">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Lifecycle', 'phase / uptime / counters')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			{#if !runtimeVm.lifecycle}
				<div class="text-surface-500-400 card preset-tonal p-3 text-sm">
					no current_state envelope yet
				</div>
			{:else}
				{@const life = runtimeVm.lifecycle}
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
					<StatTile
						label="phase"
						display={asStr(life.phase)}
						statusKind={phaseKind(asStr(life.phase))}
						icon={activityIcon}
					/>
					<StatTile
						label="started_at"
						display={asStr(life.started_at)}
						statusKind={kindFor(life.started_at)}
						title={asStr(life.started_at)}
						icon={playIcon}
					/>
					<StatTile
						label="runner_uptime"
						display={runtimeVm.runnerUptimeStr}
						statusKind={runtimeVm.runnerUptimeStr !== '—' ? 'on' : 'none'}
						icon={timerIcon}
					/>
					<StatTile label="last_read" rows={lastReadRows} icon={clockIcon} />
					<StatTile
						label="engine_tick"
						display={asStr(life.engine_tick)}
						statusKind={kindFor(life.engine_tick)}
						icon={zapIcon}
					/>
					<StatTile
						label="iterations"
						display={asStr(life.iterations)}
						statusKind={kindFor(life.iterations)}
						icon={repeatIcon}
					/>
				</div>
			{/if}
		</Accordion.ItemContent>
	</Accordion.Item>

	{#if runtimeVm.summary}
		{@const sum = runtimeVm.summary}
		<Accordion.Item value="summary">
			<Accordion.ItemTrigger
				class="group flex w-full items-center justify-between gap-2 py-2 text-left"
			>
				{@render trigger('Cross-instance summary', 'this instance in host:all')}
				<Accordion.ItemIndicator>
					<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
				</Accordion.ItemIndicator>
			</Accordion.ItemTrigger>
			<Accordion.ItemContent class="pb-3">
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
					<StatTile
						label="instance"
						display={asStr(sum.instance)}
						statusKind={kindFor(sum.instance)}
						title={asStr(sum.instance)}
						icon={tagIcon}
					/>
					<StatTile
						label="phase"
						display={asStr(sum.phase)}
						statusKind={phaseKind(asStr(sum.phase))}
						icon={activityIcon}
					/>
					<StatTile
						label="title"
						display={asStr(sum.title)}
						statusKind={kindFor(sum.title)}
						title={asStr(sum.title)}
						icon={titleIcon}
					/>
					<StatTile
						label="map"
						display={asStr(sum.map)}
						statusKind={kindFor(sum.map)}
						title={asStr(sum.map)}
						icon={mapIcon}
					/>
					<StatTile
						label="gametype"
						display={asStr(sum.gametype)}
						statusKind={kindFor(sum.gametype)}
						title={asStr(sum.gametype)}
						icon={swordsIcon}
					/>
					<StatTile
						label="score_summary"
						display={asStr(sum.score_summary)}
						statusKind={kindFor(sum.score_summary)}
						title={asStr(sum.score_summary)}
						icon={trophyIcon}
					/>
					<StatTile label="last_successful_read" rows={lastSummaryReadRows} icon={clockIcon} />
				</div>
			</Accordion.ItemContent>
		</Accordion.Item>
	{/if}

	<Accordion.Item value="freshness">
		<Accordion.ItemTrigger
			class="group flex w-full items-center justify-between gap-2 py-2 text-left"
		>
			{@render trigger('Envelope freshness', 'time since last receive')}
			<Accordion.ItemIndicator>
				<ChevronDownIcon class="size-4 transition group-data-[state=open]:rotate-180" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		<Accordion.ItemContent class="pb-3">
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
				<StatTile
					label="game_data_age"
					display={runtimeVm.gameDataStr}
					statusKind={runtimeVm.gameDataStr !== 'never' ? 'on' : 'none'}
					icon={databaseIcon}
				/>
				<StatTile
					label="tick_age"
					display={runtimeVm.tickStr}
					statusKind={runtimeVm.tickStr !== 'never' ? 'on' : 'none'}
					icon={zapIcon}
				/>
				<StatTile
					label="events_reply_age"
					display={runtimeVm.lastEventsReplyStr}
					statusKind={runtimeVm.lastEventsReplyStr !== 'never' ? 'on' : 'none'}
					icon={listIcon}
				/>
				<StatTile
					label="events_reply_phase"
					display={runtimeVm.lastEventsReplyPhase ?? '—'}
					statusKind={runtimeVm.lastEventsReplyPhase
						? phaseKind(runtimeVm.lastEventsReplyPhase)
						: 'none'}
					icon={activityIcon}
				/>
			</div>
		</Accordion.ItemContent>
	</Accordion.Item>
</Accordion>
