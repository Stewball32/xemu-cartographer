import {
	BookmarkXIcon,
	BoxIcon,
	DiscIcon,
	MapIcon,
	MemoryStickIcon,
	PlayIcon,
	ScrollTextIcon,
	SettingsIcon,
	ShieldIcon,
	SwordsIcon,
	TvIcon,
	UserIcon,
	UsersIcon,
	WallpaperIcon
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
			// The old Gamertag entry is gone — its page consolidated into the
			// tabbed /settings/ (Halo 2 / Halo: CE / Stream tabs); /gamertag/
			// still redirects there.
			{ label: 'Play', href: '/play/', icon: PlayIcon, showInBar: true }
		]
	},
	{
		label: 'Organizer',
		href: '/organizer/discs/',
		icon: SwordsIcon,
		organizerOnly: true,
		// One rail link per organizer page (the redesign replaced the old
		// in-page tabs). /organizer/games/ + /organizer/creator/ still redirect.
		links: [
			{ label: 'Offsets', href: '/organizer/offsets/', icon: MemoryStickIcon },
			{ label: 'Discs', href: '/organizer/discs/', icon: DiscIcon },
			{ label: 'Maps', href: '/organizer/maps/', icon: MapIcon },
			{ label: 'Gametypes', href: '/organizer/gametypes/', icon: SwordsIcon },
			{ label: 'Rulesets', href: '/organizer/rulesets/', icon: ScrollTextIcon },
			{ label: 'Nameplates', href: '/organizer/nameplates/', icon: WallpaperIcon }
		]
	},
	{
		label: 'Admin',
		href: '/admin/',
		icon: ShieldIcon,
		adminOnly: true,
		links: [
			{ label: 'Pod', href: '/admin/pod/', icon: BoxIcon, showInBar: true },
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
