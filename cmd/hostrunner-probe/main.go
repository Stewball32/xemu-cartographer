// Command hostrunner-probe is the FIRST LIVE MILESTONE for the host-runner:
// a single GATED menu press. It reads live Halo: CE state (via the scraper's
// reader over QMP), classifies the host-flow screen, and asks the runner's
// gating logic whether a press is allowed — then, when a container's websockify
// endpoint is given, actually taps the key over the admin's RFB channel
// (internal/vncinput) and re-reads to confirm.
//
// The read→classify→decide half runs against ANY reachable xemu QMP socket
// (host rig or rootless container) with no VNC, so the runner's "never blind"
// gating is demonstrable immediately. The -url act half needs the container's
// Xvnc.
//
// Usage:
//
//	# read + classify + gated decision only (no press):
//	go run ./cmd/hostrunner-probe -sock /run/user/1000/xemu-qmp-ce-nav.sock -expect system_link -key a
//
//	# full read → gated act → verify against a live container:
//	go run ./cmd/hostrunner-probe -sock <qmp> -url ws://127.0.0.1:3103/websockify -expect system_link -key a
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"
	"github.com/Stewball32/xemu-cartographer/internal/vncinput"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

var screens = map[string]hostrunner.Screen{
	"main_menu":   hostrunner.ScreenMainMenu,
	"system_link": hostrunner.ScreenSystemLink,
	"hosting":     hostrunner.ScreenHosting,
	"lobby":       hostrunner.ScreenLobby,
	"in_game":     hostrunner.ScreenInGame,
	"post_game":   hostrunner.ScreenPostGame,
}

func main() {
	sock := flag.String("sock", "/run/user/1000/xemu-qmp-ce-nav.sock", "xemu QMP socket to read CE state from")
	name := flag.String("name", "probe", "instance name (label)")
	url := flag.String("url", "", "optional container websockify URL — when set, actually taps via vncinput")
	expectStr := flag.String("expect", "system_link", "screen the press is gated on (main_menu/system_link/hosting/lobby)")
	key := flag.String("key", "a", "vncinput label to tap when gated-ok (a/y/b/Up/Down/...)")
	flag.Parse()

	expect, ok := screens[*expectStr]
	if !ok {
		log.Fatalf("unknown -expect %q", *expectStr)
	}

	obs := readObservation(*sock)
	fmt.Printf("== hostrunner-probe ==\n")
	fmt.Printf("read: fresh=%v phase=%s menu_active=%v game_connection=%d map=%q gametype=%q machines=%d\n",
		obs.Fresh, obs.Phase, obs.MenuActive, obs.Connection, obs.Map, obs.Gametype, obs.MachineCount)
	fmt.Printf("classified screen = %s   ready_to_start(2+box,2+team) = %v\n", hostrunner.Classify(obs), obs.ReadyToStart())

	// Pure gated decision — the "never blind" check.
	decision := hostrunner.GatedPress(obs, expect, *key)
	fmt.Printf("gated decision (expect %s, key %s): %s — %s\n", expect, *key, decision.Kind, decision.Reason)

	if *url == "" {
		fmt.Println("\n(no -url: read→classify→decide only. Provide a container's /websockify to perform the tap.)")
		return
	}

	// Full read → gated act → verify.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	inj, err := vncinput.Dial(ctx, *url)
	if err != nil {
		log.Fatalf("vncinput dial: %v", err)
	}
	defer inj.Close()

	runner := hostrunner.New(hostrunner.Config{Instance: *name}, inj, nil)
	fmt.Printf("\n[act] runner.GatedPress via vncinput...\n")
	act := runner.GatedPress(obs, expect, *key)
	fmt.Printf("[act] %s — %s\n", act.Kind, act.Reason)

	if act.Kind != hostrunner.ActionTap {
		fmt.Println("gate did not allow a press; nothing sent.")
		return
	}
	time.Sleep(700 * time.Millisecond)
	after := readObservation(*sock)
	fmt.Printf("[verify] game_connection %d → %d, screen %s → %s\n",
		obs.Connection, after.Connection, hostrunner.Classify(obs), hostrunner.Classify(after))
}

// readObservation reads the minimal CE menu/lobby state through the haloce
// reader (never touching a GameReader concurrently — this is a standalone,
// single-goroutine read) and projects it into a hostrunner.Observation.
func readObservation(sock string) hostrunner.Observation {
	inst := &xemu.Instance{Name: "probe", QMPSock: sock}
	if err := inst.Init(haloce.AllLowGVAs); err != nil {
		log.Fatalf("init instance: %v", err)
	}
	defer inst.Close()

	reader := haloce.NewReader(inst, "probe")
	state, tick, err := reader.ReadGameState()
	if err != nil {
		log.Fatalf("ReadGameState: %v", err)
	}
	inputs := reader.LastStateInputs()
	mainMenu := asU64(inputs["main_menu"]) != 0

	gc := 0
	if hva, err := inst.LowHVA(haloce.AddrGameConnection); err == nil {
		if v, err := inst.Mem.ReadU16At(hva); err == nil {
			gc = int(v)
		}
	}

	// Best-effort lobby fields for the readiness view.
	ro := hostrunner.ScraperReadout{
		Fresh:          true,
		Tick:           tick,
		GameState:      string(state),
		MenuActive:     mainMenu,
		GameConnection: gc,
	}
	if gd, err := reader.ReadReadyState(); err == nil {
		ro.Map = gd.Map
		ro.Gametype = gd.Gametype
		ro.MachineCount = len(gd.Machines)
		ro.PlayerCount = len(gd.Players)
		ro.TeamCount = len(gd.TeamScores) // best-effort readiness view
	}
	return ro.Observation()
}

func asU64(v any) uint64 {
	switch n := v.(type) {
	case uint8:
		return uint64(n)
	case uint16:
		return uint64(n)
	case uint32:
		return uint64(n)
	case int:
		return uint64(n)
	default:
		return 0
	}
}
