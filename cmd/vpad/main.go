// Command vpad is the Go port of scripts/runtime/padpool.py: it creates one
// backend-owned uinput virtual Xbox-360 pad (internal/vpad), writes the xemu
// TOML binding fragment + a manifest, and then holds the device open while
// serving drive commands over a FIFO. Closing the process destroys the pad.
//
// It is both the live-PoC pad holder and the shape the state-aware runner's
// provisioning will take: create pad → emit its GUID → xemu launches bound to
// that GUID → drive the pad.
//
// By default it blocks the Cowork/Claude-Code per-Bash-call teardown signals
// (the proven padpool trick) so a detached pad survives across tool calls;
// stop it cleanly with `echo quit > <rundir>/cmd.fifo` or SIGKILL.
//
// Usage:
//
//	vpad -rundir /run/user/1000/xemu-vpad/poc [-version 0x0701] [-name NAME]
//
// FIFO vocabulary (one command per line):
//
//	move [s] | bwd [s] | sl [s] | sr [s]     left-stick hold (s seconds, default 1)
//	tap <label>                              press+release a labelled control
//	hold <label> <s>                         hold a control s seconds
//	<label>                                  bare label = tap (e.g. a, Up, Return)
//	neutral                                  recentre everything
//	quit                                     close the pad and exit
//
// Labels mirror internal/vpad.SupportedLabels (the xemu.SendKey vocabulary).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/vpad"
)

func main() {
	rundir := flag.String("rundir", "", "run dir for manifest/binds/FIFO (required)")
	versionStr := flag.String("version", "0x0701", "USB version (sets the SDL GUID); keep distinct from other pads")
	name := flag.String("name", "", "uinput device name (default Microsoft X-Box 360 pad)")
	port := flag.Int("port", 1, "xemu port number the binds.toml fragment targets")
	noBlock := flag.Bool("no-block-signals", false, "do not block teardown signals")
	flag.Parse()

	if *rundir == "" {
		log.Fatal("vpad: -rundir is required")
	}
	version, err := strconv.ParseInt(strings.TrimSpace(*versionStr), 0, 32)
	if err != nil {
		log.Fatalf("vpad: bad -version %q: %v", *versionStr, err)
	}
	if err := os.MkdirAll(*rundir, 0o755); err != nil {
		log.Fatalf("vpad: mkdir %s: %v", *rundir, err)
	}

	// Harden against the harness tearing down a Bash call's process tree.
	if !*noBlock {
		signal.Ignore(syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP,
			syscall.SIGQUIT, syscall.SIGPIPE, syscall.SIGSTKFLT)
	}

	pad, err := vpad.New(vpad.Options{Name: *name, Version: int(version)})
	if err != nil {
		log.Fatalf("vpad: create pad: %v", err)
	}
	defer pad.Close()

	// Emit the xemu TOML binding fragment + a manifest (manifest written last =
	// readiness marker, same convention as padpool.py).
	bindsPath := filepath.Join(*rundir, "binds.toml")
	if err := os.WriteFile(bindsPath, []byte(bindsTOML(*port, pad.GUID())), 0o644); err != nil {
		log.Fatalf("vpad: write binds.toml: %v", err)
	}
	fifoPath := filepath.Join(*rundir, "cmd.fifo")
	if err := ensureFIFO(fifoPath); err != nil {
		log.Fatalf("vpad: mkfifo: %v", err)
	}
	manifestPath := filepath.Join(*rundir, "pad.json")
	manifest := fmt.Sprintf(`{"guid":%q,"name":%q,"port":%d,"binds":%q,"fifo":%q}`+"\n",
		pad.GUID(), pad.Name(), *port, bindsPath, fifoPath)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		log.Fatalf("vpad: write manifest: %v", err)
	}

	fmt.Printf("=== vpad ready ===\n")
	fmt.Printf("  guid : %s\n", pad.GUID())
	fmt.Printf("  binds: %s\n", bindsPath)
	fmt.Printf("  fifo : %s\n", fifoPath)
	fmt.Printf("  stop : echo quit > %s  (or SIGKILL)\n", fifoPath)
	os.Stdout.Sync()

	serve(pad, fifoPath)
}

// serve reads commands from the FIFO until "quit". Reopens on writer EOF so the
// pad stays drivable across many short-lived writers (each `echo > fifo`).
func serve(pad *vpad.Pad, fifoPath string) {
	for {
		f, err := os.OpenFile(fifoPath, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("vpad: reopen fifo: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if handle(pad, sc.Text()) {
				f.Close()
				log.Printf("vpad: quit")
				return
			}
		}
		f.Close()
	}
}

// handle runs one command line; returns true on "quit".
func handle(pad *vpad.Pad, line string) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return false
	}
	cmd := fields[0]
	arg := func(i int) string {
		if len(fields) > i {
			return fields[i]
		}
		return ""
	}
	secs := func(s string, def float64) time.Duration {
		if s == "" {
			return time.Duration(def * float64(time.Second))
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			v = def
		}
		return time.Duration(v * float64(time.Second))
	}

	var err error
	switch cmd {
	case "quit":
		return true
	case "neutral":
		err = pad.Neutral()
	case "move": // left stick forward
		err = pad.TapHold("e", secs(arg(1), 1))
	case "bwd":
		err = pad.TapHold("d", secs(arg(1), 1))
	case "sl":
		err = pad.TapHold("s", secs(arg(1), 1))
	case "sr":
		err = pad.TapHold("f", secs(arg(1), 1))
	case "tap":
		err = pad.Tap(arg(1))
	case "hold":
		err = pad.TapHold(arg(1), secs(arg(2), 0.3))
	default:
		// Bare label → tap.
		err = pad.Tap(cmd)
	}
	if err != nil {
		log.Printf("vpad: %q: %v", line, err)
	}
	return false
}

func bindsTOML(port int, guid string) string {
	return fmt.Sprintf(`[input]
auto_bind = false
background_input_capture = true
gamepad_mappings = [
  { gamepad_id = '%s' },
]

[input.bindings]
port%d_driver = 'usb-xbox-gamepad'
port%d = '%s'
`, guid, port, port, guid)
}

func ensureFIFO(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return syscall.Mkfifo(path, 0o644)
}
