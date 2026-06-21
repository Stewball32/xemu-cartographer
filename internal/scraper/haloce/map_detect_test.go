package haloce

import "testing"

// TestResolveMapName covers the three states the 2026-06-21 runtime pass
// measured (docs/RUNTIME-PASS-2026-06-21-CE.md, Task 7), proving the map
// identity is taken from the scenario tag path in the menu/SP/MP cases rather
// than from AddrGlobalStageName (which is empty outside MP-hosting).
func TestResolveMapName(t *testing.T) {
	const (
		uiScenario   = `levels\ui\uitest`
		campaignMap  = `levels\test\prisbots\a10`
		mpPrisoner   = `levels\test\prisoner\prisoner`
		mpHangEmHigh = `levels\test\hangemhigh\hangemhig`
	)

	tests := []struct {
		name           string
		scenarioTag    string
		mainMenuActive uint8
		stageName      string
		want           string
	}{
		{
			// Map-select menu: scenario tag is the front-end UI scenario,
			// stage name empty. Must report the scenario tag, not "".
			name:           "menu picks scenario tag",
			scenarioTag:    uiScenario,
			mainMenuActive: 1,
			stageName:      "",
			want:           uiScenario,
		},
		{
			// Singleplayer/campaign: scenario tag is the real map, main menu
			// inactive, stage name empty (was blank before this fix).
			name:           "singleplayer picks scenario tag",
			scenarioTag:    campaignMap,
			mainMenuActive: 0,
			stageName:      "",
			want:           campaignMap,
		},
		{
			// MP host steady state: both signals agree; scenario tag wins.
			name:           "mp host picks scenario tag",
			scenarioTag:    mpPrisoner,
			mainMenuActive: 0,
			stageName:      mpPrisoner,
			want:           mpPrisoner,
		},
		{
			// MP host load lag: map is loading (main menu inactive) but the
			// scenario tag still reflects the UI scenario while the host stage
			// name already names the target map — the hint leads here.
			name:           "mp host load lag uses stage-name hint",
			scenarioTag:    uiScenario,
			mainMenuActive: 0,
			stageName:      mpHangEmHigh,
			want:           mpHangEmHigh,
		},
		{
			// At the front-end menu a stale host stage name from a prior match
			// must NOT override the (correct) UI scenario tag.
			name:           "menu ignores stale stage-name hint",
			scenarioTag:    uiScenario,
			mainMenuActive: 1,
			stageName:      mpHangEmHigh,
			want:           uiScenario,
		},
		{
			// Early pregame: scenario tag not yet resolvable, but the host
			// stage name is already populated — fall back to it.
			name:           "empty scenario tag falls back to stage hint",
			scenarioTag:    "",
			mainMenuActive: 0,
			stageName:      mpPrisoner,
			want:           mpPrisoner,
		},
		{
			// Nothing resolvable yet: stay empty rather than invent a map.
			name:           "nothing resolvable stays empty",
			scenarioTag:    "",
			mainMenuActive: 1,
			stageName:      "",
			want:           "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveMapName(tc.scenarioTag, tc.mainMenuActive, tc.stageName)
			if got != tc.want {
				t.Errorf("resolveMapName(%q, %d, %q) = %q, want %q",
					tc.scenarioTag, tc.mainMenuActive, tc.stageName, got, tc.want)
			}
		})
	}
}

func TestIsUIScenario(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{`levels\ui\uitest`, true},
		{`levels\ui\mainmenu`, true},
		{`LEVELS\UI\UITEST`, true}, // case-insensitive
		{`levels/ui/uitest`, true}, // forward slashes tolerated
		{`levels\test\prisoner\prisoner`, false},
		{`levels\test\hangemhigh\hangemhig`, false},
		{``, false},
		{`levels\uiother\x`, false}, // must be the ui directory, not a prefix collision
	}
	for _, tc := range tests {
		if got := isUIScenario(tc.tag); got != tc.want {
			t.Errorf("isUIScenario(%q) = %v, want %v", tc.tag, got, tc.want)
		}
	}
}
