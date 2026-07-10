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
		{"system link", Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnSystemLink}, ScreenSystemLink},
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
