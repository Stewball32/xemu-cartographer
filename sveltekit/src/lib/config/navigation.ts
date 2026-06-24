import {
	BookmarkXIcon,
	BoxIcon,
	GamepadIcon,
	MonitorIcon,
	PlayIcon,
	SettingsIcon,
	ShieldIcon,
	TrophyIcon,
	TvIcon,
	UserIcon,
	UsersIcon
} from '@lucide/svelte';
import type { Component } from 'svelte';

export interface NavLink {
	label: string;
	href: string;
	icon: Component;
	showInDrawer?: boolean;
	showInRail?: boolean;
	showInBar?: boolean;
	adminOnly?: boolean;
}

export interface NavGroup {
	label?: string;
	href?: string;
	icon?: Component;
	links: NavLink[];
	adminOnly?: boolean;
}

export const mainGroups: NavGroup[] = [
	{
		links: [
			{ label: 'Play', href: '/play/', icon: PlayIcon, showInBar: true },
			{ label: 'Series', href: '/series/', icon: TrophyIcon },
			{ label: 'Teams', href: '/teams/', icon: UsersIcon }
		]
	},
	{
		label: 'Admin',
		href: '/admin/',
		icon: ShieldIcon,
		adminOnly: true,
		links: [
			{ label: 'Pod', href: '/admin/pod/', icon: BoxIcon, showInBar: true },
			{ label: 'LAN saves', href: '/admin/lan/', icon: GamepadIcon },
			{ label: 'Players', href: '/admin/players/', icon: UserIcon },
			{ label: 'Rosters', href: '/admin/rosters/', icon: UsersIcon },
			{ label: 'Roles', href: '/admin/roles/', icon: ShieldIcon },
			{ label: 'Reserved names', href: '/admin/reserved-names/', icon: BookmarkXIcon },
			// Both route outside the admin layout (RequireAuth + manage-overlays
			// gate) so overlay_managers can use them directly. Studio is the
			// stream-asset hub (gallery + previews + per-asset OBS links); Overlays
			// is the focused token mint/list/revoke page.
			{ label: 'Studio', href: '/studio/', icon: TvIcon },
			{ label: 'Overlays', href: '/overlays/manage/', icon: MonitorIcon }
		]
	}
];

export const mainLinks: NavLink[] = mainGroups.flatMap((g) => g.links);

export const footerLinks: NavLink[] = [
	{ label: 'Settings', href: '/settings/', icon: SettingsIcon, showInBar: true }
];
