<script lang="ts">
	// Three.js editor canvas for the BSP spectator-mesh editor. Renders the raw
	// structure-BSP mesh and lets the operator SEE + SELECT triangles to cull:
	//
	//   - KEPT mesh        — the surviving geometry in its real material colours.
	//   - REMOVED ghost    — culled triangles, faint red, toggleable (so you can
	//                        re-select + restore them; nothing is destructive).
	//   - SELECTION        — current selection, cyan, drawn on top.
	//   - PLANE PREVIEW     — what the cull-height plane would remove, orange.
	//   - CULL PLANE        — a draggable horizontal plane at world Z = cullZ.
	//   - FLOOR MARKERS     — walkable-floor reference dots (player-accessible
	//                        height) so you can see where players stand while cutting.
	//
	// All the data shaping + selection math lives in the pure $lib/utils/bsp-edit;
	// this component does only the GPU wiring, picking raycasts, the box-drag
	// rectangle, and the plane drag. Two cameras: an orthographic TOP-DOWN (the true
	// 2D-equivalent footprint) and a perspective ORBIT, toggled by `viewMode`.
	/* eslint-disable svelte/no-dom-manipulating */
	import { onMount, onDestroy } from 'svelte';
	import * as THREE from 'three';
	import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
	import type { BspMesh } from '$lib/utils/game-geometry';
	import type { TriMeta, PickMods } from '$lib/utils/bsp-edit';
	import { boxSelect, type ScreenRect } from '$lib/utils/bsp-edit';
	import { haloToThree, framingFromBounds, defaultCameraPosition } from '$lib/utils/viz3d';

	interface ColoredMarker {
		x: number;
		y: number;
		z: number;
		color: string;
	}

	let {
		mesh,
		meta,
		removed,
		selected,
		previewTris = null,
		floorMarkers = [],
		viewMode = 'orbit',
		boxMode = false,
		cullZ,
		showPlane = true,
		showRemoved = false,
		showFloors = true,
		showColors = true,
		onPick,
		onBoxSelect,
		onCullZChange
	}: {
		mesh: BspMesh;
		meta: TriMeta[];
		/** Per-triangle removed flags; identity changes on every edit (reactive). */
		removed: Uint8Array;
		/** Currently-selected triangle ordinals; identity changes on selection edits. */
		selected: Set<number>;
		/** Triangles the cull-height plane would remove (orange preview), or null. */
		previewTris?: number[] | null;
		floorMarkers?: ColoredMarker[];
		viewMode?: 'top' | 'orbit';
		/** Box-select tool active → left-drag draws a rectangle instead of orbiting. */
		boxMode?: boolean;
		cullZ: number;
		showPlane?: boolean;
		showRemoved?: boolean;
		showFloors?: boolean;
		showColors?: boolean;
		onPick: (tri: number | null, mods: PickMods) => void;
		onBoxSelect: (tris: number[], mods: PickMods) => void;
		onCullZChange: (z: number) => void;
	} = $props();

	let host: HTMLDivElement;
	let webglFailed = $state(false);
	let ready = $state(false);

	// Box-drag rectangle overlay (screen px relative to the host).
	let boxRect = $state<{ x: number; y: number; w: number; h: number } | null>(null);

	let renderer: THREE.WebGLRenderer | undefined;
	let scene: THREE.Scene | undefined;
	let perspCam: THREE.PerspectiveCamera | undefined;
	let orthoCam: THREE.OrthographicCamera | undefined;
	let controls: OrbitControls | undefined;
	let raf = 0;
	let resizeObs: ResizeObserver | undefined;
	const raycaster = new THREE.Raycaster();
	raycaster.params.Points = { threshold: 0.4 };

	// Shared vertex buffers (positions/normals/colours) — built once per mesh; the
	// four overlay meshes differ ONLY by their index buffer.
	let posAttr: THREE.BufferAttribute | undefined;
	let normAttr: THREE.BufferAttribute | undefined;
	let colAttr: THREE.BufferAttribute | null = null;

	let keptMesh: THREE.Mesh | undefined;
	let removedMesh: THREE.Mesh | undefined;
	let selMesh: THREE.Mesh | undefined;
	let previewMesh: THREE.Mesh | undefined;
	let planeMesh: THREE.Mesh | undefined;
	let floorPoints: THREE.Points | undefined;
	let levelGroup: THREE.Group | undefined;

	// faceIndex (into a drawn mesh) → original triangle ordinal.
	let keptList: number[] = [];
	let removedList: number[] = [];

	const materials = {
		kept: undefined as THREE.MeshStandardMaterial | undefined,
		removed: new THREE.MeshBasicMaterial({
			color: 0xe3413f,
			transparent: true,
			opacity: 0.16,
			side: THREE.DoubleSide,
			depthWrite: false
		}),
		sel: new THREE.MeshBasicMaterial({
			color: 0x35d2ff,
			transparent: true,
			opacity: 0.55,
			side: THREE.DoubleSide,
			depthWrite: false,
			depthTest: true
		}),
		preview: new THREE.MeshBasicMaterial({
			color: 0xffa033,
			transparent: true,
			opacity: 0.42,
			side: THREE.DoubleSide,
			depthWrite: false
		}),
		plane: new THREE.MeshBasicMaterial({
			color: 0x5cc8ff,
			transparent: true,
			opacity: 0.14,
			side: THREE.DoubleSide,
			depthWrite: false
		})
	};

	function activeCamera(): THREE.Camera | undefined {
		return viewMode === 'top' ? orthoCam : perspCam;
	}

	// --- Build the shared geometry from the mesh -------------------------------
	function buildSharedBuffers() {
		const pos = mesh.positions;
		const idx = mesh.indices;
		const T = Math.floor(idx.length / 3);
		const hasColor = showColors && !!mesh.colors && mesh.colors.length === pos.length;
		const col = hasColor ? (mesh.colors as number[]) : undefined;

		const positions9 = new Float32Array(T * 9);
		const normals9 = new Float32Array(T * 9);
		const colors9 = col ? new Float32Array(T * 9) : null;

		const ax = new THREE.Vector3();
		const bx = new THREE.Vector3();
		const cx = new THREE.Vector3();
		const ab = new THREE.Vector3();
		const acv = new THREE.Vector3();
		const nrm = new THREE.Vector3();

		for (let t = 0; t < T; t++) {
			const ia = idx[t * 3] * 3;
			const ib = idx[t * 3 + 1] * 3;
			const ic = idx[t * 3 + 2] * 3;
			const a = haloToThree({ x: pos[ia], y: pos[ia + 1], z: pos[ia + 2] });
			const b = haloToThree({ x: pos[ib], y: pos[ib + 1], z: pos[ib + 2] });
			const c = haloToThree({ x: pos[ic], y: pos[ic + 1], z: pos[ic + 2] });
			ax.set(a[0], a[1], a[2]);
			bx.set(b[0], b[1], b[2]);
			cx.set(c[0], c[1], c[2]);
			ab.subVectors(bx, ax);
			acv.subVectors(cx, ax);
			nrm.crossVectors(ab, acv).normalize();
			const o = t * 9;
			positions9[o] = a[0];
			positions9[o + 1] = a[1];
			positions9[o + 2] = a[2];
			positions9[o + 3] = b[0];
			positions9[o + 4] = b[1];
			positions9[o + 5] = b[2];
			positions9[o + 6] = c[0];
			positions9[o + 7] = c[1];
			positions9[o + 8] = c[2];
			for (let v = 0; v < 3; v++) {
				normals9[o + v * 3] = nrm.x;
				normals9[o + v * 3 + 1] = nrm.y;
				normals9[o + v * 3 + 2] = nrm.z;
			}
			if (colors9 && col) {
				colors9[o] = col[ia];
				colors9[o + 1] = col[ia + 1];
				colors9[o + 2] = col[ia + 2];
				colors9[o + 3] = col[ib];
				colors9[o + 4] = col[ib + 1];
				colors9[o + 5] = col[ib + 2];
				colors9[o + 6] = col[ic];
				colors9[o + 7] = col[ic + 1];
				colors9[o + 8] = col[ic + 2];
			}
		}

		posAttr = new THREE.BufferAttribute(positions9, 3);
		normAttr = new THREE.BufferAttribute(normals9, 3);
		colAttr = colors9 ? new THREE.BufferAttribute(colors9, 3) : null;

		materials.kept?.dispose();
		materials.kept = new THREE.MeshStandardMaterial({
			color: colAttr ? 0xffffff : 0x5a6472,
			vertexColors: !!colAttr,
			roughness: 0.92,
			metalness: 0,
			flatShading: true,
			side: THREE.DoubleSide
		});
	}

	function makeGeo(): THREE.BufferGeometry {
		const g = new THREE.BufferGeometry();
		if (posAttr) g.setAttribute('position', posAttr);
		if (normAttr) g.setAttribute('normal', normAttr);
		if (colAttr) g.setAttribute('color', colAttr);
		return g;
	}

	function buildLevel() {
		if (!scene) return;
		if (levelGroup) {
			scene.remove(levelGroup);
			levelGroup.traverse((o) => {
				const m = o as Partial<THREE.Mesh>;
				if (m.geometry && typeof m.geometry.dispose === 'function') m.geometry.dispose();
			});
		}
		buildSharedBuffers();
		levelGroup = new THREE.Group();

		keptMesh = new THREE.Mesh(makeGeo(), materials.kept);
		removedMesh = new THREE.Mesh(makeGeo(), materials.removed);
		removedMesh.renderOrder = 1;
		selMesh = new THREE.Mesh(makeGeo(), materials.sel);
		selMesh.renderOrder = 3;
		previewMesh = new THREE.Mesh(makeGeo(), materials.preview);
		previewMesh.renderOrder = 2;

		// Draggable cull-height plane, sized to the map footprint + slack.
		const spanX = mesh.bounds.maxX - mesh.bounds.minX;
		const spanY = mesh.bounds.maxY - mesh.bounds.minY;
		const planeGeo = new THREE.PlaneGeometry(spanX * 1.5 + 2, spanY * 1.5 + 2);
		planeMesh = new THREE.Mesh(planeGeo, materials.plane);
		planeMesh.rotation.x = -Math.PI / 2; // lie flat (XZ), height set via cullZ
		const cxw = (mesh.bounds.minX + mesh.bounds.maxX) / 2;
		const cyw = (mesh.bounds.minY + mesh.bounds.maxY) / 2;
		const cTop = haloToThree({ x: cxw, y: cyw, z: 0 });
		planeMesh.position.set(cTop[0], cullZ, cTop[2]);

		// Floor reference dots (player-accessible height cue).
		const fp = new Float32Array(floorMarkers.length * 3);
		const fc = new Float32Array(floorMarkers.length * 3);
		const tmpC = new THREE.Color();
		for (let i = 0; i < floorMarkers.length; i++) {
			const m = floorMarkers[i];
			const t = haloToThree({ x: m.x, y: m.y, z: m.z });
			fp[i * 3] = t[0];
			fp[i * 3 + 1] = t[1] + 0.05;
			fp[i * 3 + 2] = t[2];
			tmpC.set(m.color);
			fc[i * 3] = tmpC.r;
			fc[i * 3 + 1] = tmpC.g;
			fc[i * 3 + 2] = tmpC.b;
		}
		const fgeo = new THREE.BufferGeometry();
		fgeo.setAttribute('position', new THREE.BufferAttribute(fp, 3));
		fgeo.setAttribute('color', new THREE.BufferAttribute(fc, 3));
		floorPoints = new THREE.Points(
			fgeo,
			new THREE.PointsMaterial({ size: 0.28, vertexColors: true, sizeAttenuation: true })
		);

		levelGroup.add(keptMesh, removedMesh, selMesh, previewMesh, planeMesh, floorPoints);
		scene.add(levelGroup);

		refreshIndices();
		applyToggles();
	}

	// --- Index buffers (cheap; rebuilt on removed/selection/preview change) -----
	function setIndex(m: THREE.Mesh | undefined, tris: number[]) {
		if (!m) return;
		const arr = new Uint32Array(tris.length * 3);
		for (let i = 0; i < tris.length; i++) {
			arr[i * 3] = tris[i] * 3;
			arr[i * 3 + 1] = tris[i] * 3 + 1;
			arr[i * 3 + 2] = tris[i] * 3 + 2;
		}
		m.geometry.setIndex(new THREE.BufferAttribute(arr, 1));
		m.geometry.setDrawRange(0, arr.length);
	}

	function refreshKeptRemoved() {
		const T = meta.length;
		keptList = [];
		removedList = [];
		for (let t = 0; t < T; t++) {
			if (removed[t]) removedList.push(t);
			else keptList.push(t);
		}
		setIndex(keptMesh, keptList);
		setIndex(removedMesh, removedList);
	}

	function refreshSelection() {
		setIndex(selMesh, [...selected]);
	}

	function refreshPreview() {
		// Only preview triangles that aren't already removed (no double-paint).
		const tris = (previewTris ?? []).filter((t) => !removed[t]);
		setIndex(previewMesh, tris);
	}

	function refreshIndices() {
		refreshKeptRemoved();
		refreshSelection();
		refreshPreview();
	}

	function applyToggles() {
		if (removedMesh) removedMesh.visible = showRemoved;
		if (planeMesh) planeMesh.visible = showPlane;
		if (floorPoints) floorPoints.visible = showFloors;
		if (previewMesh) previewMesh.visible = showPlane && (previewTris?.length ?? 0) > 0;
	}

	// --- Cameras + framing ------------------------------------------------------
	function frame() {
		const f = framingFromBounds({ ...mesh.bounds, valid: true, source: 'static' });
		if (perspCam) {
			const p = defaultCameraPosition(f, 1.7);
			perspCam.position.set(p[0], p[1], p[2]);
		}
		if (orthoCam) {
			orthoCam.position.set(f.center[0], f.center[1] + f.radius * 4, f.center[2]);
			orthoCam.up.set(0, 0, -1);
			orthoCam.lookAt(f.center[0], f.center[1], f.center[2]);
			updateOrthoFrustum(f.radius);
		}
		if (controls) {
			controls.target.set(f.center[0], f.center[1], f.center[2]);
			controls.update();
		}
	}

	function updateOrthoFrustum(radius: number) {
		if (!orthoCam || !host) return;
		const aspect = (host.clientWidth || 1) / (host.clientHeight || 1);
		const r = radius * 1.15;
		orthoCam.left = -r * aspect;
		orthoCam.right = r * aspect;
		orthoCam.top = r;
		orthoCam.bottom = -r;
		orthoCam.near = -radius * 10;
		orthoCam.far = radius * 20;
		orthoCam.updateProjectionMatrix();
	}

	function rebindControls() {
		const cam = activeCamera();
		if (!cam || !renderer) return;
		const target = controls ? controls.target.clone() : new THREE.Vector3();
		controls?.dispose();
		controls = new OrbitControls(cam, renderer.domElement);
		controls.enableDamping = true;
		controls.dampingFactor = 0.1;
		controls.target.copy(target);
		if (viewMode === 'top') {
			controls.enableRotate = false;
			controls.maxPolarAngle = 0;
		}
		controls.enabled = !boxMode;
		controls.update();
	}

	export function recenter() {
		frame();
	}

	// --- Pointer interaction (pick / box / plane-drag) -------------------------
	const ndc = new THREE.Vector2();
	const dragPlane = new THREE.Plane();
	const hitPt = new THREE.Vector3();
	let downX = 0;
	let downY = 0;
	let downTime = 0;
	let planeDragging = false;

	function toNdc(e: PointerEvent) {
		const rect = renderer!.domElement.getBoundingClientRect();
		ndc.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
		ndc.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
		return rect;
	}

	function onPointerDown(e: PointerEvent) {
		if (!renderer || e.button !== 0) return;
		const rect = toNdc(e);
		downX = e.clientX - rect.left;
		downY = e.clientY - rect.top;
		downTime = e.timeStamp;

		// Plane drag: grab the cull plane ONLY where it's clicked in the open (no
		// geometry behind the cursor) — so clicking the building always SELECTS it,
		// and the plane's open skirt is its drag handle. Slider is the precise control.
		if (showPlane && !boxMode && planeMesh && viewMode === 'orbit') {
			const cam = activeCamera()!;
			raycaster.setFromCamera(ndc, cam);
			const planeHit = raycaster.intersectObject(planeMesh, false)[0];
			const geoHit = keptMesh ? raycaster.intersectObject(keptMesh, false)[0] : undefined;
			if (planeHit && !geoHit) {
				planeDragging = true;
				if (controls) controls.enabled = false;
				const dir = new THREE.Vector3();
				cam.getWorldDirection(dir);
				dir.y = 0;
				if (dir.lengthSq() < 1e-6) dir.set(0, 0, 1);
				dir.normalize();
				dragPlane.setFromNormalAndCoplanarPoint(dir, planeHit.point);
				return;
			}
		}

		if (boxMode) {
			boxRect = { x: downX, y: downY, w: 0, h: 0 };
		}
	}

	function onPointerMove(e: PointerEvent) {
		if (!renderer) return;
		const rect = toNdc(e);

		if (planeDragging) {
			const cam = activeCamera()!;
			raycaster.setFromCamera(ndc, cam);
			if (raycaster.ray.intersectPlane(dragPlane, hitPt)) {
				const z = Math.max(mesh.bounds.minZ, Math.min(mesh.bounds.maxZ, hitPt.y));
				onCullZChange(z);
			}
			return;
		}

		if (boxMode && boxRect) {
			const x = e.clientX - rect.left;
			const y = e.clientY - rect.top;
			boxRect = {
				x: Math.min(downX, x),
				y: Math.min(downY, y),
				w: Math.abs(x - downX),
				h: Math.abs(y - downY)
			};
		}
	}

	function onPointerUp(e: PointerEvent) {
		if (!renderer) return;
		const mods: PickMods = { shift: e.shiftKey, alt: e.altKey };

		if (planeDragging) {
			planeDragging = false;
			if (controls) controls.enabled = !boxMode;
			return;
		}

		if (boxMode && boxRect) {
			const rect = boxRect;
			boxRect = null;
			if (rect.w < 3 && rect.h < 3) {
				// Treated as a click in box mode → single pick.
				doPick(e, mods);
				return;
			}
			const screen: ScreenRect = {
				minX: rect.x,
				minY: rect.y,
				maxX: rect.x + rect.w,
				maxY: rect.y + rect.h
			};
			const tris = boxSelect(meta, projectCentroid, screen);
			onBoxSelect(tris, mods);
			return;
		}

		// Select mode: a click (negligible movement) picks; a drag was an orbit.
		const rect = renderer.domElement.getBoundingClientRect();
		const dx = e.clientX - rect.left - downX;
		const dy = e.clientY - rect.top - downY;
		if (dx * dx + dy * dy <= 25 && e.timeStamp - downTime < 500) {
			doPick(e, mods);
		}
	}

	function doPick(e: PointerEvent, mods: PickMods) {
		toNdc(e);
		const cam = activeCamera();
		if (!cam) return;
		raycaster.setFromCamera(ndc, cam);
		const targets: THREE.Object3D[] = [];
		if (keptMesh) targets.push(keptMesh);
		if (showRemoved && removedMesh) targets.push(removedMesh);
		const hits = raycaster.intersectObjects(targets, false);
		if (hits.length === 0) {
			onPick(null, mods);
			return;
		}
		const hit = hits[0];
		const face = hit.faceIndex ?? -1;
		const list = hit.object === removedMesh ? removedList : keptList;
		const tri = face >= 0 && face < list.length ? list[face] : -1;
		onPick(tri >= 0 ? tri : null, mods);
	}

	/** World (Halo coords) centroid → host-relative screen px, or null if behind. */
	function projectCentroid(cx: number, cy: number, cz: number): [number, number] | null {
		const cam = activeCamera();
		if (!cam || !host) return null;
		const t = haloToThree({ x: cx, y: cy, z: cz });
		const v = new THREE.Vector3(t[0], t[1], t[2]).project(cam);
		if (v.z < -1 || v.z > 1) return null;
		const x = (v.x * 0.5 + 0.5) * host.clientWidth;
		const y = (-v.y * 0.5 + 0.5) * host.clientHeight;
		return [x, y];
	}

	// --- Lifecycle --------------------------------------------------------------
	onMount(() => {
		try {
			renderer = new THREE.WebGLRenderer({ antialias: true });
		} catch {
			webglFailed = true;
			return;
		}
		renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
		host.appendChild(renderer.domElement);
		renderer.domElement.style.display = 'block';
		renderer.domElement.style.touchAction = 'none';

		scene = new THREE.Scene();
		scene.background = new THREE.Color('#0a0e16');

		perspCam = new THREE.PerspectiveCamera(55, 1, 0.05, 6000);
		orthoCam = new THREE.OrthographicCamera(-10, 10, 10, -10, -1000, 1000);

		scene.add(new THREE.HemisphereLight(0xbfd4ff, 0x2a2418, 1.1));
		const key = new THREE.DirectionalLight(0xffffff, 1.2);
		key.position.set(60, 140, 40);
		scene.add(key);
		const rim = new THREE.DirectionalLight(0x88aaff, 0.4);
		rim.position.set(-60, 80, -70);
		scene.add(rim);

		buildLevel();
		rebindControls();
		frame();
		resize();
		resizeObs = new ResizeObserver(() => resize());
		resizeObs.observe(host);

		const el = renderer.domElement;
		el.addEventListener('pointerdown', onPointerDown);
		el.addEventListener('pointermove', onPointerMove);
		window.addEventListener('pointerup', onPointerUp);

		const loop = () => {
			raf = requestAnimationFrame(loop);
			controls?.update();
			const cam = activeCamera();
			if (renderer && scene && cam) renderer.render(scene, cam);
		};
		raf = requestAnimationFrame(loop);
		ready = true;
	});

	onDestroy(() => {
		cancelAnimationFrame(raf);
		resizeObs?.disconnect();
		controls?.dispose();
		if (renderer) {
			const el = renderer.domElement;
			el.removeEventListener('pointerdown', onPointerDown);
			el.removeEventListener('pointermove', onPointerMove);
			window.removeEventListener('pointerup', onPointerUp);
		}
		for (const m of Object.values(materials)) m?.dispose();
		levelGroup?.traverse((o) => {
			const m = o as Partial<THREE.Mesh & THREE.Points>;
			if (m.geometry && typeof m.geometry.dispose === 'function') m.geometry.dispose();
		});
		renderer?.dispose();
		if (renderer?.domElement.parentNode)
			renderer.domElement.parentNode.removeChild(renderer.domElement);
	});

	function resize() {
		if (!renderer || !host) return;
		const w = host.clientWidth || 1;
		const h = host.clientHeight || 1;
		renderer.setSize(w, h);
		if (perspCam) {
			perspCam.aspect = w / h;
			perspCam.updateProjectionMatrix();
		}
		const f = framingFromBounds({ ...mesh.bounds, valid: true, source: 'static' });
		updateOrthoFrustum(f.radius);
	}

	// --- Reactivity: rebuild on mesh change, re-index on edit, retoggle ---------
	let builtKey = '';
	$effect(() => {
		if (!ready) return;
		const key = `${mesh.scenario}:${mesh.positions.length}:${showColors ? 1 : 0}:${floorMarkers.length}`;
		if (key !== builtKey) {
			builtKey = key;
			buildLevel();
			frame();
		}
	});

	// Re-index when the edit state (removed / selection / preview) identity changes.
	$effect(() => {
		void removed;
		if (ready && keptMesh) refreshKeptRemoved();
	});
	$effect(() => {
		void selected;
		if (ready && selMesh) refreshSelection();
	});
	$effect(() => {
		void previewTris;
		if (ready && previewMesh) {
			refreshPreview();
			applyToggles();
		}
	});

	// Move the cull plane when cullZ changes (no rebuild).
	$effect(() => {
		if (planeMesh) planeMesh.position.y = cullZ;
	});

	// Toggle visibility without rebuilding.
	$effect(() => {
		void showRemoved;
		void showPlane;
		void showFloors;
		if (ready) applyToggles();
	});

	// Swap camera / box-mode → rebind controls.
	$effect(() => {
		void viewMode;
		void boxMode;
		if (ready) {
			rebindControls();
			frame();
		}
	});
</script>

<div class="scene" bind:this={host}>
	{#if webglFailed}
		<div class="webgl-fail">WebGL unavailable — the editor can't render in this context.</div>
	{/if}
	{#if boxRect}
		<div
			class="boxsel"
			style="left:{boxRect.x}px; top:{boxRect.y}px; width:{boxRect.w}px; height:{boxRect.h}px;"
		></div>
	{/if}
</div>

<style>
	.scene {
		position: relative;
		width: 100%;
		height: 100%;
		overflow: hidden;
		cursor: crosshair;
	}
	.boxsel {
		position: absolute;
		border: 1px solid #35d2ff;
		background: rgba(53, 210, 255, 0.12);
		pointer-events: none;
		z-index: 5;
	}
	.webgl-fail {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		text-align: center;
		padding: 2rem;
		color: #e3413f;
	}
</style>
