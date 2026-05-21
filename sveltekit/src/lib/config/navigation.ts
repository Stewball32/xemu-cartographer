import { BoxIcon, PlayIcon, SettingsIcon, ShieldIcon, TrophyIcon, UsersIcon } from '@lucide/svelte';
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
		links: [{ label: 'Pod', href: '/admin/pod/', icon: BoxIcon, showInBar: true }]
	}
];

export const mainLinks: NavLink[] = mainGroups.flatMap((g) => g.links);

export const footerLinks: NavLink[] = [
	{ label: 'Settings', href: '/settings/', icon: SettingsIcon, showInBar: true }
];
