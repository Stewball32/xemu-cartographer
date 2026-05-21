<script lang="ts">
	import {
		Accordion,
		Pagination,
		Popover,
		Portal,
		Progress,
		SegmentedControl,
		Slider,
		Steps,
		Switch,
		Tabs,
		Tooltip
	} from '@skeletonlabs/skeleton-svelte';
	import {
		BellIcon,
		CircleCheckBigIcon,
		HomeIcon,
		InfoIcon,
		MapPinIcon,
		StarIcon
	} from '@lucide/svelte';
	import { resolve } from '$app/paths';
	import { APP_NAME } from '$lib/config/app';
	import { toaster } from '$lib/stores/toaster';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';

	type ColorName =
		| 'primary'
		| 'secondary'
		| 'tertiary'
		| 'success'
		| 'warning'
		| 'error'
		| 'surface';

	// Tailwind v4 needs literal class strings in source. Each swatch lists its
	// `bg-…-<step>` and `text-…-contrast-<step>` so the scanner sees them.
	type Swatch = { step: number; bg: string; text: string };
	type PaletteRow = { name: ColorName; steps: Swatch[] };

	const PALETTE: PaletteRow[] = [
		{
			name: 'primary',
			steps: [
				{ step: 50, bg: 'bg-primary-50', text: 'text-primary-contrast-50' },
				{ step: 100, bg: 'bg-primary-100', text: 'text-primary-contrast-100' },
				{ step: 200, bg: 'bg-primary-200', text: 'text-primary-contrast-200' },
				{ step: 300, bg: 'bg-primary-300', text: 'text-primary-contrast-300' },
				{ step: 400, bg: 'bg-primary-400', text: 'text-primary-contrast-400' },
				{ step: 500, bg: 'bg-primary-500', text: 'text-primary-contrast-500' },
				{ step: 600, bg: 'bg-primary-600', text: 'text-primary-contrast-600' },
				{ step: 700, bg: 'bg-primary-700', text: 'text-primary-contrast-700' },
				{ step: 800, bg: 'bg-primary-800', text: 'text-primary-contrast-800' },
				{ step: 900, bg: 'bg-primary-900', text: 'text-primary-contrast-900' },
				{ step: 950, bg: 'bg-primary-950', text: 'text-primary-contrast-950' }
			]
		},
		{
			name: 'secondary',
			steps: [
				{ step: 50, bg: 'bg-secondary-50', text: 'text-secondary-contrast-50' },
				{ step: 100, bg: 'bg-secondary-100', text: 'text-secondary-contrast-100' },
				{ step: 200, bg: 'bg-secondary-200', text: 'text-secondary-contrast-200' },
				{ step: 300, bg: 'bg-secondary-300', text: 'text-secondary-contrast-300' },
				{ step: 400, bg: 'bg-secondary-400', text: 'text-secondary-contrast-400' },
				{ step: 500, bg: 'bg-secondary-500', text: 'text-secondary-contrast-500' },
				{ step: 600, bg: 'bg-secondary-600', text: 'text-secondary-contrast-600' },
				{ step: 700, bg: 'bg-secondary-700', text: 'text-secondary-contrast-700' },
				{ step: 800, bg: 'bg-secondary-800', text: 'text-secondary-contrast-800' },
				{ step: 900, bg: 'bg-secondary-900', text: 'text-secondary-contrast-900' },
				{ step: 950, bg: 'bg-secondary-950', text: 'text-secondary-contrast-950' }
			]
		},
		{
			name: 'tertiary',
			steps: [
				{ step: 50, bg: 'bg-tertiary-50', text: 'text-tertiary-contrast-50' },
				{ step: 100, bg: 'bg-tertiary-100', text: 'text-tertiary-contrast-100' },
				{ step: 200, bg: 'bg-tertiary-200', text: 'text-tertiary-contrast-200' },
				{ step: 300, bg: 'bg-tertiary-300', text: 'text-tertiary-contrast-300' },
				{ step: 400, bg: 'bg-tertiary-400', text: 'text-tertiary-contrast-400' },
				{ step: 500, bg: 'bg-tertiary-500', text: 'text-tertiary-contrast-500' },
				{ step: 600, bg: 'bg-tertiary-600', text: 'text-tertiary-contrast-600' },
				{ step: 700, bg: 'bg-tertiary-700', text: 'text-tertiary-contrast-700' },
				{ step: 800, bg: 'bg-tertiary-800', text: 'text-tertiary-contrast-800' },
				{ step: 900, bg: 'bg-tertiary-900', text: 'text-tertiary-contrast-900' },
				{ step: 950, bg: 'bg-tertiary-950', text: 'text-tertiary-contrast-950' }
			]
		},
		{
			name: 'success',
			steps: [
				{ step: 50, bg: 'bg-success-50', text: 'text-success-contrast-50' },
				{ step: 100, bg: 'bg-success-100', text: 'text-success-contrast-100' },
				{ step: 200, bg: 'bg-success-200', text: 'text-success-contrast-200' },
				{ step: 300, bg: 'bg-success-300', text: 'text-success-contrast-300' },
				{ step: 400, bg: 'bg-success-400', text: 'text-success-contrast-400' },
				{ step: 500, bg: 'bg-success-500', text: 'text-success-contrast-500' },
				{ step: 600, bg: 'bg-success-600', text: 'text-success-contrast-600' },
				{ step: 700, bg: 'bg-success-700', text: 'text-success-contrast-700' },
				{ step: 800, bg: 'bg-success-800', text: 'text-success-contrast-800' },
				{ step: 900, bg: 'bg-success-900', text: 'text-success-contrast-900' },
				{ step: 950, bg: 'bg-success-950', text: 'text-success-contrast-950' }
			]
		},
		{
			name: 'warning',
			steps: [
				{ step: 50, bg: 'bg-warning-50', text: 'text-warning-contrast-50' },
				{ step: 100, bg: 'bg-warning-100', text: 'text-warning-contrast-100' },
				{ step: 200, bg: 'bg-warning-200', text: 'text-warning-contrast-200' },
				{ step: 300, bg: 'bg-warning-300', text: 'text-warning-contrast-300' },
				{ step: 400, bg: 'bg-warning-400', text: 'text-warning-contrast-400' },
				{ step: 500, bg: 'bg-warning-500', text: 'text-warning-contrast-500' },
				{ step: 600, bg: 'bg-warning-600', text: 'text-warning-contrast-600' },
				{ step: 700, bg: 'bg-warning-700', text: 'text-warning-contrast-700' },
				{ step: 800, bg: 'bg-warning-800', text: 'text-warning-contrast-800' },
				{ step: 900, bg: 'bg-warning-900', text: 'text-warning-contrast-900' },
				{ step: 950, bg: 'bg-warning-950', text: 'text-warning-contrast-950' }
			]
		},
		{
			name: 'error',
			steps: [
				{ step: 50, bg: 'bg-error-50', text: 'text-error-contrast-50' },
				{ step: 100, bg: 'bg-error-100', text: 'text-error-contrast-100' },
				{ step: 200, bg: 'bg-error-200', text: 'text-error-contrast-200' },
				{ step: 300, bg: 'bg-error-300', text: 'text-error-contrast-300' },
				{ step: 400, bg: 'bg-error-400', text: 'text-error-contrast-400' },
				{ step: 500, bg: 'bg-error-500', text: 'text-error-contrast-500' },
				{ step: 600, bg: 'bg-error-600', text: 'text-error-contrast-600' },
				{ step: 700, bg: 'bg-error-700', text: 'text-error-contrast-700' },
				{ step: 800, bg: 'bg-error-800', text: 'text-error-contrast-800' },
				{ step: 900, bg: 'bg-error-900', text: 'text-error-contrast-900' },
				{ step: 950, bg: 'bg-error-950', text: 'text-error-contrast-950' }
			]
		},
		{
			name: 'surface',
			steps: [
				{ step: 50, bg: 'bg-surface-50', text: 'text-surface-contrast-50' },
				{ step: 100, bg: 'bg-surface-100', text: 'text-surface-contrast-100' },
				{ step: 200, bg: 'bg-surface-200', text: 'text-surface-contrast-200' },
				{ step: 300, bg: 'bg-surface-300', text: 'text-surface-contrast-300' },
				{ step: 400, bg: 'bg-surface-400', text: 'text-surface-contrast-400' },
				{ step: 500, bg: 'bg-surface-500', text: 'text-surface-contrast-500' },
				{ step: 600, bg: 'bg-surface-600', text: 'text-surface-contrast-600' },
				{ step: 700, bg: 'bg-surface-700', text: 'text-surface-contrast-700' },
				{ step: 800, bg: 'bg-surface-800', text: 'text-surface-contrast-800' },
				{ step: 900, bg: 'bg-surface-900', text: 'text-surface-contrast-900' },
				{ step: 950, bg: 'bg-surface-950', text: 'text-surface-contrast-950' }
			]
		}
	];

	const SEMANTIC_COLORS: ColorName[] = [
		'primary',
		'secondary',
		'tertiary',
		'success',
		'warning',
		'error'
	];

	// Theme-agnostic Xbox / Halo presets (defined in routes/layout.css). These
	// stay constant across all six Skeleton themes — verify by switching themes
	// and confirming this section's swatches don't change while the rows above
	// do. Literal class strings so Tailwind v4's class scanner picks them up.
	type TeamName =
		| 'team-red'
		| 'team-blue'
		| 'team-green'
		| 'team-yellow'
		| 'team-orange'
		| 'team-purple'
		| 'team-brown'
		| 'team-pink';
	type TeamPaletteRow = { name: TeamName; label: string; steps: Swatch[] };

	const TEAM_PALETTE: TeamPaletteRow[] = [
		{
			name: 'team-red',
			label: 'Red Team',
			steps: [
				{ step: 50, bg: 'bg-team-red-50', text: 'text-team-red-contrast-50' },
				{ step: 100, bg: 'bg-team-red-100', text: 'text-team-red-contrast-100' },
				{ step: 200, bg: 'bg-team-red-200', text: 'text-team-red-contrast-200' },
				{ step: 300, bg: 'bg-team-red-300', text: 'text-team-red-contrast-300' },
				{ step: 400, bg: 'bg-team-red-400', text: 'text-team-red-contrast-400' },
				{ step: 500, bg: 'bg-team-red-500', text: 'text-team-red-contrast-500' },
				{ step: 600, bg: 'bg-team-red-600', text: 'text-team-red-contrast-600' },
				{ step: 700, bg: 'bg-team-red-700', text: 'text-team-red-contrast-700' },
				{ step: 800, bg: 'bg-team-red-800', text: 'text-team-red-contrast-800' },
				{ step: 900, bg: 'bg-team-red-900', text: 'text-team-red-contrast-900' },
				{ step: 950, bg: 'bg-team-red-950', text: 'text-team-red-contrast-950' }
			]
		},
		{
			name: 'team-blue',
			label: 'Blue Team',
			steps: [
				{ step: 50, bg: 'bg-team-blue-50', text: 'text-team-blue-contrast-50' },
				{ step: 100, bg: 'bg-team-blue-100', text: 'text-team-blue-contrast-100' },
				{ step: 200, bg: 'bg-team-blue-200', text: 'text-team-blue-contrast-200' },
				{ step: 300, bg: 'bg-team-blue-300', text: 'text-team-blue-contrast-300' },
				{ step: 400, bg: 'bg-team-blue-400', text: 'text-team-blue-contrast-400' },
				{ step: 500, bg: 'bg-team-blue-500', text: 'text-team-blue-contrast-500' },
				{ step: 600, bg: 'bg-team-blue-600', text: 'text-team-blue-contrast-600' },
				{ step: 700, bg: 'bg-team-blue-700', text: 'text-team-blue-contrast-700' },
				{ step: 800, bg: 'bg-team-blue-800', text: 'text-team-blue-contrast-800' },
				{ step: 900, bg: 'bg-team-blue-900', text: 'text-team-blue-contrast-900' },
				{ step: 950, bg: 'bg-team-blue-950', text: 'text-team-blue-contrast-950' }
			]
		},
		{
			name: 'team-green',
			label: 'Green Team',
			steps: [
				{ step: 50, bg: 'bg-team-green-50', text: 'text-team-green-contrast-50' },
				{ step: 100, bg: 'bg-team-green-100', text: 'text-team-green-contrast-100' },
				{ step: 200, bg: 'bg-team-green-200', text: 'text-team-green-contrast-200' },
				{ step: 300, bg: 'bg-team-green-300', text: 'text-team-green-contrast-300' },
				{ step: 400, bg: 'bg-team-green-400', text: 'text-team-green-contrast-400' },
				{ step: 500, bg: 'bg-team-green-500', text: 'text-team-green-contrast-500' },
				{ step: 600, bg: 'bg-team-green-600', text: 'text-team-green-contrast-600' },
				{ step: 700, bg: 'bg-team-green-700', text: 'text-team-green-contrast-700' },
				{ step: 800, bg: 'bg-team-green-800', text: 'text-team-green-contrast-800' },
				{ step: 900, bg: 'bg-team-green-900', text: 'text-team-green-contrast-900' },
				{ step: 950, bg: 'bg-team-green-950', text: 'text-team-green-contrast-950' }
			]
		},
		{
			name: 'team-yellow',
			label: 'Yellow Team',
			steps: [
				{ step: 50, bg: 'bg-team-yellow-50', text: 'text-team-yellow-contrast-50' },
				{ step: 100, bg: 'bg-team-yellow-100', text: 'text-team-yellow-contrast-100' },
				{ step: 200, bg: 'bg-team-yellow-200', text: 'text-team-yellow-contrast-200' },
				{ step: 300, bg: 'bg-team-yellow-300', text: 'text-team-yellow-contrast-300' },
				{ step: 400, bg: 'bg-team-yellow-400', text: 'text-team-yellow-contrast-400' },
				{ step: 500, bg: 'bg-team-yellow-500', text: 'text-team-yellow-contrast-500' },
				{ step: 600, bg: 'bg-team-yellow-600', text: 'text-team-yellow-contrast-600' },
				{ step: 700, bg: 'bg-team-yellow-700', text: 'text-team-yellow-contrast-700' },
				{ step: 800, bg: 'bg-team-yellow-800', text: 'text-team-yellow-contrast-800' },
				{ step: 900, bg: 'bg-team-yellow-900', text: 'text-team-yellow-contrast-900' },
				{ step: 950, bg: 'bg-team-yellow-950', text: 'text-team-yellow-contrast-950' }
			]
		},
		{
			name: 'team-orange',
			label: 'Orange Team',
			steps: [
				{ step: 50, bg: 'bg-team-orange-50', text: 'text-team-orange-contrast-50' },
				{ step: 100, bg: 'bg-team-orange-100', text: 'text-team-orange-contrast-100' },
				{ step: 200, bg: 'bg-team-orange-200', text: 'text-team-orange-contrast-200' },
				{ step: 300, bg: 'bg-team-orange-300', text: 'text-team-orange-contrast-300' },
				{ step: 400, bg: 'bg-team-orange-400', text: 'text-team-orange-contrast-400' },
				{ step: 500, bg: 'bg-team-orange-500', text: 'text-team-orange-contrast-500' },
				{ step: 600, bg: 'bg-team-orange-600', text: 'text-team-orange-contrast-600' },
				{ step: 700, bg: 'bg-team-orange-700', text: 'text-team-orange-contrast-700' },
				{ step: 800, bg: 'bg-team-orange-800', text: 'text-team-orange-contrast-800' },
				{ step: 900, bg: 'bg-team-orange-900', text: 'text-team-orange-contrast-900' },
				{ step: 950, bg: 'bg-team-orange-950', text: 'text-team-orange-contrast-950' }
			]
		},
		{
			name: 'team-purple',
			label: 'Purple Team',
			steps: [
				{ step: 50, bg: 'bg-team-purple-50', text: 'text-team-purple-contrast-50' },
				{ step: 100, bg: 'bg-team-purple-100', text: 'text-team-purple-contrast-100' },
				{ step: 200, bg: 'bg-team-purple-200', text: 'text-team-purple-contrast-200' },
				{ step: 300, bg: 'bg-team-purple-300', text: 'text-team-purple-contrast-300' },
				{ step: 400, bg: 'bg-team-purple-400', text: 'text-team-purple-contrast-400' },
				{ step: 500, bg: 'bg-team-purple-500', text: 'text-team-purple-contrast-500' },
				{ step: 600, bg: 'bg-team-purple-600', text: 'text-team-purple-contrast-600' },
				{ step: 700, bg: 'bg-team-purple-700', text: 'text-team-purple-contrast-700' },
				{ step: 800, bg: 'bg-team-purple-800', text: 'text-team-purple-contrast-800' },
				{ step: 900, bg: 'bg-team-purple-900', text: 'text-team-purple-contrast-900' },
				{ step: 950, bg: 'bg-team-purple-950', text: 'text-team-purple-contrast-950' }
			]
		},
		{
			name: 'team-brown',
			label: 'Brown Team',
			steps: [
				{ step: 50, bg: 'bg-team-brown-50', text: 'text-team-brown-contrast-50' },
				{ step: 100, bg: 'bg-team-brown-100', text: 'text-team-brown-contrast-100' },
				{ step: 200, bg: 'bg-team-brown-200', text: 'text-team-brown-contrast-200' },
				{ step: 300, bg: 'bg-team-brown-300', text: 'text-team-brown-contrast-300' },
				{ step: 400, bg: 'bg-team-brown-400', text: 'text-team-brown-contrast-400' },
				{ step: 500, bg: 'bg-team-brown-500', text: 'text-team-brown-contrast-500' },
				{ step: 600, bg: 'bg-team-brown-600', text: 'text-team-brown-contrast-600' },
				{ step: 700, bg: 'bg-team-brown-700', text: 'text-team-brown-contrast-700' },
				{ step: 800, bg: 'bg-team-brown-800', text: 'text-team-brown-contrast-800' },
				{ step: 900, bg: 'bg-team-brown-900', text: 'text-team-brown-contrast-900' },
				{ step: 950, bg: 'bg-team-brown-950', text: 'text-team-brown-contrast-950' }
			]
		},
		{
			name: 'team-pink',
			label: 'Pink Team',
			steps: [
				{ step: 50, bg: 'bg-team-pink-50', text: 'text-team-pink-contrast-50' },
				{ step: 100, bg: 'bg-team-pink-100', text: 'text-team-pink-contrast-100' },
				{ step: 200, bg: 'bg-team-pink-200', text: 'text-team-pink-contrast-200' },
				{ step: 300, bg: 'bg-team-pink-300', text: 'text-team-pink-contrast-300' },
				{ step: 400, bg: 'bg-team-pink-400', text: 'text-team-pink-contrast-400' },
				{ step: 500, bg: 'bg-team-pink-500', text: 'text-team-pink-contrast-500' },
				{ step: 600, bg: 'bg-team-pink-600', text: 'text-team-pink-contrast-600' },
				{ step: 700, bg: 'bg-team-pink-700', text: 'text-team-pink-contrast-700' },
				{ step: 800, bg: 'bg-team-pink-800', text: 'text-team-pink-contrast-800' },
				{ step: 900, bg: 'bg-team-pink-900', text: 'text-team-pink-contrast-900' },
				{ step: 950, bg: 'bg-team-pink-950', text: 'text-team-pink-contrast-950' }
			]
		}
	];

	// Xbox controller face button ramps (ABXY) + the controller surface gray
	// for non-ABXY buttons. ABXY ramps unlock the dark-up / bright-down pattern
	// the controller page uses (e.g. bg-xbox-a-900 unpressed, bg-xbox-a-500
	// pressed). Same swatch grid format as TEAM_PALETTE.
	type XboxRampName = 'xbox-a' | 'xbox-b' | 'xbox-x' | 'xbox-y' | 'xbox-controller';
	type XboxRampRow = { name: XboxRampName; label: string; steps: Swatch[] };

	const XBOX_RAMPS: XboxRampRow[] = [
		{
			name: 'xbox-a',
			label: 'A button (green)',
			steps: [
				{ step: 50, bg: 'bg-xbox-a-50', text: 'text-xbox-a-contrast-50' },
				{ step: 100, bg: 'bg-xbox-a-100', text: 'text-xbox-a-contrast-100' },
				{ step: 200, bg: 'bg-xbox-a-200', text: 'text-xbox-a-contrast-200' },
				{ step: 300, bg: 'bg-xbox-a-300', text: 'text-xbox-a-contrast-300' },
				{ step: 400, bg: 'bg-xbox-a-400', text: 'text-xbox-a-contrast-400' },
				{ step: 500, bg: 'bg-xbox-a-500', text: 'text-xbox-a-contrast-500' },
				{ step: 600, bg: 'bg-xbox-a-600', text: 'text-xbox-a-contrast-600' },
				{ step: 700, bg: 'bg-xbox-a-700', text: 'text-xbox-a-contrast-700' },
				{ step: 800, bg: 'bg-xbox-a-800', text: 'text-xbox-a-contrast-800' },
				{ step: 900, bg: 'bg-xbox-a-900', text: 'text-xbox-a-contrast-900' },
				{ step: 950, bg: 'bg-xbox-a-950', text: 'text-xbox-a-contrast-950' }
			]
		},
		{
			name: 'xbox-b',
			label: 'B button (red)',
			steps: [
				{ step: 50, bg: 'bg-xbox-b-50', text: 'text-xbox-b-contrast-50' },
				{ step: 100, bg: 'bg-xbox-b-100', text: 'text-xbox-b-contrast-100' },
				{ step: 200, bg: 'bg-xbox-b-200', text: 'text-xbox-b-contrast-200' },
				{ step: 300, bg: 'bg-xbox-b-300', text: 'text-xbox-b-contrast-300' },
				{ step: 400, bg: 'bg-xbox-b-400', text: 'text-xbox-b-contrast-400' },
				{ step: 500, bg: 'bg-xbox-b-500', text: 'text-xbox-b-contrast-500' },
				{ step: 600, bg: 'bg-xbox-b-600', text: 'text-xbox-b-contrast-600' },
				{ step: 700, bg: 'bg-xbox-b-700', text: 'text-xbox-b-contrast-700' },
				{ step: 800, bg: 'bg-xbox-b-800', text: 'text-xbox-b-contrast-800' },
				{ step: 900, bg: 'bg-xbox-b-900', text: 'text-xbox-b-contrast-900' },
				{ step: 950, bg: 'bg-xbox-b-950', text: 'text-xbox-b-contrast-950' }
			]
		},
		{
			name: 'xbox-x',
			label: 'X button (blue)',
			steps: [
				{ step: 50, bg: 'bg-xbox-x-50', text: 'text-xbox-x-contrast-50' },
				{ step: 100, bg: 'bg-xbox-x-100', text: 'text-xbox-x-contrast-100' },
				{ step: 200, bg: 'bg-xbox-x-200', text: 'text-xbox-x-contrast-200' },
				{ step: 300, bg: 'bg-xbox-x-300', text: 'text-xbox-x-contrast-300' },
				{ step: 400, bg: 'bg-xbox-x-400', text: 'text-xbox-x-contrast-400' },
				{ step: 500, bg: 'bg-xbox-x-500', text: 'text-xbox-x-contrast-500' },
				{ step: 600, bg: 'bg-xbox-x-600', text: 'text-xbox-x-contrast-600' },
				{ step: 700, bg: 'bg-xbox-x-700', text: 'text-xbox-x-contrast-700' },
				{ step: 800, bg: 'bg-xbox-x-800', text: 'text-xbox-x-contrast-800' },
				{ step: 900, bg: 'bg-xbox-x-900', text: 'text-xbox-x-contrast-900' },
				{ step: 950, bg: 'bg-xbox-x-950', text: 'text-xbox-x-contrast-950' }
			]
		},
		{
			name: 'xbox-y',
			label: 'Y button (yellow)',
			steps: [
				{ step: 50, bg: 'bg-xbox-y-50', text: 'text-xbox-y-contrast-50' },
				{ step: 100, bg: 'bg-xbox-y-100', text: 'text-xbox-y-contrast-100' },
				{ step: 200, bg: 'bg-xbox-y-200', text: 'text-xbox-y-contrast-200' },
				{ step: 300, bg: 'bg-xbox-y-300', text: 'text-xbox-y-contrast-300' },
				{ step: 400, bg: 'bg-xbox-y-400', text: 'text-xbox-y-contrast-400' },
				{ step: 500, bg: 'bg-xbox-y-500', text: 'text-xbox-y-contrast-500' },
				{ step: 600, bg: 'bg-xbox-y-600', text: 'text-xbox-y-contrast-600' },
				{ step: 700, bg: 'bg-xbox-y-700', text: 'text-xbox-y-contrast-700' },
				{ step: 800, bg: 'bg-xbox-y-800', text: 'text-xbox-y-contrast-800' },
				{ step: 900, bg: 'bg-xbox-y-900', text: 'text-xbox-y-contrast-900' },
				{ step: 950, bg: 'bg-xbox-y-950', text: 'text-xbox-y-contrast-950' }
			]
		},
		{
			name: 'xbox-controller',
			label: 'Controller surface (shoulders, D-pad, Back/Start, sticks)',
			steps: [
				{ step: 50, bg: 'bg-xbox-controller-50', text: 'text-xbox-controller-contrast-50' },
				{ step: 100, bg: 'bg-xbox-controller-100', text: 'text-xbox-controller-contrast-100' },
				{ step: 200, bg: 'bg-xbox-controller-200', text: 'text-xbox-controller-contrast-200' },
				{ step: 300, bg: 'bg-xbox-controller-300', text: 'text-xbox-controller-contrast-300' },
				{ step: 400, bg: 'bg-xbox-controller-400', text: 'text-xbox-controller-contrast-400' },
				{ step: 500, bg: 'bg-xbox-controller-500', text: 'text-xbox-controller-contrast-500' },
				{ step: 600, bg: 'bg-xbox-controller-600', text: 'text-xbox-controller-contrast-600' },
				{ step: 700, bg: 'bg-xbox-controller-700', text: 'text-xbox-controller-contrast-700' },
				{ step: 800, bg: 'bg-xbox-controller-800', text: 'text-xbox-controller-contrast-800' },
				{ step: 900, bg: 'bg-xbox-controller-900', text: 'text-xbox-controller-contrast-900' },
				{ step: 950, bg: 'bg-xbox-controller-950', text: 'text-xbox-controller-contrast-950' }
			]
		}
	];

	// Controller-style buttons. Each row demos the dark-up / bright-down state
	// pattern used by the live controller in routes/containers/[name]. Pairs
	// step-900 background + step-500 border for "up" and step-500 background +
	// step-500 border for "down".
	type ControllerButton = {
		label: string;
		up: string;
		down: string;
	};
	const CONTROLLER_BUTTONS: ControllerButton[] = [
		{
			label: 'A',
			up: 'bg-xbox-a-900 border-2 border-xbox-a-500',
			down: 'bg-xbox-a-500 border-2 border-xbox-a-500'
		},
		{
			label: 'B',
			up: 'bg-xbox-b-900 border-2 border-xbox-b-500',
			down: 'bg-xbox-b-500 border-2 border-xbox-b-500'
		},
		{
			label: 'X',
			up: 'bg-xbox-x-900 border-2 border-xbox-x-500',
			down: 'bg-xbox-x-500 border-2 border-xbox-x-500'
		},
		{
			label: 'Y',
			up: 'bg-xbox-y-900 border-2 border-xbox-y-500',
			down: 'bg-xbox-y-500 border-2 border-xbox-y-500'
		},
		{
			label: 'LB',
			up: 'bg-xbox-controller-900 border-2 border-xbox-controller-500',
			down: 'bg-xbox-controller-500 border-2 border-xbox-controller-500'
		}
	];

	// Black / White face buttons (OG Xbox) — single-tone, displayed as chips.
	type ButtonSwatch = { name: string; bg: string; text: string; ring?: string };
	const XBOX_FACE_CHIPS: ButtonSwatch[] = [
		{
			name: 'Black',
			bg: 'bg-xbox-black',
			text: 'text-xbox-black-contrast',
			ring: 'ring-1 ring-surface-300-700'
		},
		{
			name: 'White',
			bg: 'bg-xbox-white',
			text: 'text-xbox-white-contrast',
			ring: 'ring-1 ring-surface-300-700'
		}
	];

	// Skeleton v4 preset shape:
	//   preset-filled-{color}-{step}     (step required, e.g. -500)
	//   preset-tonal-{color}             (no step)
	//   preset-outlined-{color}-{step}   (step required)
	const BUTTON_VARIANTS = [
		{ label: 'Filled', prefix: 'preset-filled', suffix: '-500' },
		{ label: 'Tonal', prefix: 'preset-tonal', suffix: '' },
		{ label: 'Outlined', prefix: 'preset-outlined', suffix: '-500' }
	] as const;

	// Cards: every size × tone combination from chrome/Card.svelte.
	const CARD_SIZES = ['sm', 'md', 'lg', 'flush'] as const;
	const CARD_TONES = ['tonal', 'filled', 'outlined'] as const;

	// Form state
	let textValue = $state('Map of Blood Gulch');
	let emailValue = $state('not-an-email');
	let textareaValue = $state('Coordinates noted at 47.6062° N, 122.3321° W.');
	let selectValue = $state('contour');
	let checkboxA = $state(true);
	let checkboxB = $state(false);
	let radioValue = $state('topo');
	let switchOn = $state(true);
	let sliderValue = $state([60]);

	// Overlay state
	let dialogOpen = $state(false);

	// Layout/nav state
	let activeTab = $state('overview');
	let segmentValue = $state('contour');
	let stepIndex = $state(0);
	let pageNum = $state(2);
	const PAGE_TOTAL = 47;
	const PAGE_SIZE = 5;

	const STEPS = [
		{ title: 'Survey', body: 'Walk the perimeter and note major landmarks.' },
		{ title: 'Plot', body: 'Lay out the topographic grid in 5m increments.' },
		{ title: 'Annotate', body: 'Add elevation contours, water, and structures.' },
		{ title: 'Publish', body: 'Export the chart and share with the squad.' }
	];

	function fireToast(kind: 'success' | 'error' | 'info' | 'warning') {
		const payload = {
			title: `${kind[0].toUpperCase()}${kind.slice(1)} toast`,
			description: 'This is a sample toast triggered from the theme preview page.'
		};
		switch (kind) {
			case 'success':
				toaster.success(payload);
				break;
			case 'error':
				toaster.error(payload);
				break;
			case 'warning':
				toaster.warning(payload);
				break;
			default:
				toaster.info(payload);
		}
	}
</script>

<svelte:head>
	<title>Theme Preview · {APP_NAME}</title>
</svelte:head>

<div class="mx-auto flex max-w-6xl flex-col gap-10">
	<PageHeader
		title="Theme Preview"
		description="Cartographer Skeleton v4 theme — typography, palette, components, and chrome wrappers. Use the mode toggle in the header to flip between dark and light."
	>
		{#snippet actions()}
			<a class="btn preset-tonal" href={resolve('/')}>
				<HomeIcon class="size-4" />
				<span>Home</span>
			</a>
		{/snippet}
	</PageHeader>

	<!-- ========================================================== TYPOGRAPHY -->
	<section class="space-y-4">
		<h2 class="h3">Typography</h2>
		<Card tone="outlined">
			<div class="space-y-3">
				<h1 class="h1">h1 — Heading One</h1>
				<h2 class="h2">h2 — Heading Two</h2>
				<h3 class="h3">h3 — Heading Three</h3>
				<h4 class="h4">h4 — Heading Four</h4>
				<h5 class="h5">h5 — Heading Five</h5>
				<h6 class="h6">h6 — Heading Six</h6>
				<p>
					The default body paragraph uses the base font size and color. Inline
					<a class="anchor" href={resolve('/debug/theme/')}>anchor links</a>
					follow the theme's primary accent. Some <code>inline code</code> sits at the same
					baseline, and <strong>bold</strong> / <em>italic</em> render with the theme's expected weights.
				</p>
				<blockquote class="blockquote">
					“There are no foreign lands. It is the traveler only who is foreign.” — R. L. Stevenson
				</blockquote>
				<pre class="pre"><code
						>const greeting = "hello, cartographer";
console.log(greeting);</code
					></pre>
				<p class="text-xs text-surface-700-300">
					Helper / caption text — small + muted via <code>text-surface-700-300</code>.
				</p>
			</div>
		</Card>
	</section>

	<!-- ============================================================== PALETTE -->
	<section class="space-y-4">
		<h2 class="h3">Color palette</h2>
		<p class="text-sm text-surface-700-300">
			Each row shows steps 50–950 with the theme's <code>contrast-*</code> text mapping, so you can spot
			any unreadable pairings.
		</p>
		<Card tone="outlined">
			<div class="space-y-3">
				{#each PALETTE as row (row.name)}
					<div>
						<div class="mb-1 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
							{row.name}
						</div>
						<div class="grid grid-cols-11 overflow-hidden rounded-md">
							{#each row.steps as swatch (swatch.step)}
								<div
									class="flex h-12 items-center justify-center text-[10px] font-medium {swatch.bg} {swatch.text}"
									title="{row.name}-{swatch.step}"
								>
									{swatch.step}
								</div>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		</Card>
	</section>

	<!-- ==================================== XBOX / HALO PRESETS (THEME-AGNOSTIC) -->
	<section class="space-y-4">
		<h2 class="h3">Xbox / Halo presets (theme-agnostic)</h2>
		<p class="text-sm text-surface-700-300">
			These tokens are defined once in
			<code>routes/layout.css</code> and stay constant regardless of the active Skeleton theme — switch
			themes from the header and confirm the swatches below don't change while the rows above do.
		</p>
		<Card tone="outlined">
			<div class="space-y-6">
				<div class="space-y-3">
					<div class="text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Halo team colors (Halo 2 set: 8 teams)
					</div>
					{#each TEAM_PALETTE as row (row.name)}
						<div>
							<div class="mb-1 text-xs font-medium text-surface-700-300">
								{row.label} <code class="ml-1">{row.name}</code>
							</div>
							<div class="grid grid-cols-11 overflow-hidden rounded-md">
								{#each row.steps as swatch (swatch.step)}
									<div
										class="flex h-12 items-center justify-center text-[10px] font-medium {swatch.bg} {swatch.text}"
										title="{row.name}-{swatch.step}"
									>
										{swatch.step}
									</div>
								{/each}
							</div>
						</div>
					{/each}
				</div>

				<div class="space-y-3">
					<div class="text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Xbox controller ramps (ABXY + surface)
					</div>
					{#each XBOX_RAMPS as row (row.name)}
						<div>
							<div class="mb-1 text-xs font-medium text-surface-700-300">
								{row.label} <code class="ml-1">{row.name}</code>
							</div>
							<div class="grid grid-cols-11 overflow-hidden rounded-md">
								{#each row.steps as swatch (swatch.step)}
									<div
										class="flex h-12 items-center justify-center text-[10px] font-medium {swatch.bg} {swatch.text}"
										title="{row.name}-{swatch.step}"
									>
										{swatch.step}
									</div>
								{/each}
							</div>
						</div>
					{/each}
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Controller buttons (up / down state)
					</div>
					<p class="mb-2 text-xs text-surface-700-300">
						Pattern used by the live controller at <code>/containers/[name]/</code>: dark fill
						(step-900) + bright border (step-500) when up, bright fill (step-500) + bright border
						(step-500) when down.
					</p>
					<div class="flex flex-wrap items-center gap-4">
						{#each CONTROLLER_BUTTONS as button (button.label)}
							<div class="flex items-center gap-2">
								<button
									type="button"
									class="btn aspect-square rounded-full {button.up}"
									aria-label="{button.label} up"
								>
									{button.label}
								</button>
								<button
									type="button"
									class="btn aspect-square rounded-full {button.down}"
									aria-label="{button.label} down"
								>
									{button.label}
								</button>
							</div>
						{/each}
					</div>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						OG face buttons (Black / White)
					</div>
					<div class="flex flex-wrap items-center gap-3">
						{#each XBOX_FACE_CHIPS as chip (chip.name)}
							<div class="flex flex-col items-center gap-1">
								<div
									class="flex size-10 items-center justify-center rounded-full text-sm font-bold {chip.bg} {chip.text} {chip.ring ??
										''}"
									title={chip.bg}
								>
									{chip.name}
								</div>
								<code class="text-[10px] text-surface-700-300">{chip.bg}</code>
							</div>
						{/each}
					</div>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Xbox brand
					</div>
					<div class="flex flex-wrap items-center gap-3">
						<div class="flex flex-col items-center gap-1">
							<div
								class="flex h-10 items-center justify-center rounded-md bg-xbox-green px-4 text-sm font-bold text-xbox-green-contrast"
							>
								xbox-green
							</div>
							<code class="text-[10px] text-surface-700-300">bg-xbox-green</code>
						</div>
					</div>
				</div>
			</div>
		</Card>
	</section>

	<!-- =========================================== BUTTONS / BADGES / CHIPS -->
	<section class="space-y-4">
		<h2 class="h3">Buttons / Badges / Chips</h2>

		<Card tone="outlined">
			<div class="space-y-6">
				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Button presets × colors
					</div>
					<div class="flex flex-col gap-3">
						{#each BUTTON_VARIANTS as variant (variant.label)}
							<div class="flex flex-wrap items-center gap-2">
								<span class="w-20 text-xs text-surface-700-300">{variant.label}</span>
								{#each SEMANTIC_COLORS as color (color)}
									<button type="button" class="btn {variant.prefix}-{color}{variant.suffix}">
										{color}
									</button>
								{/each}
							</div>
						{/each}
						<div class="flex flex-wrap items-center gap-2">
							<span class="w-20 text-xs text-surface-700-300">Sizes</span>
							<button type="button" class="btn preset-filled-primary-500 btn-sm">Small</button>
							<button type="button" class="btn preset-filled-primary-500">Default</button>
							<button type="button" class="btn preset-filled-primary-500 btn-lg">Large</button>
							<button type="button" class="btn preset-filled-primary-500" disabled>Disabled</button>
							<button type="button" class="btn-icon preset-tonal-primary" aria-label="Star">
								<StarIcon class="size-4" />
							</button>
						</div>
					</div>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Badges
					</div>
					<div class="flex flex-wrap items-center gap-2">
						{#each SEMANTIC_COLORS as color (color)}
							<span class="badge preset-filled-{color}-500">{color}</span>
						{/each}
						{#each SEMANTIC_COLORS as color (color)}
							<span class="badge preset-tonal-{color}">{color}</span>
						{/each}
					</div>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Chips
					</div>
					<div class="flex flex-wrap items-center gap-2">
						{#each SEMANTIC_COLORS as color (color)}
							<span class="chip preset-tonal-{color}">{color}</span>
						{/each}
						<span class="chip preset-filled-primary-500">
							<MapPinIcon class="size-4" />
							<span>With icon</span>
						</span>
					</div>
				</div>
			</div>
		</Card>
	</section>

	<!-- ======================================================= FORM CONTROLS -->
	<section class="space-y-4">
		<h2 class="h3">Form controls</h2>
		<Card tone="outlined">
			<div class="grid gap-4 sm:grid-cols-2">
				<label class="label">
					<span class="label-text">Text input</span>
					<input class="input" type="text" bind:value={textValue} placeholder="Map name" />
				</label>

				<label class="label">
					<span class="label-text">Email (invalid)</span>
					<input
						class="input ring-2 ring-error-500"
						type="email"
						bind:value={emailValue}
						aria-invalid="true"
						placeholder="you@example.com"
					/>
					<span class="text-xs text-error-700-300">Must be a valid email address.</span>
				</label>

				<label class="label">
					<span class="label-text">Select</span>
					<select class="select" bind:value={selectValue}>
						<option value="contour">Contour</option>
						<option value="hillshade">Hillshade</option>
						<option value="satellite">Satellite</option>
					</select>
				</label>

				<label class="label">
					<span class="label-text">Disabled input</span>
					<input class="input" type="text" value="Read-only value" disabled />
				</label>

				<label class="label sm:col-span-2">
					<span class="label-text">Textarea</span>
					<textarea class="textarea" rows="3" bind:value={textareaValue}></textarea>
				</label>

				<div class="flex flex-col gap-2">
					<span class="label-text">Checkboxes</span>
					<label class="flex items-center gap-2">
						<input class="checkbox" type="checkbox" bind:checked={checkboxA} />
						<span>Show contour lines</span>
					</label>
					<label class="flex items-center gap-2">
						<input class="checkbox" type="checkbox" bind:checked={checkboxB} />
						<span>Show grid overlay</span>
					</label>
				</div>

				<div class="flex flex-col gap-2">
					<span class="label-text">Radio</span>
					<label class="flex items-center gap-2">
						<input class="radio" type="radio" bind:group={radioValue} value="topo" />
						<span>Topographic</span>
					</label>
					<label class="flex items-center gap-2">
						<input class="radio" type="radio" bind:group={radioValue} value="political" />
						<span>Political</span>
					</label>
					<label class="flex items-center gap-2">
						<input class="radio" type="radio" bind:group={radioValue} value="terrain" />
						<span>Terrain</span>
					</label>
				</div>

				<div class="flex flex-col gap-2">
					<span class="label-text">Switch</span>
					<Switch
						checked={switchOn}
						onCheckedChange={(d) => (switchOn = d.checked)}
						aria-label="Toggle preview"
					>
						<Switch.Control
							class="inline-flex h-6 w-11 cursor-pointer items-center rounded-full bg-surface-300-700 transition-colors data-[state=checked]:bg-primary-500"
						>
							<Switch.Thumb class="size-5 rounded-full bg-white shadow-sm transition-transform" />
						</Switch.Control>
						<Switch.HiddenInput />
					</Switch>
					<span class="text-xs text-surface-700-300">{switchOn ? 'On' : 'Off'}</span>
				</div>

				<div class="flex flex-col gap-2 sm:col-span-2">
					<span class="label-text">Slider — {sliderValue[0]}</span>
					<Slider
						value={sliderValue}
						onValueChange={(d) => (sliderValue = d.value)}
						min={0}
						max={100}
						step={1}
					>
						<Slider.Control class="relative flex h-6 w-full items-center">
							<Slider.Track class="relative h-1.5 w-full grow rounded-full bg-surface-300-700">
								<Slider.Range class="absolute h-full rounded-full bg-primary-500" />
							</Slider.Track>
							<Slider.Thumb
								index={0}
								class="block size-4 rounded-full bg-primary-500 shadow ring-2 ring-surface-50 focus:outline-none dark:ring-surface-950"
							>
								<Slider.HiddenInput />
							</Slider.Thumb>
						</Slider.Control>
					</Slider>
				</div>

				<div class="flex flex-col gap-2 sm:col-span-2">
					<span class="label-text">Segmented control</span>
					<SegmentedControl
						value={segmentValue}
						onValueChange={(d) => (segmentValue = d.value ?? segmentValue)}
					>
						<SegmentedControl.Indicator />
						<SegmentedControl.Item value="contour">
							<SegmentedControl.ItemText>Contour</SegmentedControl.ItemText>
							<SegmentedControl.ItemHiddenInput />
						</SegmentedControl.Item>
						<SegmentedControl.Item value="hillshade">
							<SegmentedControl.ItemText>Hillshade</SegmentedControl.ItemText>
							<SegmentedControl.ItemHiddenInput />
						</SegmentedControl.Item>
						<SegmentedControl.Item value="satellite">
							<SegmentedControl.ItemText>Satellite</SegmentedControl.ItemText>
							<SegmentedControl.ItemHiddenInput />
						</SegmentedControl.Item>
					</SegmentedControl>
				</div>
			</div>
		</Card>
	</section>

	<!-- ============================================================ OVERLAYS -->
	<section class="space-y-4">
		<h2 class="h3">Overlays</h2>
		<Card tone="outlined">
			<div class="flex flex-wrap items-center gap-3">
				<button
					type="button"
					class="btn preset-filled-primary-500"
					onclick={() => (dialogOpen = true)}
				>
					Open Dialog
				</button>

				<Popover>
					<Popover.Trigger class="btn preset-tonal">
						<BellIcon class="size-4" />
						<span>Open Popover</span>
					</Popover.Trigger>
					<Portal>
						<Popover.Positioner>
							<Popover.Content class="max-w-xs space-y-2 card preset-filled-surface-100-900 p-3">
								<div class="flex items-center gap-2 text-sm font-semibold">
									<InfoIcon class="size-4" />
									<span>Popover content</span>
								</div>
								<p class="text-xs text-surface-700-300">
									Cards inside Popover.Content render with the surface fill so they're readable
									against any backdrop.
								</p>
								<div class="flex justify-end">
									<Popover.CloseTrigger class="btn preset-tonal btn-sm">Close</Popover.CloseTrigger>
								</div>
							</Popover.Content>
						</Popover.Positioner>
					</Portal>
				</Popover>

				<Tooltip positioning={{ placement: 'top' }}>
					<Tooltip.Trigger class="btn preset-outlined">Hover for tooltip</Tooltip.Trigger>
					<Portal>
						<Tooltip.Positioner>
							<Tooltip.Content class="card preset-filled-surface-950-50 px-2 py-1 text-xs">
								<span>Hello cartographer</span>
								<Tooltip.Arrow
									class="[--arrow-background:var(--color-surface-950-50)] [--arrow-size:--spacing(2)]"
								>
									<Tooltip.ArrowTip />
								</Tooltip.Arrow>
							</Tooltip.Content>
						</Tooltip.Positioner>
					</Portal>
				</Tooltip>

				<div class="mx-1 h-8 border-l border-surface-200-800"></div>

				<button type="button" class="btn preset-tonal-success" onclick={() => fireToast('success')}>
					<CircleCheckBigIcon class="size-4" /> <span>Success toast</span>
				</button>
				<button type="button" class="btn preset-tonal-warning" onclick={() => fireToast('warning')}>
					Warning toast
				</button>
				<button type="button" class="btn preset-tonal-error" onclick={() => fireToast('error')}>
					Error toast
				</button>
				<button type="button" class="btn preset-tonal" onclick={() => fireToast('info')}>
					Info toast
				</button>
			</div>
		</Card>

		<Dialog open={dialogOpen} onClose={() => (dialogOpen = false)} title="Sample dialog" size="md">
			<p class="text-sm">
				This is the <code>chrome/Dialog</code> wrapper from M6b. Click the backdrop or press
				<kbd class="kbd">Esc</kbd> to dismiss.
			</p>
			<p class="mt-2 text-sm text-surface-700-300">
				The footer below uses the named <code>footer</code> snippet.
			</p>
			{#snippet footer()}
				<button type="button" class="btn preset-tonal" onclick={() => (dialogOpen = false)}>
					Cancel
				</button>
				<button
					type="button"
					class="btn preset-filled-primary-500"
					onclick={() => (dialogOpen = false)}
				>
					Confirm
				</button>
			{/snippet}
		</Dialog>
	</section>

	<!-- ================================================ LAYOUT / NAVIGATION -->
	<section class="space-y-4">
		<h2 class="h3">Layout / navigation</h2>

		<Card tone="outlined">
			<div class="space-y-6">
				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Tabs
					</div>
					<Tabs value={activeTab} onValueChange={(d) => (activeTab = d.value)}>
						<Tabs.List>
							<Tabs.Trigger value="overview">Overview</Tabs.Trigger>
							<Tabs.Trigger value="players">Players</Tabs.Trigger>
							<Tabs.Trigger value="events">Events</Tabs.Trigger>
							<Tabs.Indicator />
						</Tabs.List>
						<Tabs.Content value="overview" class="pt-3 text-sm">
							A summary of the current map run. This panel renders inside the
							<code>Tabs.Content</code> slot.
						</Tabs.Content>
						<Tabs.Content value="players" class="pt-3 text-sm">
							Player roster goes here — name, team, score, etc.
						</Tabs.Content>
						<Tabs.Content value="events" class="pt-3 text-sm">
							Event log: kills, captures, power-item spawns.
						</Tabs.Content>
					</Tabs>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Accordion
					</div>
					<Accordion value={['general']} multiple>
						<Accordion.Item value="general">
							<Accordion.ItemTrigger class="flex w-full items-center justify-between py-2">
								<span>General settings</span>
								<Accordion.ItemIndicator>+</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-3 text-sm text-surface-700-300">
								Configure general behavior — units, default zoom, autosave interval.
							</Accordion.ItemContent>
						</Accordion.Item>
						<Accordion.Item value="advanced">
							<Accordion.ItemTrigger class="flex w-full items-center justify-between py-2">
								<span>Advanced</span>
								<Accordion.ItemIndicator>+</Accordion.ItemIndicator>
							</Accordion.ItemTrigger>
							<Accordion.ItemContent class="pb-3 text-sm text-surface-700-300">
								Tweak scraper tick rate, broadcast cadence, and event log size.
							</Accordion.ItemContent>
						</Accordion.Item>
					</Accordion>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Steps
					</div>
					<Steps count={STEPS.length} step={stepIndex} onStepChange={(d) => (stepIndex = d.step)}>
						<Steps.List class="flex items-center gap-2">
							{#each STEPS as item, index (item.title)}
								<Steps.Item {index} class="flex items-center gap-2">
									<Steps.Trigger class="flex items-center gap-2">
										<Steps.Indicator
											class="flex size-7 items-center justify-center rounded-full border border-surface-300-700 text-xs data-complete:bg-success-500 data-complete:text-success-contrast-500 data-current:bg-primary-500 data-current:text-primary-contrast-500"
										>
											{index + 1}
										</Steps.Indicator>
										<span class="text-sm">{item.title}</span>
									</Steps.Trigger>
									{#if index < STEPS.length - 1}
										<Steps.Separator class="h-px w-8 bg-surface-300-700" />
									{/if}
								</Steps.Item>
							{/each}
						</Steps.List>
						{#each STEPS as item, index (`content-${item.title}`)}
							<Steps.Content {index} class="pt-3 text-sm">
								{item.body}
							</Steps.Content>
						{/each}
						<div class="mt-3 flex gap-2">
							<Steps.PrevTrigger class="btn preset-tonal btn-sm">Previous</Steps.PrevTrigger>
							<Steps.NextTrigger class="btn preset-filled-primary-500 btn-sm"
								>Next</Steps.NextTrigger
							>
						</div>
					</Steps>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Pagination
					</div>
					<Pagination
						count={PAGE_TOTAL}
						pageSize={PAGE_SIZE}
						page={pageNum}
						onPageChange={(d) => (pageNum = d.page)}
					>
						<div class="flex items-center gap-1">
							<Pagination.PrevTrigger class="btn preset-tonal btn-sm">Prev</Pagination.PrevTrigger>
							<Pagination.Context>
								{#snippet children(api)}
									{#each api().pages as p, i (i)}
										{#if p.type === 'page'}
											<Pagination.Item
												{...p}
												class="btn preset-tonal btn-sm data-selected:preset-filled-primary-500"
											>
												{p.value}
											</Pagination.Item>
										{:else}
											<Pagination.Ellipsis index={i} class="px-1 text-sm">…</Pagination.Ellipsis>
										{/if}
									{/each}
								{/snippet}
							</Pagination.Context>
							<Pagination.NextTrigger class="btn preset-tonal btn-sm">Next</Pagination.NextTrigger>
						</div>
					</Pagination>
					<p class="mt-1 text-xs text-surface-700-300">
						Page {pageNum} of {Math.ceil(PAGE_TOTAL / PAGE_SIZE)}
					</p>
				</div>

				<div>
					<div class="mb-2 text-xs font-semibold tracking-wider text-surface-700-300 uppercase">
						Progress
					</div>
					<div class="grid items-center gap-4 sm:grid-cols-[1fr_auto]">
						<Progress value={sliderValue[0]}>
							<Progress.Track
								class="block h-2 w-full overflow-hidden rounded-full bg-surface-300-700"
							>
								<Progress.Range class="block h-full bg-primary-500 transition-[width]" />
							</Progress.Track>
						</Progress>
						<Progress value={sliderValue[0]}>
							<Progress.Circle class="size-12 [--size:48px] [--thickness:6px]">
								<Progress.CircleTrack class="text-surface-300-700" />
								<Progress.CircleRange class="text-primary-500" />
							</Progress.Circle>
						</Progress>
					</div>
					<p class="mt-1 text-xs text-surface-700-300">
						Driven by the slider above ({sliderValue[0]}%).
					</p>
				</div>
			</div>
		</Card>
	</section>

	<!-- ===================================================== CHROME WRAPPERS -->
	<section class="space-y-4">
		<h2 class="h3">Chrome wrappers (M6b)</h2>
		<p class="text-sm text-surface-700-300">
			The <code>PageHeader</code> at the top, the <code>Dialog</code> in the Overlays section, and
			the cards used throughout this page are all the M6b chrome wrappers. Below: every
			<code>Card</code> size × tone combination.
		</p>

		<div class="grid gap-3 sm:grid-cols-3">
			{#each CARD_TONES as tone (tone)}
				{#each CARD_SIZES as size (size)}
					<Card {size} {tone}>
						<div class="flex items-center justify-between text-sm">
							<span class="font-semibold">size: {size}</span>
							<span class="text-xs text-surface-700-300">{tone}</span>
						</div>
						<p class="mt-1 text-xs text-surface-700-300">
							{tone === 'tonal'
								? 'preset-tonal — soft surface fill'
								: tone === 'filled'
									? 'preset-filled-surface-100-900 — solid'
									: 'border only — transparent fill'}
						</p>
					</Card>
				{/each}
			{/each}
		</div>
	</section>
</div>
