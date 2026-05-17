// Pretty-view VM for the Xbox debug tab — formats the XboxPayload into four
// section records that mirror the envelope's JSON shape (top-level scalars
// + time_zone + xbe + kernel). Every field is always present; payloads
// missing a field render as '—' so the Pretty view exposes the expected
// envelope structure even before the first read.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type { XboxPayload } from '$lib/types/scraper-v2';

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

// One tile's display state + the raw value (passed through StatTile's
// `title` prop so hovering a formatted tile reveals the underlying
// wire value).
export interface FieldDisplay {
	display: string;
	raw: string;
	present: boolean;
}

export interface XboxPrettyVm {
	topLevel: {
		title: FieldDisplay;
		title_id: FieldDisplay;
		name: FieldDisplay;
		serial_number: FieldDisplay;
		mac_address: FieldDisplay;
		video_standard: FieldDisplay;
	};
	timeZone: {
		bias_minutes: FieldDisplay;
		std_name: FieldDisplay;
		dlt_name: FieldDisplay;
	};
	xbe: {
		title_name: FieldDisplay;
		version: FieldDisplay;
		game_region: FieldDisplay;
		disk_number: FieldDisplay;
		allowed_media: FieldDisplay;
	};
	kernel: {
		system_time: FieldDisplay;
		boot_time: FieldDisplay;
		uptime_seconds: FieldDisplay;
	};
}

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
	// pairs like "00:50:f2:96:b6:6f"; uppercase for consistency with EEPROM
	// dumps in the wild.
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

function formatUptime(seconds: number | undefined): string {
	if (seconds === undefined || !Number.isFinite(seconds) || seconds < 0) return '—';
	let s = Math.floor(seconds);
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
	return `${new Date(t).toLocaleString()} (${iso})`;
}

function field(display: string, raw: string | undefined, present: boolean): FieldDisplay {
	return { display, raw: raw ?? '—', present };
}

function fieldStr(value: string | undefined): FieldDisplay {
	const v = value ?? '';
	return field(v || '—', v || undefined, v.length > 0);
}

function fieldNum(value: number | undefined, render: (n: number) => string): FieldDisplay {
	if (value === undefined) return field('—', undefined, false);
	return field(render(value), String(value), true);
}

export function buildXboxPrettyVm(payload: XboxPayload | null): XboxPrettyVm {
	const tz = payload?.time_zone ?? null;
	const xbe = payload?.xbe ?? null;
	const kr = payload?.kernel ?? null;

	return {
		topLevel: {
			title: fieldStr(payload?.title),
			// title_id=0 is "no title loaded" (same convention the dashboard
			// uses); render as '—' to match the rest of the empty state
			// instead of "0x00000000".
			title_id:
				payload?.title_id !== undefined && payload.title_id !== 0
					? field(
							`${hex8(payload.title_id)} (${payload.title_id >>> 0})`,
							String(payload.title_id),
							true
						)
					: field(
							'—',
							payload?.title_id !== undefined ? String(payload.title_id) : undefined,
							false
						),
			name: fieldStr(payload?.name),
			serial_number: fieldStr(payload?.serial_number),
			mac_address: field(
				formatMac(payload?.mac_address),
				payload?.mac_address || undefined,
				!!payload?.mac_address
			),
			video_standard: fieldStr(payload?.video_standard)
		},
		timeZone: {
			bias_minutes: field(
				formatTimeZoneBias(tz?.bias_minutes),
				tz?.bias_minutes !== undefined ? String(tz.bias_minutes) : undefined,
				tz?.bias_minutes !== undefined
			),
			std_name: fieldStr(tz?.std_name),
			dlt_name: fieldStr(tz?.dlt_name)
		},
		xbe: {
			title_name: fieldStr(xbe?.title_name),
			version: fieldNum(xbe?.version, (n) => `${hex8(n)} (${n >>> 0})`),
			game_region: field(
				formatRegion(xbe?.game_region),
				xbe?.game_region !== undefined ? String(xbe.game_region) : undefined,
				xbe?.game_region !== undefined
			),
			disk_number: fieldNum(xbe?.disk_number, (n) => String(n)),
			allowed_media: field(
				formatAllowedMedia(xbe?.allowed_media),
				xbe?.allowed_media !== undefined ? String(xbe.allowed_media) : undefined,
				xbe?.allowed_media !== undefined
			)
		},
		kernel: {
			system_time: field(
				formatIsoLocal(kr?.system_time),
				kr?.system_time || undefined,
				!!kr?.system_time
			),
			boot_time: field(formatIsoLocal(kr?.boot_time), kr?.boot_time || undefined, !!kr?.boot_time),
			uptime_seconds: field(
				formatUptime(kr?.uptime_seconds),
				kr?.uptime_seconds !== undefined ? String(kr.uptime_seconds) : undefined,
				kr?.uptime_seconds !== undefined
			)
		}
	};
}
