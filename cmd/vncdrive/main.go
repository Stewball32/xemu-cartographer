// Command vncdrive is a throwaway host-side driver that uses the PRODUCTION
// vncinput.Injector (the same RFB-over-WebSocket key path the host runner uses) to
// tap a sequence of key labels into a container's browser-kiosk websockify — to
// verify vncinput → Xvnc → kiosk → xemu input in-container.
//
//	vncdrive <ws-url> <label> [label ...]
//
// Labels are the runner's vncinput labels: a b x y Up Down Left Right Return ...
// A ~350ms gap between taps lets the CE menu register each one.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/vncinput"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: vncdrive <ws-url> <label> [label ...]")
		os.Exit(2)
	}
	url := os.Args[1]
	labels := os.Args[2:]

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	inj, err := vncinput.Dial(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer inj.Close()

	// Mirror the production pump: focus the Selkies canvas so KeyEvents register.
	if err := inj.FocusClick(); err != nil {
		fmt.Fprintf(os.Stderr, "focus-click: %v\n", err)
	}
	time.Sleep(200 * time.Millisecond)

	for _, l := range labels {
		if err := inj.Tap(l); err != nil {
			fmt.Fprintf(os.Stderr, "tap %q: %v\n", l, err)
			os.Exit(1)
		}
		fmt.Printf("tapped %s\n", l)
		time.Sleep(350 * time.Millisecond)
	}
}
