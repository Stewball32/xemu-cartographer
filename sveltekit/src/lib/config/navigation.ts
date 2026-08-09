import {
	BookmarkXIcon,
	BoxIcon,
	GamepadIcon,
	IdCardIcon,
	LibraryIcon,
	PlayIcon,
	SettingsIcon,
	ShieldIcon,
	SwordsIcon,
	TrophyIcon,
	TvIcon,
	UserIcon,
	UsersIcon,
	WandSparklesIcon
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
	organizerOnly?: boolean;
}

export interface NavGroup {
	label?: string;
	href?: string;
	icon?: Component;
	links: NavLink[];
	adminOnly?: boolean;
	organizerOnly?: boolean;
}

export const mainGroups: NavGroup[] = [
	{
		links: [
			{ label: 'Play', href: '/play/', icon: PlayIcon, showInBar: true },
			{ label: 'Gamertag', href: '/gamertag/', icon: IdCardIcon },
			{ label: 'Series', href: '/series/', icon: TrophyIcon },
			{ label: 'Teams', href: '/teams/', icon: UsersIcon }
		]
	},
	{
		label: 'Organizer',
		href: '/organizer/creator/',
		icon: SwordsIcon,
		organizerOnly: true,
		links: [
			{ label: 'Creator', href: '/organizer/creator/', icon: WandSparklesIcon },
			{ label: 'Games', href: '/organizer/games/', icon: LibraryIcon }
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
			// Routes outside the admin layout (RequireAuth + manage-overlays gate) so
			// overlay_managers can use them directly. Studio is the OBS
			// browser-source catalog (the LAN_OBS_Browser_Sources pack: POV overlay +
			// scorebug + leaderboard + postgame, with copy-able URLs).
			{ label: 'Studio', href: '/studio/', icon: TvIcon }
		]
	}
];

export const mainLinks: NavLink[] = mainGroups.flatMap((g) => g.links);

export const footerLinks: NavLink[] = [
	{ label: 'Settings', href: '/settings/', icon: SettingsIcon, showInBar: true }
];
