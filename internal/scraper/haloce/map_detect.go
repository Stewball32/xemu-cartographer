package haloce

import "strings"

// resolveMapName decides the reported current-map identity from the three
// signals the 2026-06-21 xemu runtime pass characterized (see halo-offset-mapper
// docs/RUNTIME-PASS-2026-06-21-CE.md, Task 7):
//
//   - scenarioTag — the scenario tag path resolved from the cache tag header
//     (AddrTagHeaderPtr 0x2DF1C4 → scenario_tag_id&0xFFFF → tag_array[idx] +
//     OffTagNamePtr). This is the RELIABLE map identifier in the front-end menu
//     ("levels\ui\uitest"), singleplayer, AND MP.
//   - mainMenuActive — AddrMainMenuActive (0x2E4068): 1 at the front-end menu,
//     0 once a gameplay map is loaded. Gates whether we are actually in a map.
//   - stageName — AddrGlobalStageName (0x2FAC20): the engine writes this only
//     while MP-hosting (empty in menu and singleplayer). A HINT, never the
//     primary identity.
//
// The bug this replaces: cartographer derived the map from AddrGlobalStageName
// (and an unverified AddrGlobalScenarioPtr scan), so the map read blank/stale at
// the map-select menu and in singleplayer — 0x2FAC20 is only populated while
// MP-hosting.
//
// Policy: the scenario tag is authoritative in the menu/SP/MP steady states.
// The MP-host stage name only fills in when (a) the scenario tag is empty, or
// (b) a map is loaded (mainMenuActive==0) but the scenario tag still reflects
// the front-end UI scenario — the brief host-load lag the runtime pass observed,
// where the stage name leads the (still "levels\ui\uitest") scenario tag during
// an MP host load. Crucially, at the front-end menu (mainMenuActive==1) the
// stage name is never allowed to override the scenario tag, so a stale host
// stage name from a prior match cannot leak into the menu view.
func resolveMapName(scenarioTag string, mainMenuActive uint8, stageName string) string {
	inMap := mainMenuActive == 0
	if stageName != "" && (scenarioTag == "" || (inMap && isUIScenario(scenarioTag))) {
		return stageName
	}
	return scenarioTag
}

// isUIScenario reports whether a scenario tag path is the front-end UI scenario
// (e.g. "levels\ui\uitest") rather than a real gameplay map. Match is
// case-insensitive and tolerant of forward- or back-slash separators.
func isUIScenario(tag string) bool {
	t := strings.ToLower(tag)
	return strings.HasPrefix(t, `levels\ui\`) || strings.HasPrefix(t, "levels/ui/")
}
