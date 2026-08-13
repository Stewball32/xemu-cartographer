<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked
	import { themes } from './themes.js';

	let { seconds = 3, max = 5, theme = 'ffa' } = $props();

	const t = $derived(themes[theme] ?? themes.ffa);
	const deg = $derived(Math.max(0, Math.min(360, (seconds / max) * 360)));
</script>

<div class="ring-wrap">
	<div
		class="ring"
		style="background: conic-gradient({t.accent} 0deg {deg}deg, {t.glow} {deg}deg 360deg); --glow:{t.glow}"
	>
		<div class="ring-inner"><span class="secs">{Math.ceil(seconds)}</span></div>
	</div>
	<span class="label" style="color:{t.tagColor}; border-color:{t.border}">RESPAWNING</span>
</div>

<style>
	.ring-wrap {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
		font-family: Inter, sans-serif;
	}
	.ring {
		width: 118px;
		height: 118px;
		border-radius: 50%;
		box-shadow: 0 0 26px var(--glow);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.ring-inner {
		width: 98px;
		height: 98px;
		border-radius: 50%;
		background: rgba(11, 14, 26, 0.92);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.12);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.secs {
		font-size: 46px;
		font-weight: 700;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.label {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.32em;
		background: rgba(11, 14, 26, 0.88);
		border: 1px solid;
		border-radius: 999px;
		padding: 5px 12px 5px 16px;
	}
</style>
