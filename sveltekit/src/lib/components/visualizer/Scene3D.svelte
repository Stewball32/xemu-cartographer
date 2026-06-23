<script lang="ts">
	// Three.js render of a live VizModel inside the real structure-BSP mesh, with
	// the spectator-readability set layered on top of the base scene:
	//
	//   - EXTERIOR-SHELL CULL: the map's outer boundary (ceilings/roofs + perimeter
	//     walls — `shellTris`) is removed entirely so you see straight into the
	//     interior. The interior rooms stay SOLID + in their real texture-sampled
	//     material colours; only the selective fades below touch them.
	//   - OCCUPANCY REVEAL: interior rooms (BSP-cluster approximation from
	//     $lib/utils/rooms); a room fades when a player stands inside it.
	//   - CAMERA-OCCLUSION FADE: an interior room fades when it sits between the
	//     camera and a player (per-frame raycast), so interior walls never hide them.
	//   - THROUGH-WALL MARKERS: player markers draw always-on-top (depthTest off);
	//     a marker dims when a wall is between it and the camera, full in the open.
	//   - SHOW OUTER SHELL: optional toggle to bring the culled shell back (faint,
	//     for context).
	//
	// All data shaping + the coordinate remap + the clustering live in the pure
	// utils; this component does only the GPU wiring + the per-frame raycasts.
	/* eslint-disable svelte/no-dom-manipulating, svelte/prefer-svelte-reactivity */
	import { onMount, onDestroy } from 'svelte';
	import * as THREE from 'three';
	import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
	import { CSS2DRenderer, CSS2DObject } from 'three/examples/jsm/renderers/CSS2DRenderer.js';
	import type { VizModel } from '$lib/utils/visualizer-view';
	import type { BspMesh } from '$lib/utils/game-geometry';
	import { roomForPoint, type Room } from '$lib/utils/rooms';
	import {
		haloToThree,
		facingAngle,
		framingFromBounds,
		defaultCameraPosition
	} from '$lib/utils/viz3d';

	let {
		model,
		mesh = null,
		rooms = null,
		shellTris = null,
		occupancyReveal = true,
		occlusionFade = true,
		throughWall = true,
		showOuterShell = false,
		showItems = true,
		showVehicles = true,
		showProjectiles = true,
		showSpawns = false,
		showNames = true
	}: {
		model: VizModel;
		mesh?: BspMesh | null;
		/** Precomputed INTERIOR room clusters → independently-fadeable sub-meshes. */
		rooms?: Room[] | null;
		/** Triangle ordinals forming the exterior shell (culled by default). */
		shellTris?: number[] | null;
		occupancyReveal?: boolean;
		occlusionFade?: boolean;
		throughWall?: boolean;
		/** Bring the culled exterior shell back (faint, for context). */
		showOuterShell?: boolean;
		showItems?: boolean;
		showVehicles?: boolean;
		showProjectiles?: boolean;
		showSpawns?: boolean;
		showNames?: boolean;
	} = $props();

	let host: HTMLDivElement;
	let webglFailed = $state(false);
	let ready = $state(false);

	let renderer: THREE.WebGLRenderer | undefined;
	let labelRenderer: CSS2DRenderer | undefined;
	let scene: THREE.Scene | undefined;
	let camera: THREE.PerspectiveCamera | undefined;
	let controls: OrbitControls | undefined;
	let raf = 0;
	let resizeObs: ResizeObserver | undefined;
	const raycaster = new THREE.Raycaster();

	let levelGroup: THREE.Group | undefined;
	let playersGroup: THREE.Group | undefined;
	let itemsGroup: THREE.Group | undefined;
	let vehiclesGroup: THREE.Group | undefined;
	let projGroup: THREE.Group | undefined;
	let spawnsGroup: THREE.Group | undefined;
	let flagsGroup: THREE.Group | undefined;

	// Per-room level meshes (one independently-fadeable mesh per cluster).
	interface RoomMesh {
		mesh: THREE.Mesh;
		material: THREE.MeshStandardMaterial;
		roomIndex: number;
	}
	let roomMeshes: RoomMesh[] = [];
	// The culled exterior shell as one mesh, hidden by default (toggle to show).
	let shellMesh: THREE.Mesh | undefined;
	let shellMaterial: THREE.MeshStandardMaterial | undefined;

	// Occupancy/occlusion state, recomputed off the model tick + camera moves.
	const occupiedRooms = new Set<number>();
	const occludingRooms = new Set<number>();
	const playerOccluded = new Map<number, boolean>();
	let occlusionDirty = true;

	// Interior is SOLID + colored by default. The selective fades only DIM a room
	// (keep its colour) rather than ghost it to gray — so the interior reads in its
	// real material colours even where a player is revealed.
	const SOLID_OP = 1.0;
	const OCCUPIED_OP = 0.5;
	const OCCLUDE_OP = 0.45;
	const SHELL_SHOWN_OP = 0.1;

	interface PlayerNode {
		group: THREE.Group;
		body: THREE.Mesh;
		arrow: THREE.Mesh;
		label: CSS2DObject;
		labelEl: HTMLDivElement;
	}
	const playerNodes = new Map<number, PlayerNode>();

	const geom = {
		capsule: new THREE.CapsuleGeometry(0.55, 1.4, 6, 12),
		arrow: new THREE.ConeGeometry(0.32, 0.9, 10),
		item: new THREE.OctahedronGeometry(0.7),
		vehicle: new THREE.BoxGeometry(2.2, 1.1, 3.4),
		proj: new THREE.SphereGeometry(0.28, 8, 8),
		spawn: new THREE.CylinderGeometry(0.9, 0.9, 0.12, 16)
	};
	const matCache = new Map<string, THREE.MeshStandardMaterial>();
	function mat(
		hex: string,
		opts: { emissive?: boolean; opacity?: number; depthTest?: boolean } = {}
	): THREE.MeshStandardMaterial {
		const opacity = opts.opacity ?? 1;
		const depthTest = opts.depthTest ?? true;
		const key = `${hex}|${opts.emissive ? 1 : 0}|${opacity}|${depthTest ? 1 : 0}`;
		let m = matCache.get(key);
		if (!m) {
			const c = new THREE.Color(hex);
			m = new THREE.MeshStandardMaterial({
				color: c,
				emissive: opts.emissive ? c.clone().multiplyScalar(0.4) : new THREE.Color(0x000000),
				roughness: 0.55,
				metalness: 0,
				transparent: opacity < 1 || !depthTest,
				opacity,
				depthTest,
				depthWrite: depthTest
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
		controls.maxPolarAngle = Math.PI * 0.495;
		controls.addEventListener('change', () => (occlusionDirty = true));

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
			if (occlusionDirty) {
				recomputeOcclusion();
				applyPlayerOcclusionStyles();
				occlusionDirty = false;
			}
			applyRoomOpacities();
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
		renderer.setSize(w, h);
		labelRenderer.setSize(w, h);
		camera.aspect = w / h;
		camera.updateProjectionMatrix();
	}

	// --- Level geometry --------------------------------------------------------
	function disposeLevelChildren() {
		if (!levelGroup) return;
		levelGroup.traverse((o) => {
			const obj = o as Partial<THREE.Mesh>;
			if (obj.geometry && typeof obj.geometry.dispose === 'function') obj.geometry.dispose();
			const m = obj.material;
			if (Array.isArray(m)) m.forEach((mm) => mm.dispose());
			else if (m && typeof (m as THREE.Material).dispose === 'function')
				(m as THREE.Material).dispose();
		});
	}

	/** Build a Three BufferGeometry (Halo→Three remapped, flat-shaded) from a set
	 *  of triangle ordinals into the mesh, expanded to non-indexed so each room is
	 *  self-contained. */
	function roomGeometry(src: BspMesh, triOrdinals: number[]): THREE.BufferGeometry {
		const pos = new Float32Array(triOrdinals.length * 9);
		const hasColor = !!src.colors && src.colors.length === src.positions.length;
		const col = hasColor ? new Float32Array(triOrdinals.length * 9) : null;
		let w = 0;
		for (const t of triOrdinals) {
			for (let v = 0; v < 3; v++) {
				const vi = src.indices[t * 3 + v] * 3;
				const tt = haloToThree({
					x: src.positions[vi],
					y: src.positions[vi + 1],
					z: src.positions[vi + 2]
				});
				pos[w] = tt[0];
				pos[w + 1] = tt[1];
				pos[w + 2] = tt[2];
				if (col && src.colors) {
					col[w] = src.colors[vi];
					col[w + 1] = src.colors[vi + 1];
					col[w + 2] = src.colors[vi + 2];
				}
				w += 3;
			}
		}
		const g = new THREE.BufferGeometry();
		g.setAttribute('position', new THREE.BufferAttribute(pos, 3));
		if (col) g.setAttribute('color', new THREE.BufferAttribute(col, 3));
		g.computeVertexNormals();
		return g;
	}

	function buildLevel() {
		if (!levelGroup) return;
		disposeLevelChildren();
		levelGroup.clear();
		roomMeshes = [];
		shellMesh = undefined;
		shellMaterial = undefined;

		if (mesh && rooms && rooms.length > 0) {
			const hasColor = !!mesh.colors && mesh.colors.length === mesh.positions.length;
			// Interior room cluster meshes — SOLID + real material colours by default,
			// each independently fadeable for occupancy/occlusion.
			for (const room of rooms) {
				const g = roomGeometry(mesh, room.triIndices);
				const material = new THREE.MeshStandardMaterial({
					color: hasColor ? '#ffffff' : '#5a6472',
					vertexColors: hasColor,
					roughness: 0.92,
					metalness: 0,
					flatShading: true,
					side: THREE.DoubleSide,
					transparent: false,
					opacity: SOLID_OP
				});
				const m = new THREE.Mesh(g, material);
				m.userData.roomIndex = room.index;
				levelGroup.add(m);
				roomMeshes.push({ mesh: m, material, roomIndex: room.index });
			}
			// Exterior shell as ONE mesh — culled (hidden) by default; the toggle
			// brings it back faint for context. Built in the same material colours.
			if (shellTris && shellTris.length > 0) {
				const sg = roomGeometry(mesh, shellTris);
				shellMaterial = new THREE.MeshStandardMaterial({
					color: hasColor ? '#ffffff' : '#5a6472',
					vertexColors: hasColor,
					roughness: 0.95,
					metalness: 0,
					flatShading: true,
					side: THREE.DoubleSide,
					transparent: true,
					opacity: SHELL_SHOWN_OP,
					depthWrite: false
				});
				shellMesh = new THREE.Mesh(sg, shellMaterial);
				shellMesh.visible = showOuterShell;
				levelGroup.add(shellMesh);
			}
		} else if (mesh && mesh.positions.length >= 9 && mesh.indices.length >= 3) {
			// Single merged surface (mesh present but no room clustering).
			const src = mesh.positions;
			const pos = new Float32Array(src.length);
			for (let i = 0; i + 2 < src.length; i += 3) {
				const t = haloToThree({ x: src[i], y: src[i + 1], z: src[i + 2] });
				pos[i] = t[0];
				pos[i + 1] = t[1];
				pos[i + 2] = t[2];
			}
			const g = new THREE.BufferGeometry();
			g.setAttribute('position', new THREE.BufferAttribute(pos, 3));
			const hasColor = !!mesh.colors && mesh.colors.length === src.length;
			if (hasColor)
				g.setAttribute('color', new THREE.BufferAttribute(new Float32Array(mesh.colors!), 3));
			g.setIndex(mesh.indices.slice());
			g.computeVertexNormals();
			levelGroup.add(
				new THREE.Mesh(
					g,
					new THREE.MeshStandardMaterial({
						color: hasColor ? '#ffffff' : '#5a6472',
						vertexColors: hasColor,
						roughness: 0.95,
						metalness: 0,
						flatShading: true,
						side: THREE.DoubleSide
					})
				)
			);
		} else {
			// Fallback: wire box of the auto-fit world bounds + a ground grid.
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
			const groundY = b.valid ? b.minZ : 0;
			const grid = new THREE.GridHelper(Math.max(40, f.radius * 2.2), 24, 0x2a3850, 0x18202e);
			grid.position.set(f.center[0], groundY, f.center[2]);
			levelGroup.add(grid);
		}
		occlusionDirty = true;
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
		occlusionDirty = true;
	}

	export function recenter() {
		framedOnce = false;
		frameCamera(true);
	}

	/** Fixed high overview camera — the use case for the outer-shell toggle. */
	export function overview() {
		if (!camera || !controls) return;
		const f = framingFromBounds(meshBoundsForFraming());
		camera.position.set(
			f.center[0] + f.radius * 0.15,
			f.center[1] + f.radius * 2.6,
			f.center[2] + f.radius * 0.15
		);
		controls.target.set(f.center[0], f.center[1], f.center[2]);
		controls.update();
		occlusionDirty = true;
	}

	// --- Occupancy + occlusion -------------------------------------------------
	function recomputeOcclusion() {
		occupiedRooms.clear();
		occludingRooms.clear();
		playerOccluded.clear();
		if (!rooms || rooms.length === 0 || !camera) return;

		// Occupancy: which room each live player sits in (Halo coords).
		for (const pl of model.players) {
			if (!pl.alive) continue;
			const r = roomForPoint(rooms, pl.pos);
			if (r >= 0) occupiedRooms.add(r);
		}

		// Camera-occlusion + per-player blocked test (Three coords).
		const meshes = roomMeshes.map((rm) => rm.mesh);
		if (meshes.length === 0) return;
		const camPos = camera.position;
		for (const [idx, node] of playerNodes) {
			const target = node.group.position;
			const dir = new THREE.Vector3().subVectors(target, camPos);
			const dist = dir.length();
			if (dist < 1e-3) {
				playerOccluded.set(idx, false);
				continue;
			}
			dir.normalize();
			raycaster.set(camPos, dir);
			raycaster.near = 0.5;
			raycaster.far = Math.max(0.6, dist - 1.0); // stop short of the player's own floor
			const hits = raycaster.intersectObjects(meshes, false);
			const blocked = hits.length > 0;
			// Selective: fade ONLY the nearest occluder room (the wall directly in
			// front of the player), not every room the ray passes through — keeps the
			// rest of the interior solid + colored.
			if (blocked) {
				const ri = (hits[0].object.userData as { roomIndex?: number }).roomIndex;
				if (typeof ri === 'number') occludingRooms.add(ri);
			}
			playerOccluded.set(idx, blocked);
		}
	}

	function applyRoomOpacities() {
		for (const rm of roomMeshes) {
			// Interior stays SOLID unless this room is occupied or is occluding the
			// camera→player line — those are the only selective fades.
			let target = SOLID_OP;
			if (occupancyReveal && occupiedRooms.has(rm.roomIndex))
				target = Math.min(target, OCCUPIED_OP);
			if (occlusionFade && occludingRooms.has(rm.roomIndex)) target = Math.min(target, OCCLUDE_OP);
			const cur = rm.material.opacity;
			const next = cur + (target - cur) * 0.18;
			rm.material.opacity = Math.abs(next - target) < 0.005 ? target : next;
			// Opaque (vivid colour) when solid; transparent only while faded.
			rm.material.transparent = rm.material.opacity < 0.99;
			rm.material.depthWrite = rm.material.opacity > 0.85;
		}
	}

	function applyPlayerOcclusionStyles() {
		for (const [idx, node] of playerNodes) {
			const pl = model.players.find((q) => q.index === idx);
			if (!pl) continue;
			const occluded = throughWall && (playerOccluded.get(idx) ?? false);
			const opacity = pl.alive ? (occluded ? 0.5 : 1) : 0.4;
			node.body.material = mat(pl.color, { emissive: true, opacity, depthTest: !throughWall });
			node.body.renderOrder = throughWall ? 20 : 0;
			node.arrow.material = mat('#f4f7fb', { opacity, depthTest: !throughWall });
			node.arrow.renderOrder = throughWall ? 20 : 0;
			node.labelEl.style.opacity = pl.alive ? (occluded ? '0.7' : '1') : '0.5';
		}
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
				arrow.rotation.z = -Math.PI / 2;
				arrow.position.set(1.0, 0, 0);
				const { obj, el } = makeLabel(pl.name);
				group.add(body, arrow, obj);
				playersGroup.add(group);
				node = { group, body, arrow, label: obj, labelEl: el };
				playerNodes.set(pl.index, node);
			}
			const t = haloToThree(pl.pos);
			node.group.position.set(t[0], t[1] + 1.0, t[2]);
			node.body.scale.setScalar(pl.alive ? 1 : 0.85);

			const ang = facingAngle(pl.heading);
			if (ang != null && pl.alive) {
				node.arrow.visible = true;
				node.group.rotation.y = -ang;
			} else {
				node.arrow.visible = false;
			}

			if (node.labelEl.textContent !== pl.name) node.labelEl.textContent = pl.name;
			node.label.visible = showNames;
			node.labelEl.style.color = pl.color;
		}
		for (const [idx, node] of playerNodes) {
			if (!seen.has(idx)) {
				playersGroup.remove(node.group);
				playerNodes.delete(idx);
			}
		}
		// Occlusion styling (opacity/depthTest/renderOrder) is applied separately so
		// it also updates on camera moves; trigger a recompute now that nodes moved.
		occlusionDirty = true;
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
		group.clear();
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
			model.projectiles.map((pp) => ({
				pos: pp.pos,
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
	// actually changes — NOT every model tick.
	function levelKey(): string {
		if (mesh && rooms && rooms.length > 0)
			return `rooms:${mesh.scenario}:${mesh.positions.length}:${rooms.length}`;
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
		// syncMarkers reads model + every show* prop, so this re-runs on any tick or
		// toggle change; it sets occlusionDirty so styling re-applies in the loop.
		syncMarkers();
	});

	// Re-apply occlusion styling immediately when a readability toggle flips.
	$effect(() => {
		void occupancyReveal;
		void occlusionFade;
		void throughWall;
		occlusionDirty = true;
	});

	// Show/hide the culled exterior shell on toggle (no rebuild needed).
	$effect(() => {
		if (shellMesh) shellMesh.visible = showOuterShell;
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
