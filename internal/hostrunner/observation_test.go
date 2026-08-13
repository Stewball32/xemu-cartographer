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

// screenFromUiPath maps a screen-record tag path (2026-08-10 classifier) to the
// host-flow screen; unknown/empty paths must yield ScreenUnknown so Classify falls
// through to the heap-derived signals.
func TestScreenFromUiPath(t *testing.T) {
	cases := []struct {
		path string
		want Screen
	}{
		{"", ScreenUnknown},
		{uiPathMainMenuRoot, ScreenMainMenu},
		{`ui\shell\main_menu\multiplayer_type_select\multiplayer_type_select_screen`, ScreenMainMenu},
		{`ui\shell\main_menu\new_campaign_creating_profile`, ScreenMainMenu},
		{`ui\shell\main_menu\multiplayer_type_select\connected\4way_profile_select\4way_start2join_screen`, ScreenMainMenu},
		{`ui\shell\main_menu\multiplayer_type_select\connected\server_list\server_list_screen`, ScreenSystemLink},
		{`ui\shell\main_menu\multiplayer_type_select\connected\pregame\pregame_screen`, ScreenLobby},
		{`ui\shell\main_menu\multiplayer_type_select\mp_map_select\mp_map_select_screen`, ScreenHosting},
		{`ui\shell\main_menu\gametype_select\gametype_select_screen`, ScreenHosting},
		{`levels\a30\a30`, ScreenUnknown}, // a scenario path, not a shell screen
	}
	for _, c := range cases {
		if got := screenFromUiPath(c.path); got != c.want {
			t.Errorf("screenFromUiPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// AtRootMainMenu needs BOTH the resolved root-menu screen AND back-rec == 0 — a
// cold, unreadable record pool (everything zero) must NOT read as a confirmed root.
func TestAtRootMainMenu(t *testing.T) {
	if !(Observation{UiScreen: uiPathMainMenuRoot, UiBackScreenRec: 0}).AtRootMainMenu() {
		t.Error("root path + back 0 should confirm the root menu")
	}
	if (Observation{UiScreen: uiPathMainMenuRoot, UiBackScreenRec: 0x2E40C8}).AtRootMainMenu() {
		t.Error("a non-zero back record means a sub-screen — not root")
	}
	if (Observation{UiScreen: "", UiBackScreenRec: 0}).AtRootMainMenu() {
		t.Error("an unreadable record pool must not confirm root")
	}
	if (Observation{UiScreen: `ui\shell\main_menu\multiplayer_type_select\multiplayer_type_select_screen`, UiBackScreenRec: 0}).AtRootMainMenu() {
		t.Error("a sub-screen path must not confirm root even with back 0")
	}
}

// The screen-record classifier fills the gaps the heap signals leave — most
// importantly the COLD main menu (record readable, widget tree unbuilt) — but sits
// BELOW the runtime-verified signals so it can never override them.
func TestClassifyScreenRecord(t *testing.T) {
	cases := []struct {
		name string
		obs  Observation
		want Screen
	}{
		{"cold blank menu classifies via record (stale main_menu=0)",
			Observation{Fresh: true, Phase: PhaseMenu, MenuActive: false, Connection: ConnMenu,
				UiScreen: uiPathMainMenuRoot}, ScreenMainMenu},
		{"campaign ENTER NAME (no recognisable item) is the front-end",
			Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
				UiScreen: `ui\shell\main_menu\new_campaign_creating_profile`}, ScreenMainMenu},
		{"server_list record classifies the CREATE screen",
			Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
				UiScreen: `ui\shell\main_menu\multiplayer_type_select\connected\server_list\server_list_screen`}, ScreenSystemLink},
		{"create-flow map select record + live carousel classify as hosting",
			Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting,
				MapCursorCount: 13, MapCursorValid: true,
				UiScreen: `ui\shell\main_menu\multiplayer_type_select\connected\connected_map_select_wrapper`}, ScreenHosting},
		{"pregame record beats lingering server_list item (no dela needed — the pick reports no live item there)",
			Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting,
				MenuItem: MenuItemSystemLinkGames,
				UiScreen: `ui\shell\main_menu\multiplayer_type_select\connected\pregame\connected_pregame_screen`}, ScreenLobby},
		{"recognised highlighted item (verified signal) outranks the record",
			Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
				MenuItem: MenuItemSystemLinkGames, UiScreen: uiPathMainMenuRoot}, ScreenSystemLink},
		{"engine phase outranks everything",
			Observation{Fresh: true, Phase: PhaseInGame, UiScreen: uiPathMainMenuRoot}, ScreenInGame},
	}
	for _, c := range cases {
		if got := Classify(c.obs); got != c.want {
			t.Errorf("%s: Classify = %v, want %v", c.name, got, c.want)
		}
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
