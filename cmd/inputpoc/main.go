// Command inputpoc is the proof-of-concept for the programmatic input keystone:
// it runs the read → act → verify loop against a LIVE xemu instance to prove
// (or disprove) that the new xemu.SendKey primitive drives Xbox input.
//
//	read   — resolve game state via the haloce reader (the "state gate"), then
//	         locate the local player's biped and sample its position/crouch
//	         through internal/xemu's /proc/<pid>/mem path.
//	act    — issue a movement keypress via inst.SendKeyHold (the new primitive).
//	verify — re-sample the biped across the hold window and report the delta.
//
// It only ACTS when the state gate passes (in-game with a live local biped),
// which is the "state-gated action" the keystone must demonstrate.
//
// Optional positive control: with -control-fifo pointing at a padpool command
// FIFO (the offset-rig's virtual-pad channel), the tool drives the same
// biped through real pad input after the sendkey attempt, so a single run
// shows both "did QMP sendkey move it?" and "does the biped move at all?".
//
// This is deliberately a standalone command, single-goroutine and the sole
// owner of the instance — it never touches a running scraper runner's
// GameReader, honouring the reader concurrency model.
//
// Usage:
//
//	go run ./cmd/inputpoc -sock /run/user/1000/xemu-qmp-ce-nav.sock \
//	    -key e -hold 900ms [-control-fifo /run/user/1000/xemu-harness/ce-nav/p1.fifo]
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"
	"unicode/utf16"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

func main() {
	sock := flag.String("sock", "/run/user/1000/xemu-qmp-ce-nav.sock", "xemu QMP unix socket")
	name := flag.String("name", "ce-nav", "instance name (label only)")
	key := flag.String("key", "e", "logical key to send (VNC-map label; e=left-stick-forward)")
	hold := flag.Duration("hold", 900*time.Millisecond, "how long to hold the key")
	samples := flag.Int("samples", 8, "biped samples taken across the hold window")
	moveEps := flag.Float64("move-eps", 0.05, "min world-unit displacement counted as movement")
	controlFifo := flag.String("control-fifo", "", "optional padpool command FIFO for the positive control")
	flag.Parse()

	inst := &xemu.Instance{Name: *name, QMPSock: *sock}
	if err := inst.Init(haloce.AllLowGVAs); err != nil {
		log.Fatalf("init instance: %v", err)
	}
	defer inst.Close()
	fmt.Printf("== inputpoc: %s (pid=%d, sock=%s) ==\n", inst.Name, inst.PID, *sock)

	reader := haloce.NewReader(inst, inst.Name)

	// ---- READ: state gate ----
	state, tick, err := reader.ReadGameState()
	if err != nil {
		log.Fatalf("ReadGameState: %v", err)
	}
	fmt.Printf("state=%s tick=%d inputs=%v\n", state, tick, reader.LastStateInputs())

	bip, err := findLocalBiped(inst)
	if err != nil {
		log.Fatalf("locate local biped: %v", err)
	}
	fmt.Printf("local player: slot=%d name=%q handle=0x%08X biped@0x%08X\n",
		bip.slot, bip.name, bip.handle, bip.dataAddr)

	if state != scraper.GameStateInGame && state != scraper.GameStatePostGame {
		log.Fatalf("state gate: %q is not a drivable in-game state — aborting (nothing to drive)", state)
	}
	fmt.Printf("state gate PASSED (%s, live biped) — proceeding to act\n\n", state)

	// ---- ACT + VERIFY: QMP sendkey ----
	fmt.Printf("[sendkey] baseline...\n")
	base := samplePos(inst, bip.dataAddr)
	printSample("  baseline", base, base)

	fmt.Printf("[sendkey] SendKeyHold(%q, %s) then sampling %d× ...\n", *key, *hold, *samples)
	if err := inst.SendKeyHold(*key, *hold); err != nil {
		log.Fatalf("SendKeyHold: %v", err)
	}
	maxKey := watch(inst, bip.dataAddr, base, *hold, *samples, "  +sendkey")
	fmt.Printf("[sendkey] max displacement = %.4f, max speed = %.4f\n", maxKey.disp, maxKey.speed)
	sendkeyMoved := maxKey.disp >= *moveEps
	verdict("QMP sendkey", sendkeyMoved)

	// ---- Optional positive control: real pad input via padpool FIFO ----
	if *controlFifo != "" {
		fmt.Printf("\n[control] driving pad via %s (move 1) ...\n", *controlFifo)
		settle(inst, bip.dataAddr)
		cbase := samplePos(inst, bip.dataAddr)
		if err := writeFifo(*controlFifo, "move 1"); err != nil {
			log.Fatalf("control FIFO: %v", err)
		}
		maxCtl := watch(inst, bip.dataAddr, cbase, time.Second, *samples, "  +padmove")
		fmt.Printf("[control] max displacement = %.4f, max speed = %.4f\n", maxCtl.disp, maxCtl.speed)
		verdict("padpool pad", maxCtl.disp >= *moveEps)
	}

	fmt.Printf("\n== done ==\n")
}

// ---- biped location + sampling ---------------------------------------------

type biped struct {
	slot     int
	name     string
	handle   uint32
	dataAddr uint32
}

type vec3 struct{ x, y, z float32 }

type reading struct {
	disp  float64 // max displacement from baseline
	speed float64 // max instantaneous velocity magnitude
}

// findLocalBiped walks the player datum array, picks the local player (the one
// with a non-negative local index — the host's own player), and resolves its
// biped object-data address through the object header datum array.
func findLocalBiped(inst *xemu.Instance) (biped, error) {
	m := inst.Mem
	pda, err := inst.DerefLowPtr(haloce.AddrPlayerDatumArrayPtr)
	if err != nil || pda < haloce.HighGVAThreshold {
		return biped{}, fmt.Errorf("player datum array ptr invalid (0x%08X): %v", pda, err)
	}
	elemSize, _ := m.ReadU16(pda + haloce.OffPDAElementSize)
	maxCount, _ := m.ReadU16(pda + haloce.OffPDAMaxCount)
	first, _ := m.ReadU32(pda + haloce.OffPDAFirstElement)
	if elemSize == 0 || maxCount == 0 || first < haloce.HighGVAThreshold {
		return biped{}, fmt.Errorf("player array header unpopulated (elem=%d max=%d first=0x%08X)", elemSize, maxCount, first)
	}

	var fallback *biped
	for i := 0; i < int(maxCount); i++ {
		rec := first + uint32(i)*uint32(elemSize)
		sig, _ := m.ReadU16(rec) // 0 = empty slot
		if sig == 0 {
			continue
		}
		localIdx, _ := m.ReadS16(rec + haloce.OffPlrLocalIndex)
		handle, _ := m.ReadU32(rec + haloce.OffPlrObjectHandle)
		name := readUTF16(inst, rec+haloce.OffPlrName, 24)
		data := resolveObjData(inst, handle)
		if data == 0 {
			continue // dead / no biped this slot
		}
		b := biped{slot: i, name: name, handle: handle, dataAddr: data}
		if localIdx >= 0 {
			return b, nil // the local (host) player
		}
		if fallback == nil {
			cp := b
			fallback = &cp
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return biped{}, fmt.Errorf("no player with a live biped found")
}

// resolveObjData maps an object handle to its object-data address via the
// object header datum array (entry stride 12, data pointer at +0x08).
func resolveObjData(inst *xemu.Instance, handle uint32) uint32 {
	if handle == 0 || handle&0xFFFF == 0xFFFF {
		return 0
	}
	m := inst.Mem
	ohd, err := inst.DerefLowPtr(haloce.AddrObjectHeaderDatumPtr)
	if err != nil || ohd < haloce.HighGVAThreshold {
		return 0
	}
	elemSize, _ := m.ReadU16(ohd + haloce.OffOHDElementSize)
	ofirst, _ := m.ReadU32(ohd + haloce.OffOHDFirstElement)
	if elemSize == 0 || ofirst < haloce.HighGVAThreshold {
		return 0
	}
	idx := handle & 0xFFFF
	entry := ofirst + idx*uint32(elemSize)
	data, _ := m.ReadU32(entry + haloce.OffObjEntryDataAddr)
	if data < haloce.HighGVAThreshold {
		return 0
	}
	return data
}

func samplePos(inst *xemu.Instance, data uint32) vec3 {
	m := inst.Mem
	x, _ := m.ReadF32(data + haloce.OffObjX)
	y, _ := m.ReadF32(data + haloce.OffObjX + 4)
	z, _ := m.ReadF32(data + haloce.OffObjX + 8)
	return vec3{x, y, z}
}

// watch samples the biped across a window, tracking the max displacement from
// baseline and the max velocity magnitude (biped velocity lives at +0x18).
func watch(inst *xemu.Instance, data uint32, base vec3, window time.Duration, samples int, tag string) reading {
	var out reading
	dt := window / time.Duration(samples)
	m := inst.Mem
	for i := 0; i < samples; i++ {
		time.Sleep(dt)
		p := samplePos(inst, data)
		vx, _ := m.ReadF32(data + 0x18)
		vy, _ := m.ReadF32(data + 0x1C)
		vz, _ := m.ReadF32(data + 0x20)
		disp := dist(p, base)
		spd := math.Sqrt(float64(vx*vx + vy*vy + vz*vz))
		if disp > out.disp {
			out.disp = disp
		}
		if spd > out.speed {
			out.speed = spd
		}
		crouch, _ := m.ReadF32(data + haloce.OffDynCrouchScale)
		printSampleLine(tag, p, disp, spd, crouch)
	}
	return out
}

// settle waits until the biped's velocity is ~0 so a control run starts clean.
func settle(inst *xemu.Instance, data uint32) {
	m := inst.Mem
	for i := 0; i < 20; i++ {
		vx, _ := m.ReadF32(data + 0x18)
		vy, _ := m.ReadF32(data + 0x1C)
		vz, _ := m.ReadF32(data + 0x20)
		if math.Sqrt(float64(vx*vx+vy*vy+vz*vz)) < 1e-4 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ---- output helpers --------------------------------------------------------

func verdict(mechanism string, moved bool) {
	if moved {
		fmt.Printf(">>> %s: INPUT REGISTERED (biped moved)\n", mechanism)
	} else {
		fmt.Printf(">>> %s: NO OBSERVED EFFECT (biped did not move)\n", mechanism)
	}
}

func printSample(tag string, p, base vec3) {
	printSampleLine(tag, p, dist(p, base), 0, 0)
}

func printSampleLine(tag string, p vec3, disp, spd float64, crouch float32) {
	fmt.Printf("%-12s pos=(%.2f,%.2f,%.2f) disp=%.4f speed=%.4f crouch=%.2f\n",
		tag, p.x, p.y, p.z, disp, spd, crouch)
}

func dist(a, b vec3) float64 {
	dx, dy, dz := float64(a.x-b.x), float64(a.y-b.y), float64(a.z-b.z)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func readUTF16(inst *xemu.Instance, gva uint32, nBytes int) string {
	raw, err := inst.Mem.ReadBytes(gva, nBytes)
	if err != nil {
		return ""
	}
	u := make([]uint16, 0, nBytes/2)
	for i := 0; i+1 < len(raw); i += 2 {
		c := binary.LittleEndian.Uint16(raw[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

func writeFifo(path, cmd string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(cmd + "\n")
	return err
}
