# Milestone 12 — POV marker overlay (stretch)

> Long-shot. Browser source that overlays directly on top of a machine's actual game POV (e.g. as an OBS Browser Source layered above the kiosk capture), drawing world-anchored markers in real time: enemy silhouettes through walls, powerup tags, teammate gamertags floating above their heads. Requires perspective projection (3D world → 2D screen) instead of M11's top-down projection — same input data, harder math.

## 12a. Camera + projection model

Halo: CE per-player camera state (FOV, position, view direction) is already partially read by the scraper. Audit what's there; spec any missing offsets (camera matrix, near/far planes) for an M19 follow-up if needed. Build a per-tick "render frustum" model in the overlay client.

## 12b. Single-player POV alignment

New overlay at `/overlays/pov/<machine>/`. For full-screen single-player on the target machine: project enemy world positions through the player's camera matrix to screen coordinates. Render markers (silhouettes, name tags, distance indicators) as positioned absolute elements. Validate alignment by overlaying onto the actual kiosk video stream — markers should track tightly as the player turns.

## 12c. Split-screen handling

When the target machine is running 2/3/4-player split-screen, partition the screen into the appropriate viewports and project per-viewport per-player. The M5 reader already exposes local-player count + indices.

## 12d. Marker types

Initial set:

- Enemy silhouettes (filled shape outlining the enemy's bounding box, with team color).
- Teammate gamertag floats above-head.
- Powerup labels at spawn positions (with active/respawn timer if available).
- Optional: line-to-objective in CTF (toward the enemy flag if you're attacking, toward your base if you have it).

## 12e. Composite verification

OBS scene = kiosk video source layered with the POV overlay browser source above it. Test rig: known-good viewing angle on a known map → measure pixel offset between marker and actual entity at multiple angles. If offsets are consistent, calibration matrix is correct; if they drift, the camera offsets are off and feed back to 12a as offset bugs.

Smoke test: 1v1 Slayer on Wizard, single full-screen → enemy silhouette tracks the opponent through walls; teammate-tag-above-head test in a 2v2 game on Hang 'Em High. Split-screen verification: same setup with 2v2 on a single console.

**Stretch flag.** This milestone is explicitly stretch — if M11 reveals that the projection math is brittle, defer M12 to M21+ open bucket and ship M11 alone. Also explicitly out of scope for v1: through-wall occlusion (rendering markers dimmed when behind geometry), since that requires BSP knowledge from M11a's deferred case.
