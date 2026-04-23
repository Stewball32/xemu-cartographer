<script lang="ts">
	import { ClockIcon, UserIcon, TagIcon, BookmarkIcon, Share2Icon } from '@lucide/svelte';

	const toc = [
		{ id: 'introduction', label: 'Introduction' },
		{ id: 'installation', label: 'Installation' },
		{ id: 'quickstart', label: 'Quick Start' },
		{ id: 'configuration', label: 'Configuration' },
		{ id: 'next-steps', label: 'Next Steps' }
	];
</script>

<div class="mx-auto max-w-4xl">
	<!-- Breadcrumb -->
	<nav class="mb-4 text-sm opacity-60">
		<span class="cursor-pointer hover:text-primary-500">Docs</span>
		<span class="mx-2">/</span>
		<span class="cursor-pointer hover:text-primary-500">Getting Started</span>
		<span class="mx-2">/</span>
		<span>Introduction</span>
	</nav>

	<!-- Header -->
	<header class="mb-8 space-y-4">
		<div class="flex flex-wrap gap-2">
			<span class="badge preset-tonal-primary text-xs">Guide</span>
			<span class="badge preset-tonal-surface text-xs">Beginner</span>
		</div>
		<h1 class="h1">Getting Started with the Template</h1>
		<p class="text-lg opacity-80">
			A walkthrough of everything you need to spin up your first project — from installing the
			prerequisites to deploying a working build.
		</p>
		<div class="flex flex-wrap items-center gap-4 text-sm opacity-60">
			<span class="flex items-center gap-1">
				<UserIcon class="size-4" />
				Jane Doe
			</span>
			<span class="flex items-center gap-1">
				<ClockIcon class="size-4" />
				Updated Apr 10, 2026 · 8 min read
			</span>
			<span class="flex items-center gap-1">
				<TagIcon class="size-4" />
				v2.5
			</span>
		</div>
		<div class="flex gap-2">
			<button class="btn preset-tonal btn-sm">
				<BookmarkIcon class="size-4" />
				<span>Save</span>
			</button>
			<button class="btn preset-tonal btn-sm">
				<Share2Icon class="size-4" />
				<span>Share</span>
			</button>
		</div>
	</header>

	<div class="grid gap-8 lg:grid-cols-[1fr_14rem]">
		<!-- Article -->
		<article class="space-y-6">
			<section class="space-y-3">
				<h2 id="introduction" class="h3">Introduction</h2>
				<p class="opacity-80">
					This template pairs a Go + PocketBase backend with a SvelteKit frontend powered by
					Skeleton UI v4 and Tailwind CSS v4. It bakes in authentication, a Discord bot, and a
					WebSocket hub — all compiled into a single deployable binary.
				</p>
				<p class="opacity-80">
					The goal is to give you a production-ready starting point without the usual weeks of
					plumbing. Every moving part is wired up and ready to extend, so you can focus on writing
					your actual features.
				</p>
				<blockquote class="border-l-4 border-primary-500 py-2 pl-4 italic opacity-80">
					“The best starter is the one that gets out of your way — this one does.”
				</blockquote>
			</section>

			<section class="space-y-3">
				<h2 id="installation" class="h3">Installation</h2>
				<p class="opacity-80">Clone the repository and install dependencies for both stacks:</p>
				<pre class="overflow-x-auto card preset-outlined-surface-200-800 p-4 text-sm"><code
						>git clone https://github.com/youruser/yourproject
cd yourproject
task install</code
					></pre>
				<p class="opacity-80">
					You’ll need Go 1.22+, Node.js 20+, and <code
						class="rounded bg-surface-200-800 px-1.5 py-0.5 text-xs">pnpm</code
					>
					installed globally. The Taskfile handles everything else.
				</p>
			</section>

			<section class="space-y-3">
				<h2 id="quickstart" class="h3">Quick Start</h2>
				<p class="opacity-80">
					Run the dev server with a single command. It starts the Go backend with hot reload and the
					SvelteKit dev server side-by-side:
				</p>
				<pre class="overflow-x-auto card preset-outlined-surface-200-800 p-4 text-sm"><code
						>task dev</code
					></pre>
				<p class="opacity-80">
					Visit <span class="text-primary-500 underline">http://localhost:8090</span> — the
					SvelteKit app is mounted at the root, and the PocketBase admin lives under
					<code class="rounded bg-surface-200-800 px-1.5 py-0.5 text-xs">/_/</code>.
				</p>
			</section>

			<section class="space-y-3">
				<h2 id="configuration" class="h3">Configuration</h2>
				<p class="opacity-80">
					All environment configuration lives in a single root-level <code
						class="rounded bg-surface-200-800 px-1.5 py-0.5 text-xs">.env</code
					> file. The important knobs are:
				</p>
				<ul class="list-inside list-disc space-y-2 opacity-80">
					<li>
						<code class="rounded bg-surface-200-800 px-1.5 py-0.5 text-xs">PUBLIC_PB_PORT</code> — the
						port the backend listens on
					</li>
					<li>
						<code class="rounded bg-surface-200-800 px-1.5 py-0.5 text-xs">DISCORD_TOKEN</code> — optional
						bot token; leave blank to skip
					</li>
					<li>
						<code class="rounded bg-surface-200-800 px-1.5 py-0.5 text-xs">OAUTH_*</code> — per-provider
						OAuth credentials
					</li>
				</ul>
			</section>

			<section class="space-y-3">
				<h2 id="next-steps" class="h3">Next Steps</h2>
				<p class="opacity-80">
					Now that you have a running project, try adding a new collection schema, a Discord slash
					command, or a custom WebSocket handler. Each follows a simple registration pattern that
					requires only a single new file.
				</p>
			</section>
		</article>

		<!-- TOC -->
		<aside class="hidden lg:block">
			<div class="sticky top-4 space-y-3 card p-4">
				<h3 class="text-xs font-semibold uppercase opacity-60">On this page</h3>
				<ul class="space-y-2 text-sm">
					{#each toc as item (item.id)}
						<li>
							<a
								href="#{item.id}"
								class="block border-l-2 border-surface-200-800 py-1 pl-3 hover:border-primary-500 hover:text-primary-500"
							>
								{item.label}
							</a>
						</li>
					{/each}
				</ul>
			</div>
		</aside>
	</div>
</div>
