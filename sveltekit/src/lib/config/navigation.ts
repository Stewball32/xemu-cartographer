import {
	LayoutDashboardIcon,
	UserIcon,
	TableIcon,
	SquareKanbanIcon,
	CalendarIcon,
	MessageSquareIcon,
	TagIcon,
	ListChecksIcon,
	InboxIcon,
	FolderIcon,
	UsersIcon,
	CreditCardIcon,
	BookOpenIcon,
	ImageIcon,
	SettingsIcon
} from '@lucide/svelte';
import type { Component } from 'svelte';

export interface NavLink {
	label: string;
	href: string;
	icon: Component;
	showInBar?: boolean;
}

export interface NavGroup {
	label: string;
	links: NavLink[];
}

export const mainGroups: NavGroup[] = [
	{
		label: 'Overview',
		links: [
			{
				label: 'Dashboard',
				href: '/examples/dashboard/',
				icon: LayoutDashboardIcon,
				showInBar: true
			},
			{ label: 'Profile', href: '/examples/profile/', icon: UserIcon }
		]
	},
	{
		label: 'Workspace',
		links: [
			{ label: 'Inbox', href: '/examples/inbox/', icon: InboxIcon, showInBar: true },
			{ label: 'Chat', href: '/examples/chat/', icon: MessageSquareIcon, showInBar: true },
			{ label: 'Calendar', href: '/examples/calendar/', icon: CalendarIcon, showInBar: true },
			{ label: 'Kanban', href: '/examples/kanban/', icon: SquareKanbanIcon }
		]
	},
	{
		label: 'Content',
		links: [
			{ label: 'Files', href: '/examples/files/', icon: FolderIcon },
			{ label: 'Gallery', href: '/examples/gallery/', icon: ImageIcon },
			{ label: 'Docs', href: '/examples/docs/', icon: BookOpenIcon },
			{ label: 'Data Table', href: '/examples/data-table/', icon: TableIcon }
		]
	},
	{
		label: 'Business',
		links: [
			{ label: 'Team', href: '/examples/team/', icon: UsersIcon },
			{ label: 'Billing', href: '/examples/billing/', icon: CreditCardIcon },
			{ label: 'Pricing', href: '/examples/pricing/', icon: TagIcon },
			{ label: 'Wizard', href: '/examples/wizard/', icon: ListChecksIcon }
		]
	}
];

export const mainLinks: NavLink[] = mainGroups.flatMap((g) => g.links);

export const footerLinks: NavLink[] = [
	{ label: 'Settings', href: '/settings/', icon: SettingsIcon, showInBar: true }
];
