package xemu

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"syscall"
)

// Mem reads typed values from a xemu process's /proc/<pid>/mem.
// Methods ending in At take a pre-computed host virtual address (int64).
// Methods without At take a guest VA >= 0x80000000 and compute the host VA via base.
type Mem struct {
	fd   int
	base uint64   // host VA of Xbox RAM start (gpa2hva 0x0)
	file *os.File // held to prevent GC from closing the fd
}

// Base returns the host VA of guest physical 0x0 established at Init time.
func (m *Mem) Base() uint64 { return m.base }

// HighGVA computes the host virtual address for a guest VA >= 0x80000000.
func (m *Mem) HighGVA(gva uint32) int64 {
	return int64(m.base) + int64(gva-0x80000000)
}

func (m *Mem) readAt(hva int64, buf []byte) error {
	n, err := syscall.Pread(m.fd, buf, hva)
	if err != nil {
		return fmt.Errorf("pread at 0x%x: %w", hva, err)
	}
	if n != len(buf) {
		return fmt.Errorf("short read at 0x%x: got %d/%d", hva, n, len(buf))
	}
	return nil
}

// ReadBytesAt reads n raw bytes from a host VA.
func (m *Mem) ReadBytesAt(hva int64, n int) ([]byte, error) {
	buf := make([]byte, n)
	return buf, m.readAt(hva, buf)
}

// ReadBytes reads n raw bytes from a high guest VA.
func (m *Mem) ReadBytes(gva uint32, n int) ([]byte, error) {
	return m.ReadBytesAt(m.HighGVA(gva), n)
}

func (m *Mem) ReadU8At(hva int64) (uint8, error) {
	var buf [1]byte
	return buf[0], m.readAt(hva, buf[:])
}

func (m *Mem) ReadU16At(hva int64) (uint16, error) {
	var buf [2]byte
	if err := m.readAt(hva, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf[:]), nil
}

func (m *Mem) ReadU32At(hva int64) (uint32, error) {
	var buf [4]byte
	if err := m.readAt(hva, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func (m *Mem) ReadS16At(hva int64) (int16, error) {
	v, err := m.ReadU16At(hva)
	return int16(v), err
}

func (m *Mem) ReadS32At(hva int64) (int32, error) {
	v, err := m.ReadU32At(hva)
	return int32(v), err
}

func (m *Mem) ReadF32At(hva int64) (float32, error) {
	var buf [4]byte
	if err := m.readAt(hva, buf[:]); err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(buf[:])), nil
}

// High-GVA convenience methods (guest VA must be >= 0x80000000).

func (m *Mem) ReadU8(gva uint32) (uint8, error)    { return m.ReadU8At(m.HighGVA(gva)) }
func (m *Mem) ReadU16(gva uint32) (uint16, error)  { return m.ReadU16At(m.HighGVA(gva)) }
func (m *Mem) ReadU32(gva uint32) (uint32, error)  { return m.ReadU32At(m.HighGVA(gva)) }
func (m *Mem) ReadS16(gva uint32) (int16, error)   { return m.ReadS16At(m.HighGVA(gva)) }
func (m *Mem) ReadS32(gva uint32) (int32, error)   { return m.ReadS32At(m.HighGVA(gva)) }
func (m *Mem) ReadF32(gva uint32) (float32, error) { return m.ReadF32At(m.HighGVA(gva)) }
