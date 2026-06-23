<script lang="ts">
	// Three.js render of a live VizModel inside the real Blood Gulch structure-BSP
	// mesh (when the geometry cache is present; otherwise an auto-fit world-bounds
	// box). All data shaping + the coordinate remap live in the pure utils
	// ($lib/utils/visualizer-view + $lib/utils/viz3d) — this component does only
	// the GPU wiring: scene/camera/lights, an orbit camera, the level mesh, and
	// reconciled markers (team-colored player capsules with a heading arrow + name
	// label; octahedra for items, boxes for vehicles, spheres for projectiles,
	// discs for spawns, flags as bright octahedra). Everything is created once in
	// onMount; effects sync the cheap per-frame transforms as the feed ticks, and
	// the (expensive) level rebuild is gated on a key so it only runs when the
	// level actually changes — never per tick.
	//
	// This component bridges Svelte to an imperative Three.js renderer: it mounts a
	// WebGL <canvas> + a CSS2D label layer into a bound host element and keeps
	// non-reactive Map/Set caches of GPU objects (a SvelteMap would make the marker
	// sync $effect self-trigger). Both are intentional + correct here, so the two
	// lint rules that flag them don't apply to this file.
	/* eslint-disable svelte/no-dom-manipulating, svelte/prefer-svelte-reactivity */
	import { onMount, onDestroy } from 'svelte';
	import * as THREE from 'three';
	import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
	import { CSS2DRenderer, CSS2DObject } from 'three/examples/jsm/renderers/CSS2DRenderer.js';
	import type { VizModel } from '$lib/utils/visualizer-view';
	import type { BspMesh } from '$lib/utils/game-geometry';
	import {
		haloToThree,
		facingAngle,
		framingFromBounds,
		defaultCameraPosition
	} from '$lib/utils/viz3d';

	let {
		model,
		mesh = null,
		showItems = true,
		showVehicles = true,
		showProjectiles = true,
		showSpawns = false,
		showNames = true
	}: {
		model: VizModel;
		mesh?: BspMesh | null;
		showItems?: boolean;
		showVehicles?: boolean;
		showProjectiles?: boolean;
		showSpawns?: boolean;
		showNames?: boolean;
	} = $props();

	let host: HTMLDivElement;
	let webglFailed = $state(false);
	// Flipped at the end of onMount so the scene-touching effects only fire once
	// the renderer/scene/groups exist (avoids an effect racing onMount).
	let ready = $state(false);

	let renderer: THREE.WebGLRenderer | undefined;
	let labelRenderer: CSS2DRenderer | undefined;
	let scene: THREE.Scene | undefined;
	let camera: THREE.PerspectiveCamera | undefined;
	let controls: OrbitControls | undefined;
	let raf = 0;
	let resizeObs: ResizeObserver | undefined;

	let levelGroup: THREE.Group | undefined;
	let playersGroup: THREE.Group | undefined;
	let itemsGroup: THREE.Group | undefined;
	let vehiclesGroup: THREE.Group | undefined;
	let projGroup: THREE.Group | undefined;
	let spawnsGroup: THREE.Group | undefined;
	let flagsGroup: THREE.Group | undefined;

	// Reconciled player markers, keyed by player index (capsule + heading arrow +
	// name label in one sub-group so a single transform moves all three).
	interface PlayerNode {
		group: THREE.Group;
		body: THREE.Mesh;
		arrow: THREE.Mesh;
		label: CSS2DObject;
		labelEl: HTMLDivElement;
	}
	const playerNodes = new Map<number, PlayerNode>();

	// Shared geometries — created once, reused across every marker of a kind.
	const geom = {
		capsule: new THREE.CapsuleGeometry(0.55, 1.4, 6, 12),
		arrow: new THREE.ConeGeometry(0.32, 0.9, 10),
		item: new THREE.OctahedronGeometry(0.7),
		vehicle: new THREE.BoxGeometry(2.2, 1.1, 3.4),
		proj: new THREE.SphereGeometry(0.28, 8, 8),
		spawn: new THREE.CylinderGeometry(0.9, 0.9, 0.12, 16)
	};
	// Materials cached by (hex, emissive, opacity) so we never allocate per tick.
	const matCache = new Map<string, THREE.MeshStandardMaterial>();
	function mat(
		hex: string,
		opts: { emissive?: boolean; opacity?: number } = {}
	): THREE.MeshStandardMaterial {
		const opacity = opts.opacity ?? 1;
		const key = `${hex}|${opts.emissive ? 1 : 0}|${opacity}`;
		let m = matCache.get(key);
		if (!m) {
			const c = new THREE.Color(hex);
			m = new THREE.MeshStandardMaterial({
				color: c,
				emissive: opts.emissive ? c.clone().multiplyScalar(0.4) : new THREE.Color(0x000000),
				roughness: 0.55,
				metalness: 0,
				transparent: opacity < 1,
				opacity
			});
			matCache.set(key, m);
		}
		return m;
	}

	const ITEM_COLOR: Record<string, string> = {
		weapon: '#e0a32e',
		powerup: '#9b51e0',
		equipment: '#27c0c4',
		other: '#8b97a7'
	};

	let framedOnce = false;

	onMount(() => {
		try {
			renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
		} catch {
			webglFailed = true;
			return;
		}
		renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
		host.appendChild(renderer.domElement);

		labelRenderer = new CSS2DRenderer();
		labelRenderer.domElement.style.position = 'absolute';
		labelRenderer.domElement.style.top = '0';
		labelRenderer.domElement.style.left = '0';
		labelRenderer.domElement.style.pointerEvents = 'none';
		host.appendChild(labelRenderer.domElement);

		scene = new THREE.Scene();
		scene.background = new THREE.Color('#060911');
		scene.fog = new THREE.Fog('#060911', 240, 1100);

		camera = new THREE.PerspectiveCamera(55, 1, 0.5, 6000);

		controls = new OrbitControls(camera, renderer.domElement);
		controls.enableDamping = true;
		controls.dampingFactor = 0.08;
		controls.screenSpacePanning = false;
		controls.maxPolarAngle = Math.PI * 0.495; // keep the camera above the floor

		const hemi = new THREE.HemisphereLight(0xbfd4ff, 0x2a2418, 1.05);
		scene.add(hemi);
		const keyLight = new THREE.DirectionalLight(0xffffff, 1.25);
		keyLight.position.set(120, 220, 80);
		scene.add(keyLight);
		const rim = new THREE.DirectionalLight(0x88aaff, 0.4);
		rim.position.set(-100, 120, -120);
		scene.add(rim);

		levelGroup = new THREE.Group();
		playersGroup = new THREE.Group();
		itemsGroup = new THREE.Group();
		vehiclesGroup = new THREE.Group();
		projGroup = new THREE.Group();
		spawnsGroup = new THREE.Group();
		flagsGroup = new THREE.Group();
		scene.add(
			levelGroup,
			playersGroup,
			itemsGroup,
			vehiclesGroup,
			projGroup,
			spawnsGroup,
			flagsGroup
		);

		resize();
		resizeObs = new ResizeObserver(() => resize());
		resizeObs.observe(host);

		const loop = () => {
			raf = requestAnimationFrame(loop);
			controls?.update();
			if (renderer && scene && camera) renderer.render(scene, camera);
			if (labelRenderer && scene && camera) labelRenderer.render(scene, camera);
		};
		raf = requestAnimationFrame(loop);

		ready = true;
	});

	onDestroy(() => {
		cancelAnimationFrame(raf);
		resizeObs?.disconnect();
		controls?.dispose();
		for (const g of Object.values(geom)) g.dispose();
		for (const m of matCache.values()) m.dispose();
		disposeLevelChildren();
		renderer?.dispose();
		if (renderer?.domElement.parentNode)
			renderer.domElement.parentNode.removeChild(renderer.domElement);
		if (labelRenderer?.domElement.parentNode)
			labelRenderer.domElement.parentNode.removeChild(labelRenderer.domElement);
	});

	function resize() {
		if (!renderer || !labelRenderer || !camera || !host) return;
		const w = host.clientWidth || 1;
		const h = host.clientHeight || 1;
		renderer.setSize(w, h); // updateStyle=true so the canvas CSS size tracks the host
		labelRenderer.setSize(w, h);
		camera.aspect = w / h;
		camera.updateProjectionMatrix();
	}

	// --- Level geometry (BSP mesh or fallback box) -----------------------------
	function disposeLevelChildren() {
		if (!levelGroup) return;
		// Level geometries/materials are all created fresh in buildLevel (none
		// shared), so disposing on rebuild is safe and prevents GPU leaks.
		levelGroup.traverse((o) => {
			const obj = o as Partial<THREE.Mesh>;
			if (obj.geometry && typeof obj.geometry.dispose === 'function') obj.geometry.dispose();
			const m = obj.material;
			if (Array.isArray(m)) m.forEach((mm) => mm.dispose());
			else if (m && typeof (m as THREE.Material).dispose === 'function')
				(m as THREE.Material).dispose();
		});
	}

	function buildLevel() {
		if (!levelGroup) return;
		disposeLevelChildren();
		levelGroup.clear();

		if (mesh && mesh.positions.length >= 9 && mesh.indices.length >= 3) {
			// Real structure-BSP surfaces. Remap every vertex Halo→Three so it shares
			// the marker frame, then flat-shade with computed normals (the cache
			// carries positions + indices only).
			const g = new THREE.BufferGeometry();
			const src = mesh.positions;
			const pos = new Float32Array(src.length);
			for (let i = 0; i + 2 < src.length; i += 3) {
				const t = haloToThree({ x: src[i], y: src[i + 1], z: src[i + 2] });
				pos[i] = t[0];
				pos[i + 1] = t[1];
				pos[i + 2] = t[2];
			}
			g.setAttribute('position', new THREE.BufferAttribute(pos, 3));
			g.setIndex(mesh.indices.slice());
			g.computeVertexNormals();
			const surface = new THREE.Mesh(
				g,
				new THREE.MeshStandardMaterial({
					color: '#5a6472',
					roughness: 0.95,
					metalness: 0,
					flatShading: true,
					side: THREE.DoubleSide
				})
			);
			levelGroup.add(surface);
			// Faint wireframe so surface relief reads even under flat light.
			const wire = new THREE.LineSegments(
				new THREE.WireframeGeometry(g),
				new THREE.LineBasicMaterial({ color: 0x0b1018, transparent: true, opacity: 0.16 })
			);
			levelGroup.add(wire);
		} else {
			// Fallback: a wire box of the auto-fit world bounds + a ground grid so
			// markers still have spatial reference without the real mesh.
			const b = model.bounds;
			if (b.valid) {
				const c1 = haloToThree({ x: b.minX, y: b.minY, z: b.minZ });
				const c2 = haloToThree({ x: b.maxX, y: b.maxY, z: b.maxZ });
				const box = new THREE.Box3(
					new THREE.Vector3(Math.min(c1[0], c2[0]), Math.min(c1[1], c2[1]), Math.min(c1[2], c2[2])),
					new THREE.Vector3(Math.max(c1[0], c2[0]), Math.max(c1[1], c2[1]), Math.max(c1[2], c2[2]))
				);
				levelGroup.add(new THREE.Box3Helper(box, new THREE.Color(0x3a4860)));
			}
			const f = framingFromBounds(model.bounds);
			const groundY = b.valid ? b.minZ : 0; // Halo Z → Three Y
			const grid = new THREE.GridHelper(Math.max(40, f.radius * 2.2), 24, 0x2a3850, 0x18202e);
			grid.position.set(f.center[0], groundY, f.center[2]);
			levelGroup.add(grid);
		}
	}

	function meshBoundsForFraming() {
		if (mesh) return { ...mesh.bounds, valid: true, source: 'static' as const };
		return model.bounds;
	}

	function frameCamera(force = false) {
		if (!camera || !controls) return;
		if (!force && framedOnce) return;
		const f = framingFromBounds(meshBoundsForFraming());
		const p = defaultCameraPosition(f);
		camera.position.set(p[0], p[1], p[2]);
		controls.target.set(f.center[0], f.center[1], f.center[2]);
		controls.update();
		framedOnce = true;
	}

	/** Re-centre the camera on demand (wired to the page's Recenter button). */
	export function recenter() {
		framedOnce = false;
		frameCamera(true);
	}

	// --- Marker sync -----------------------------------------------------------
	function makeLabel(text: string): { obj: CSS2DObject; el: HTMLDivElement } {
		const el = document.createElement('div');
		el.className = 'viz3d-label';
		el.textContent = text;
		const obj = new CSS2DObject(el);
		obj.position.set(0, 1.7, 0);
		return { obj, el };
	}

	function syncPlayers() {
		if (!playersGroup) return;
		const seen = new Set<number>();
		for (const pl of model.players) {
			seen.add(pl.index);
			let node = playerNodes.get(pl.index);
			if (!node) {
				const group = new THREE.Group();
				const body = new THREE.Mesh(geom.capsule, mat(pl.color, { emissive: true }));
				const arrow = new THREE.Mesh(geom.arrow, mat('#f4f7fb'));
				// Cone points +Y by default; lay it flat (pointing +X) and offset it
				// forward; the group's Y-rotation then aims it along the heading.
				arrow.rotation.z = -Math.PI / 2;
				arrow.position.set(1.0, 0, 0);
				const { obj, el } = makeLabel(pl.name);
				group.add(body, arrow, obj);
				playersGroup.add(group);
				node = { group, body, arrow, label: obj, labelEl: el };
				playerNodes.set(pl.index, node);
			}
			const t = haloToThree(pl.pos);
			node.group.position.set(t[0], t[1] + 1.0, t[2]); // lift so the capsule sits on the floor
			// Pick a cached material by alive-opacity rather than mutating a shared one.
			node.body.material = mat(pl.color, { emissive: true, opacity: pl.alive ? 1 : 0.4 });
			node.body.scale.setScalar(pl.alive ? 1 : 0.85);

			const ang = facingAngle(pl.heading);
			if (ang != null && pl.alive) {
				node.arrow.visible = true;
				// World ground angle (CCW from +X) → rotation about Three's up (Y).
				// world +Y maps to −Z, so a CCW world turn is CW about Y → negate.
				node.group.rotation.y = -ang;
			} else {
				node.arrow.visible = false;
			}

			if (node.labelEl.textContent !== pl.name) node.labelEl.textContent = pl.name;
			node.label.visible = showNames;
			node.labelEl.style.color = pl.color;
			node.labelEl.style.opacity = pl.alive ? '1' : '0.5';
		}
		for (const [idx, node] of playerNodes) {
			if (!seen.has(idx)) {
				playersGroup.remove(node.group);
				playerNodes.delete(idx);
			}
		}
	}

	interface SimpleEntry {
		pos: { x: number; y: number; z: number };
		g: THREE.BufferGeometry;
		color: string;
		lift: number;
		emissive?: boolean;
		opacity?: number;
	}

	function rebuildSimple(group: THREE.Group | undefined, show: boolean, entries: SimpleEntry[]) {
		if (!group) return;
		group.clear(); // meshes only — geometry + material are shared, nothing to dispose
		group.visible = show;
		if (!show) return;
		for (const e of entries) {
			const m = new THREE.Mesh(e.g, mat(e.color, { emissive: e.emissive, opacity: e.opacity }));
			const t = haloToThree(e.pos);
			m.position.set(t[0], t[1] + e.lift, t[2]);
			group.add(m);
		}
	}

	function syncMarkers() {
		syncPlayers();
		rebuildSimple(
			itemsGroup,
			showItems,
			model.items
				.filter((i) => i.heldBy == null && !i.respawning)
				.map((i) => ({
					pos: i.pos,
					g: geom.item,
					color: ITEM_COLOR[i.kind] ?? ITEM_COLOR.other,
					lift: 0.8,
					emissive: true
				}))
		);
		rebuildSimple(
			vehiclesGroup,
			showVehicles,
			model.vehicles.map((v) => ({
				pos: v.pos,
				g: geom.vehicle,
				color: v.occupied ? '#e9edf2' : '#8b97a7',
				lift: 0.6
			}))
		);
		rebuildSimple(
			projGroup,
			showProjectiles,
			model.projectiles.map((p) => ({
				pos: p.pos,
				g: geom.proj,
				color: '#ffd27d',
				lift: 0.4,
				emissive: true
			}))
		);
		rebuildSimple(
			spawnsGroup,
			showSpawns,
			model.spawns.map((s) => ({
				pos: s.pos,
				g: geom.spawn,
				color: s.color,
				lift: 0.06,
				opacity: 0.5
			}))
		);
		rebuildSimple(
			flagsGroup,
			true,
			model.flags.map((f) => ({
				pos: f.pos,
				g: geom.item,
				color: f.color,
				lift: 1.0,
				emissive: true
			}))
		);
	}

	// Gate the (expensive) level rebuild on a key so it runs only when the level
	// actually changes — NOT every model tick. Real mesh → key by identity;
	// fallback box → key by validity + bounds rounded to 5wu so dynamic jitter
	// doesn't churn it.
	function levelKey(): string {
		if (mesh) return `mesh:${mesh.scenario}:${mesh.positions.length}`;
		const b = model.bounds;
		if (!b.valid) return 'box:none';
		const r = (n: number) => Math.round(n / 5) * 5;
		return `box:${b.source}:${r(b.minX)},${r(b.maxX)},${r(b.minY)},${r(b.maxY)},${r(b.minZ)},${r(b.maxZ)}`;
	}

	let lastLevelKey = '';
	let lastMeshKey = '';
	$effect(() => {
		if (!ready) return;
		const k = levelKey();
		if (k !== lastLevelKey) {
			lastLevelKey = k;
			buildLevel();
		}
		// Re-frame the camera when a real mesh first arrives, else on first valid box.
		const mk = mesh ? `${mesh.scenario}:${mesh.positions.length}` : '';
		if (mesh && mk !== lastMeshKey) {
			lastMeshKey = mk;
			frameCamera(true);
		} else if (!framedOnce && (mesh || model.bounds.valid)) {
			frameCamera(false);
		}
	});

	$effect(() => {
		if (!ready) return;
		// syncMarkers reads model + every show* prop, so this effect re-runs on any
		// feed tick or toggle change — no explicit dependency list needed.
		syncMarkers();
	});
</script>

<div class="scene" bind:this={host}>
	{#if webglFailed}
		<div class="webgl-fail">WebGL unavailable — the 3D scene can't render in this context.</div>
	{/if}
</div>

<style>
	.scene {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: hidden;
	}
	.scene :global(canvas) {
		display: block;
	}
	.scene :global(.viz3d-label) {
		font-family: 'Inter', system-ui, sans-serif;
		font-size: 0.72rem;
		font-weight: 600;
		letter-spacing: 0.01em;
		text-shadow:
			0 0 3px #000,
			0 1px 2px #000;
		white-space: nowrap;
		transform: translate(-50%, -100%);
		user-select: none;
	}
	.webgl-fail {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		text-align: center;
		padding: 2rem;
		color: #e3413f;
		font-size: 0.95rem;
	}
</style>
