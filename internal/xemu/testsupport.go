package xemu

import "os"

// NewTestInstance builds an Instance whose memory reads resolve against an
// in-memory RAM image (written to a temp file that pread can address) plus a
// caller-supplied low-GVA → file-offset translation table. It lets packages
// that read guest memory (the game readers) unit-test their read logic without
// a live xemu.
//
// The mapped offsets are byte offsets into ram. A low GVA whose offset lies at
// or beyond len(ram) yields a genuine short-read/pread failure — which is how
// tests exercise the read-error paths (e.g. a game-.data region that isn't
// resident at the front-end menu). base is 0, so a high GVA (>= 0x80000000)
// reads ram[gva-0x80000000]; size the image accordingly if a test needs those.
//
// The returned cleanup closes the fd and removes the temp file; call it via
// t.Cleanup. This is test-only infrastructure — no production code constructs
// an Instance this way (Init translates against a live QMP socket instead).
func NewTestInstance(name string, ram []byte, lowHVAs map[uint32]int64) (*Instance, func(), error) {
	f, err := os.CreateTemp("", "xemu-testmem-")
	if err != nil {
		return nil, nil, err
	}
	if _, err := f.WriteAt(ram, 0); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, nil, err
	}
	inst := &Instance{
		Name:    name,
		PID:     os.Getpid(),
		Mem:     &Mem{fd: int(f.Fd()), base: 0, file: f},
		lowHVAs: make(map[uint32]int64, len(lowHVAs)),
	}
	for gva, hva := range lowHVAs {
		inst.lowHVAs[gva] = hva
	}
	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	return inst, cleanup, nil
}
