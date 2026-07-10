// Command vncinput-poc drives a container's Xbox controller over the admin's
// RFB channel: it dials a container's websockify endpoint (the same target
// internal/pocketbase/routes/containers/vnc.go proxies to) and taps a sequence
// of buttons via internal/vncinput — server-side RFB KeyEvents, identical to
// what the admin panel's on-screen controller sends.
//
// This is the "act" half of the runner's read→act→verify loop for the keyboard
// channel. Pair it with a state read (the server's scraper / debug page, or QMP
// against the container's bind-mounted socket) to confirm the input registered
// in guest memory.
//
// Usage:
//
//	# against a live container's browser-web websockify port (see ContainerInfo.Ports):
//	go run ./cmd/vncinput-poc -url ws://127.0.0.1:3103/websockify -keys "Down Down a"
//
// Keys are space-separated logical labels (see internal/vncinput.SupportedLabels):
// a b x y, Up Down Left Right, Return (Start), BackSpace (Back), 1 2 3 4, etc.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/vncinput"
)

func main() {
	url := flag.String("url", "", "websockify RFB URL, e.g. ws://127.0.0.1:3103/websockify (required)")
	keys := flag.String("keys", "a", "space-separated sequence of button labels to tap")
	hold := flag.Duration("hold", 90*time.Millisecond, "per-key hold duration")
	gap := flag.Duration("gap", 250*time.Millisecond, "delay between taps")
	flag.Parse()

	if *url == "" {
		log.Fatal("vncinput-poc: -url is required (a container's /websockify endpoint)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Printf("dialing %s ...\n", *url)
	inj, err := vncinput.Dial(ctx, *url)
	if err != nil {
		log.Fatalf("dial/handshake: %v", err)
	}
	defer inj.Close()
	fmt.Printf("RFB handshake OK — driving keys: %s\n", *keys)

	for _, label := range strings.Fields(*keys) {
		if err := inj.TapHold(label, *hold); err != nil {
			log.Fatalf("tap %q: %v", label, err)
		}
		fmt.Printf("  tapped %s (keysym 0x%x)\n", label, vncinput.KEYSYM[label])
		time.Sleep(*gap)
	}
	fmt.Println("done — verify the state change via the server's scraper / debug page.")
}
