// notification-render.ts — M23b
//
// Parses payload_json (opaque on the wire) into typed shapes and produces
// the title + description + link-out the bell + /notifications/ page use
// to render each row. The per-type structs mirror those in
// internal/notifications/notification_*.go; keep these in sync when adding
// a new notification type.

import { resolve } from '$app/paths';

export interface RenderedNotification {
	title: string;
	description: string;
	href: string | null;
}

interface TeamInvitePayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	initiated_by_id: string;
	initiated_by_name?: string;
	initiated_by_handle: string;
}

interface TeamInviteAcceptedPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	acceptor_id: string;
	acceptor_name?: string;
	acceptor_handle: string;
}

interface TeamInviteDeclinedPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	decliner_id: string;
	decliner_name?: string;
	decliner_handle: string;
}

interface TeamInviteCancelledPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	cancelled_by_id: string;
	cancelled_by_name?: string;
	cancelled_handle?: string;
	by_soft_delete?: boolean;
}

interface TeamJoinRequestPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	requester_id: string;
	requester_name?: string;
	requester_handle: string;
}

interface TeamJoinRequestAcceptedPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	acceptor_id: string;
	acceptor_name?: string;
	acceptor_handle: string;
}

interface TeamJoinRequestDeclinedPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	decliner_id: string;
	decliner_name?: string;
	decliner_handle: string;
}

interface TeamJoinRequestCancelledPayload {
	request_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	cancelled_by_id: string;
	cancelled_by_name?: string;
	cancelled_handle?: string;
	by_soft_delete?: boolean;
}

interface TeamRemovedPayload {
	roster_id: string;
	team_id: string;
	team_name: string;
	team_slug: string;
	removed_gamertag: string;
	removed_by_id: string;
	removed_by_name?: string;
	removed_by_handle: string;
}

function tryParse<T>(raw: string): T | null {
	if (!raw) return null;
	try {
		return JSON.parse(raw) as T;
	} catch {
		return null;
	}
}

function teamHref(slug: string): string {
	return `${resolve('/teams/')}${slug}/`;
}

function whoLabel(name: string | undefined, handle: string): string {
	if (name && name.trim() !== '') return `${name} (@${handle})`;
	return `@${handle}`;
}

export function renderNotification(type: string, payloadJson: string): RenderedNotification {
	switch (type) {
		case 'team_invite': {
			const p = tryParse<TeamInvitePayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `Invited to ${p.team_name}`,
				description: `${whoLabel(p.initiated_by_name, p.initiated_by_handle)} invited you to join.`,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_invite_accepted': {
			const p = tryParse<TeamInviteAcceptedPayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `${p.team_name} — invite accepted`,
				description: `${whoLabel(p.acceptor_name, p.acceptor_handle)} accepted your invite.`,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_invite_declined': {
			const p = tryParse<TeamInviteDeclinedPayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `${p.team_name} — invite declined`,
				description: `${whoLabel(p.decliner_name, p.decliner_handle)} declined your invite.`,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_invite_cancelled': {
			const p = tryParse<TeamInviteCancelledPayload>(payloadJson);
			if (!p) return fallback(type);
			const handle = p.cancelled_handle ?? '';
			const tail = p.by_soft_delete
				? 'The inviter deleted their account.'
				: `${whoLabel(p.cancelled_by_name, handle)} cancelled the invite.`;
			return {
				title: `${p.team_name} — invite cancelled`,
				description: tail,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_join_request': {
			const p = tryParse<TeamJoinRequestPayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `${p.team_name} — join request`,
				description: `${whoLabel(p.requester_name, p.requester_handle)} requested to join.`,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_join_request_accepted': {
			const p = tryParse<TeamJoinRequestAcceptedPayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `${p.team_name} — request accepted`,
				description: `${whoLabel(p.acceptor_name, p.acceptor_handle)} accepted your join request.`,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_join_request_declined': {
			const p = tryParse<TeamJoinRequestDeclinedPayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `${p.team_name} — request declined`,
				description: `${whoLabel(p.decliner_name, p.decliner_handle)} declined your join request.`,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_join_request_cancelled': {
			const p = tryParse<TeamJoinRequestCancelledPayload>(payloadJson);
			if (!p) return fallback(type);
			const handle = p.cancelled_handle ?? '';
			const tail = p.by_soft_delete
				? 'The requester deleted their account.'
				: `${whoLabel(p.cancelled_by_name, handle)} cancelled the request.`;
			return {
				title: `${p.team_name} — request cancelled`,
				description: tail,
				href: teamHref(p.team_slug)
			};
		}
		case 'team_removed': {
			const p = tryParse<TeamRemovedPayload>(payloadJson);
			if (!p) return fallback(type);
			return {
				title: `Removed from ${p.team_name}`,
				description: `${whoLabel(p.removed_by_name, p.removed_by_handle)} removed @${p.removed_gamertag}.`,
				href: teamHref(p.team_slug)
			};
		}
		default:
			return fallback(type);
	}
}

function fallback(type: string): RenderedNotification {
	return {
		title: 'Notification',
		description: `(unknown type: ${type})`,
		href: null
	};
}

// Notification types whose row carries inline Accept/Decline actions on
// the bell + /notifications/ rendering. Both team_invite and
// team_join_request are dual-action (recipient/owner can accept or
// decline); other types are info-only and click-through.
export function isActionableInvite(type: string): boolean {
	return type === 'team_invite' || type === 'team_join_request';
}

// Pulls the request_id out of an actionable notification's payload so the
// inline buttons can call /api/team-membership-requests/{id}/... without
// re-parsing the per-type struct from the call site.
export function extractRequestID(type: string, payloadJson: string): string | null {
	if (!isActionableInvite(type)) return null;
	try {
		const obj = JSON.parse(payloadJson || '{}') as { request_id?: string };
		return obj.request_id ?? null;
	} catch {
		return null;
	}
}
