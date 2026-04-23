<script lang="ts">
	import { ChevronLeftIcon, ChevronRightIcon, PlusIcon } from '@lucide/svelte';

	interface CalendarEvent {
		day: number;
		title: string;
		time: string;
		preset: string;
	}

	const monthNames = [
		'January',
		'February',
		'March',
		'April',
		'May',
		'June',
		'July',
		'August',
		'September',
		'October',
		'November',
		'December'
	];

	const weekdays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

	let viewYear = $state(2026);
	let viewMonth = $state(3); // April (0-indexed)
	let selectedDay = $state(12);

	const events: CalendarEvent[] = [
		{ day: 3, title: 'Sprint planning', time: '10:00 AM', preset: 'preset-filled-primary-500' },
		{ day: 7, title: 'Design review', time: '2:00 PM', preset: 'preset-filled-secondary-500' },
		{ day: 10, title: 'Customer call', time: '11:30 AM', preset: 'preset-filled-tertiary-500' },
		{ day: 12, title: 'Team standup', time: '9:00 AM', preset: 'preset-filled-primary-500' },
		{ day: 12, title: 'Lunch with Alex', time: '12:30 PM', preset: 'preset-filled-success-500' },
		{ day: 12, title: 'Release v2.5', time: '4:00 PM', preset: 'preset-filled-warning-500' },
		{ day: 15, title: 'Board meeting', time: '3:00 PM', preset: 'preset-filled-error-500' },
		{ day: 18, title: 'All-hands', time: '10:00 AM', preset: 'preset-filled-primary-500' },
		{ day: 22, title: 'Security audit', time: '9:00 AM', preset: 'preset-filled-warning-500' },
		{ day: 25, title: 'Offsite kickoff', time: 'All day', preset: 'preset-filled-secondary-500' },
		{ day: 29, title: 'Retro', time: '2:00 PM', preset: 'preset-filled-tertiary-500' }
	];

	let firstWeekday = $derived(new Date(viewYear, viewMonth, 1).getDay());
	let daysInMonth = $derived(new Date(viewYear, viewMonth + 1, 0).getDate());
	let grid = $derived.by(() => {
		const cells: (number | null)[] = [];
		for (let i = 0; i < firstWeekday; i++) cells.push(null);
		for (let d = 1; d <= daysInMonth; d++) cells.push(d);
		while (cells.length % 7 !== 0) cells.push(null);
		return cells;
	});

	let selectedEvents = $derived(events.filter((e) => e.day === selectedDay));

	function prevMonth() {
		if (viewMonth === 0) {
			viewMonth = 11;
			viewYear--;
		} else {
			viewMonth--;
		}
	}

	function nextMonth() {
		if (viewMonth === 11) {
			viewMonth = 0;
			viewYear++;
		} else {
			viewMonth++;
		}
	}

	function eventsForDay(day: number) {
		return events.filter((e) => e.day === day);
	}
</script>

<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
	<div>
		<h1 class="h2">Calendar</h1>
		<p class="text-sm opacity-70">Month view — click a day to see events.</p>
	</div>
	<button class="btn preset-filled">
		<PlusIcon class="size-4" />
		<span>New Event</span>
	</button>
</div>

<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
	<div class="card p-6 lg:col-span-2">
		<div class="mb-4 flex items-center justify-between">
			<h2 class="h4">{monthNames[viewMonth]} {viewYear}</h2>
			<div class="flex items-center gap-1">
				<button class="btn-icon preset-tonal" aria-label="Previous month" onclick={prevMonth}>
					<ChevronLeftIcon class="size-4" />
				</button>
				<button class="btn-icon preset-tonal" aria-label="Next month" onclick={nextMonth}>
					<ChevronRightIcon class="size-4" />
				</button>
			</div>
		</div>

		<div class="grid grid-cols-7 gap-1 text-center text-xs font-semibold opacity-60">
			{#each weekdays as w (w)}
				<div class="py-2">{w}</div>
			{/each}
		</div>

		<div class="grid grid-cols-7 gap-1">
			{#each grid as day, i (i)}
				{#if day === null}
					<div class="aspect-square"></div>
				{:else}
					{@const dayEvents = eventsForDay(day)}
					<button
						class="flex aspect-square flex-col items-start gap-1 rounded-lg p-1.5 text-left text-xs transition hover:preset-tonal-primary {selectedDay ===
						day
							? 'preset-filled-primary-500'
							: 'preset-outlined-surface-200-800'}"
						onclick={() => (selectedDay = day)}
					>
						<span class="font-semibold">{day}</span>
						<div class="flex flex-wrap gap-0.5">
							{#each dayEvents.slice(0, 3) as _e, idx (idx)}
								<span class="size-1.5 rounded-full {_e.preset}"></span>
							{/each}
						</div>
					</button>
				{/if}
			{/each}
		</div>
	</div>

	<div class="space-y-4 card p-6">
		<div>
			<h3 class="h4">
				{monthNames[viewMonth]}
				{selectedDay}
			</h3>
			<p class="text-sm opacity-70">
				{selectedEvents.length}
				{selectedEvents.length === 1 ? 'event' : 'events'}
			</p>
		</div>

		{#if selectedEvents.length === 0}
			<p class="text-sm opacity-60">No events scheduled.</p>
		{:else}
			<div class="space-y-3">
				{#each selectedEvents as event (event.title + event.time)}
					<div class="flex items-start gap-3 card p-3 card-hover">
						<span class="mt-1 size-3 rounded-full {event.preset}"></span>
						<div class="flex-1">
							<p class="text-sm font-semibold">{event.title}</p>
							<p class="text-xs opacity-60">{event.time}</p>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
