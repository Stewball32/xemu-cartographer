package hostrunner

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		obs  Observation
		want Screen
	}{
		{"stale", Observation{Fresh: false, Connection: ConnSystemLink}, ScreenUnknown},
		{"main menu", Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu}, ScreenMainMenu},
		{"menu but not active (attract)", Observation{Fresh: true, Phase: PhaseMenu, MenuActive: false, Connection: ConnMenu}, ScreenUnknown},
		{"system link games via high-GVA widget (conn stale-0)", Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu, MenuItem: MenuItemSystemLinkGames}, ScreenSystemLink},
		{"conn==1 alone no longer classifies system-link (widget-only)", Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnSystemLink}, ScreenUnknown},
		{"hosting blind cards", Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting}, ScreenHosting},
		{"lobby via machines", Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting, MachineCount: 1}, ScreenLobby},
		{"lobby via map+gametype", Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting, Map: "bloodgulch", Gametype: "slayer"}, ScreenLobby},
		{"lobby via pregame", Observation{Fresh: true, Phase: PhasePreGame, Connection: ConnHosting}, ScreenLobby},
		{"in game wins over connection", Observation{Fresh: true, Phase: PhaseInGame, Connection: ConnMenu}, ScreenInGame},
		{"post game", Observation{Fresh: true, Phase: PhasePostGame, Connection: ConnHosting}, ScreenPostGame},
	}
	for _, c := range cases {
		if got := Classify(c.obs); got != c.want {
			t.Errorf("%s: Classify = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReadyToStart(t *testing.T) {
	if (Observation{MachineCount: 2, TeamCount: 2}).ReadyToStart() != true {
		t.Error("2 boxes + 2 teams should be ready")
	}
	if (Observation{MachineCount: 1, TeamCount: 2}).ReadyToStart() != false {
		t.Error("1 box should not be ready")
	}
	if (Observation{MachineCount: 2, TeamCount: 1}).ReadyToStart() != false {
		t.Error("1 team should not be ready")
	}
}

// A box that BOOTED INTO a persisted pregame lobby lingers the System Link
// server_list (menu_item=SystemLinkGames), but its highlighted widget is under
// \connected\pregame — it must classify as the LOBBY, never system_link (which
// would loop "create system-link game"/Y). Runtime-observed on Stewart's box.
func TestPregameLobbyOverridesStickySystemLink(t *testing.T) {
	base := Observation{
		Fresh:    true,
		MenuItem: MenuItemSystemLinkGames, // server_list lingering
		Dela:     `ui\shell\main_menu\multiplayer_type_select\connected\pregame\xbox_graphic`,
	}
	for _, phase := range []Phase{PhasePreGame, PhaseMenu} {
		for _, conn := range []Connection{ConnHosting, ConnMenu} {
			o := base
			o.Phase, o.Connection = phase, conn
			if got := Classify(o); got != ScreenLobby {
				t.Fatalf("pregame dela phase=%v conn=%v: got %v, want ScreenLobby (not system_link)", phase, conn, got)
			}
		}
	}
	// Sanity: the ACTUAL System Link games browser (no \connected\pregame) still
	// classifies as system_link so Y-create still fires.
	browser := Observation{Fresh: true, Connection: ConnMenu, MenuItem: MenuItemSystemLinkGames,
		Dela: `ui\shell\main_menu\multiplayer_type_select\connected\server_list\server_list_item0`}
	if got := Classify(browser); got != ScreenSystemLink {
		t.Fatalf("System Link games browser must stay ScreenSystemLink, got %v", got)
	}
}
