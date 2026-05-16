// View-model for the System tab's Xbox sections. Sources every field from
// scraperWSV2.xbox[name] (v2 XboxPayload — title + EEPROM + XBE cert +
// kernel clock) with fall-back to the 3 s HTTP-poll inspect endpoint
// while the first xbox-class envelope is in flight.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { GamePayload, XboxPayload } from '$lib/types/scraper-v2';
import type { ScraperInspect } from '$lib/types/scraper';

type ScraperWSV2 = typeof scraperWSV2;

// XBE game-region bitfield masks — mirror internal/scraper/xbox/offsets.go.
const XBE_REGION_NTSC_US = 0x00000001;
const XBE_REGION_NTSC_J = 0x00000002;
const XBE_REGION_PAL = 0x00000004;
const XBE_REGION_DEVKIT = 0x80000000;
const XBE_REGION_ANY = 0x7fffffff;

// XBE allowed-media bitfield masks — names match the public xbe-tool /
// Cxbx-Reloaded constants so a reader cross-referencing those projects sees
// the same labels.
const ALLOWED_MEDIA_FLAGS: Array<[number, string]> = [
	[0x00000001, 'HardDisk'],
	[0x00000002, 'DVD_X2'],
	[0x00000004, 'DVD_CD'],
	[0x00000008, 'CD'],
	[0x00000010, 'DVD_5_RO'],
	[0x00000020, 'DVD_9_RO'],
	[0x00000040, 'DVD_5_RW'],
	[0x00000080, 'DVD_9_RW'],
	[0x00000100, 'Dongle'],
	[0x00000200, 'MediaBoard'],
	[0x40000000, 'NonsecureHardDisk'],
	[0x80000000, 'NonsecureMode']
];
const ALLOWED_MEDIA_ANY = 0x00ffffff;

export type XboxVm = {
	identity: Record<string, unknown> | null;
	xbeCert: Record<string, unknown> | null;
	eeprom: Record<string, unknown> | null;
	kernelClock: Record<string, unknown> | null;
};

function hex8(n: number): string {
	// Force unsigned 32-bit width — Title IDs and bitfields are u32 on the
	// wire; JS bitwise operators sign-extend, so we go via >>> 0.
	const u = n >>> 0;
	return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
}

function formatTimeZoneBias(minutes: number | undefined): string {
	if (minutes === undefined) return '—';
	// Microsoft's TIME_ZONE_INFORMATION convention: positive bias means UTC
	// is N minutes ahead of local time (i.e. local is *west* of UTC). We
	// invert the sign for display so e.g. PST (bias=480) renders as "-08:00".
	const totalMin = -minutes;
	const sign = totalMin < 0 ? '-' : '+';
	const abs = Math.abs(totalMin);
	const hh = Math.floor(abs / 60)
		.toString()
		.padStart(2, '0');
	const mm = (abs % 60).toString().padStart(2, '0');
	return `UTC${sign}${hh}:${mm}`;
}

function formatMac(raw: string | undefined): string {
	if (!raw) return '—';
	// v2 backend (internal/scraper/xbox/mac.go) already returns colon-separated
	// pairs like "00:50:f2:96:b6:6f". The v1 inspect endpoint may still hand
	// over a raw 12-char hex string, so fall back to that case.
	if (raw.includes(':')) return raw.toUpperCase();
	if (raw.length === 12 && /^[0-9A-Fa-f]+$/.test(raw)) {
		return raw.match(/.{2}/g)!.join(':').toUpperCase();
	}
	return raw;
}

function formatRegion(region: number | undefined): string {
	if (region === undefined) return '—';
	if (region === 0) return 'none';
	if (region === XBE_REGION_ANY) return 'World';
	const parts: string[] = [];
	if (region & XBE_REGION_NTSC_US) parts.push('NTSC-US');
	if (region & XBE_REGION_NTSC_J) parts.push('NTSC-J');
	if (region & XBE_REGION_PAL) parts.push('PAL');
	if (region & XBE_REGION_DEVKIT) parts.push('Devkit');
	return parts.length > 0 ? `${parts.join(',')} (${hex8(region)})` : hex8(region);
}

function formatAllowedMedia(media: number | undefined): string {
	if (media === undefined) return '—';
	if (media === 0) return 'none';
	if ((media & ALLOWED_MEDIA_ANY) === ALLOWED_MEDIA_ANY) {
		return `Any (${hex8(media)})`;
	}
	const parts = ALLOWED_MEDIA_FLAGS.filter(([mask]) => (media & mask) !== 0).map(
		([, name]) => name
	);
	return parts.length > 0 ? `${parts.join(',')} (${hex8(media)})` : hex8(media);
}

// formatUptime accepts either seconds (v2 — XboxKernel.uptime_seconds)
// or nanoseconds (v1 inspect endpoint — kernel_uptime_ns), preferring the
// first non-undefined argument. Lets the migration cross over without
// losing the inspect fallback.
function formatUptime(
	seconds: number | undefined,
	nsFallback: number | undefined = undefined
): string {
	let s: number;
	if (seconds !== undefined && Number.isFinite(seconds) && seconds >= 0) {
		s = Math.floor(seconds);
	} else if (nsFallback !== undefined && Number.isFinite(nsFallback) && nsFallback >= 0) {
		s = Math.floor(nsFallback / 1_000_000_000);
	} else {
		return '—';
	}
	const days = Math.floor(s / 86400);
	s -= days * 86400;
	const hh = Math.floor(s / 3600)
		.toString()
		.padStart(2, '0');
	s -= Number(hh) * 3600;
	const mm = Math.floor(s / 60)
		.toString()
		.padStart(2, '0');
	const ss = (s - Number(mm) * 60).toString().padStart(2, '0');
	return days > 0 ? `${days}d ${hh}:${mm}:${ss}` : `${hh}:${mm}:${ss}`;
}

function formatIsoLocal(iso: string | undefined): string {
	if (!iso) return '—';
	const t = Date.parse(iso);
	if (!Number.isFinite(t)) return iso;
	// Locale-aware date/time + the original ISO so a reader can still see the
	// raw wire value.
	return `${new Date(t).toLocaleString()} (${iso})`;
}

// Strip object entries whose value is "—" so KvCard's empty-state kicks in
// when a section's data hasn't been broadcast yet (e.g. between an Idle
// runner attaching and its first system-snapshot tick).
function compact(record: Record<string, unknown>): Record<string, unknown> | null {
	const entries = Object.entries(record).filter(([, v]) => v !== '—' && v !== undefined);
	if (entries.length === 0) return null;
	return Object.fromEntries(entries);
}

export function buildXboxVm(name: string, ws: ScraperWSV2, inspect: ScraperInspect | null): XboxVm {
	const xbox: XboxPayload | null = ws.xbox[name] ?? null;
	const game: GamePayload | null = ws.game[name] ?? null;

	// Identity is sourced from xbox class (title / xbox_name / xbe) plus the
	// runner's started_at from the game class. The inspect fallback keeps
	// the section populated before the first xbox-class envelope arrives.
	const titleIdNum = xbox?.title_id ?? inspect?.title_id;
	const titleStr = xbox?.title || inspect?.title || '';
	const xboxName = xbox?.name || inspect?.xbox_name || '';
	const xbeName = xbox?.xbe?.title_name || inspect?.xbe_title_name || '';
	const startedAt = game?.started_at ?? inspect?.started_at;
	const identity = compact({
		title: titleStr || '—',
		title_id:
			titleIdNum !== undefined && titleIdNum !== 0
				? `${hex8(titleIdNum)} (${titleIdNum >>> 0})`
				: '—',
		xbe_title_name: xbeName || '—',
		xbox_name: xboxName || '—',
		started_at: formatIsoLocal(startedAt)
	});

	const xbeVersion = xbox?.xbe?.version ?? inspect?.xbe_version;
	const xbeRegion = xbox?.xbe?.game_region ?? inspect?.xbe_game_region;
	const xbeDisk = xbox?.xbe?.disk_number ?? inspect?.xbe_disk_number;
	const xbeMedia = xbox?.xbe?.allowed_media ?? inspect?.xbe_allowed_media;
	const xbeCert = compact({
		xbe_version: xbeVersion !== undefined ? `${hex8(xbeVersion)} (${xbeVersion >>> 0})` : '—',
		xbe_game_region: formatRegion(xbeRegion),
		xbe_disk_number: xbeDisk !== undefined ? String(xbeDisk) : '—',
		xbe_allowed_media: formatAllowedMedia(xbeMedia)
	});

	const serial = xbox?.serial_number ?? inspect?.serial_number;
	const mac = xbox?.mac_address ?? inspect?.mac_address;
	const video = xbox?.video_standard ?? inspect?.video_standard;
	const tzBias = xbox?.time_zone?.bias_minutes ?? inspect?.time_zone_bias;
	const tzStd = xbox?.time_zone?.std_name ?? inspect?.time_zone_std_name;
	const tzDlt = xbox?.time_zone?.dlt_name ?? inspect?.time_zone_dlt_name;
	const eeprom = compact({
		serial_number: serial || '—',
		mac_address: formatMac(mac),
		video_standard: video || '—',
		time_zone: formatTimeZoneBias(tzBias),
		time_zone_std_name: tzStd || '—',
		time_zone_dlt_name: tzDlt || '—'
	});

	const sysTime = xbox?.kernel?.system_time ?? inspect?.kernel_system_time;
	const bootTime = xbox?.kernel?.boot_time ?? inspect?.kernel_boot_time;
	const uptimeSec = xbox?.kernel?.uptime_seconds;
	const kernelClock = compact({
		kernel_system_time: formatIsoLocal(sysTime),
		kernel_boot_time: formatIsoLocal(bootTime),
		kernel_uptime: formatUptime(uptimeSec, inspect?.kernel_uptime_ns)
	});

	return { identity, xbeCert, eeprom, kernelClock };
}
