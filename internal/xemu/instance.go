package xemu

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Instance represents one running xemu process with its memory reader and
// pre-translated address cache for low guest VAs.
type Instance struct {
	Name    string
	QMPSock string
	PID     int
	Mem     *Mem

	// lowHVAs caches host VAs for low guest VAs (< 0x80000000), translated
	// once at startup via gva2gpa + gpa2hva.
	lowHVAs map[uint32]int64
}

// Init finds the xemu PID, connects to QMP, translates all provided low guest
// VAs to host VAs, then opens /proc/<pid>/mem. Must be called before any reads.
func (inst *Instance) Init(lowGVAs []uint32) error {
	pid, err := findPID(inst.QMPSock)
	if err != nil {
		return fmt.Errorf("%s: find PID: %w", inst.Name, err)
	}

	qmp, err := newQMPClient(inst.QMPSock)
	if err != nil {
		return fmt.Errorf("%s: QMP: %w", inst.Name, err)
	}
	defer qmp.close()

	base, err := qmp.gpa2hva(0x0)
	if err != nil {
		return fmt.Errorf("%s: gpa2hva base: %w", inst.Name, err)
	}

	inst.lowHVAs = make(map[uint32]int64, len(lowGVAs))
	for _, gva := range lowGVAs {
		hva, err := qmp.translateLowGVA(gva)
		if err != nil {
			return fmt.Errorf("%s: translate 0x%x: %w", inst.Name, gva, err)
		}
		inst.lowHVAs[gva] = hva
	}

	f, err := os.OpenFile(fmt.Sprintf("/proc/%d/mem", pid), os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("%s: open /proc/%d/mem: %w", inst.Name, pid, err)
	}

	inst.PID = pid
	inst.Mem = &Mem{fd: int(f.Fd()), base: base, file: f}
	return nil
}

// Close releases the /proc/<pid>/mem file descriptor. Call before reinitialising
// the instance so the stale fd does not linger.
func (inst *Instance) Close() {
	if inst.Mem != nil && inst.Mem.file != nil {
		inst.Mem.file.Close()
		inst.Mem = nil
	}
}

// LowHVA returns the cached host VA for a low guest VA (< 0x80000000).
// Returns an error if the address was not translated at Init time.
func (inst *Instance) LowHVA(gva uint32) (int64, error) {
	hva, ok := inst.lowHVAs[gva]
	if !ok {
		return 0, fmt.Errorf("low GVA 0x%x not translated at Init", gva)
	}
	return hva, nil
}

// DerefLowPtr reads the u32 pointer stored at a low guest VA and returns the
// pointed-to value, which is expected to be a high guest VA (>= 0x80000000).
func (inst *Instance) DerefLowPtr(lowGVA uint32) (uint32, error) {
	hva, err := inst.LowHVA(lowGVA)
	if err != nil {
		return 0, err
	}
	return inst.Mem.ReadU32At(hva)
}

// findPID scans /proc for a process whose cmdline references the given QMP socket.
// Matches either AppRun (containerised xemu AppImage) or a bare xemu binary
// (native install), so both deployment modes work without a separate code path.
func findPID(qmpSock string) (int, error) {
	sockToken := filepath.Base(qmpSock)
	entries, _ := filepath.Glob("/proc/*/cmdline")
	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}
		fields := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if len(fields) == 0 {
			continue
		}
		exe := filepath.Base(fields[0])
		cmdline := strings.Join(fields, " ")
		if !strings.Contains(cmdline, sockToken) {
			continue
		}
		if exe == "AppRun" || strings.HasPrefix(exe, "xemu") {
			parts := strings.Split(entry, "/")
			if len(parts) >= 3 {
				if pid, err := strconv.Atoi(parts[2]); err == nil {
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no xemu/AppRun process found matching %s", sockToken)
}
