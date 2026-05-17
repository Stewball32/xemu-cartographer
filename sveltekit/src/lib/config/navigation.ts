import { BoxIcon, FilmIcon, SettingsIcon, UsersIcon } from '@lucide/svelte';
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
	label: string;
	href?: string;
	links: NavLink[];
	adminOnly?: boolean;
}

export const mainGroups: NavGroup[] = [
	{
		label: 'Overlays',
		links: [{ label: 'Players', href: '/overlays/players/', icon: UsersIcon, showInBar: true }]
	},
	{
		label: 'Admin',
		adminOnly: true,
		links: [
			{ label: 'Pod', href: '/pod/', icon: BoxIcon, showInBar: true },
			{
				label: 'Capture',
				href: '/capture-policies/',
				icon: FilmIcon,
				showInBar: false
			}
		]
	}
];

export const mainLinks: NavLink[] = mainGroups.flatMap((g) => g.links);

export const footerLinks: NavLink[] = [
	{ label: 'Settings', href: '/settings/', icon: SettingsIcon, showInBar: true }
];
