package xemu

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

// notReadyErr builds a not-ready error exactly the way the real path does —
// fmt.Errorf(...%w) around ErrNotMapped, produced by the actual parser — so the
// gate tests exercise real wrapping rather than a hand-rolled stand-in.
func notReadyErr() error {
	_, err := parseHexSuffix("Unmapped")
	return err
}

// The attach-too-early root cause: xemu's QMP socket exists seconds before the
// guest has page tables, so translations answer with prose instead of an
// address. That must be a typed NOT-READY signal we can wait on — never a
// strconv crash, which is what killed scraper attach:
//
//	translate 0x10000: gva2gpa 0x10000: parse "Unmapped": strconv.ParseUint: …
func TestParseHexSuffixNotMapped(t *testing.T) {
	notReady := []string{
		"Unmapped",
		"  Unmapped\n",
		"No memory is mapped at address 0x10000",
		// Classified by "the monitor gave us no number", so a reworded or new
		// qemu message still degrades to a retry instead of a hard failure.
		"some future qemu wording",
	}
	for _, s := range notReady {
		v, err := parseHexSuffix(s)
		if err == nil {
			t.Errorf("parseHexSuffix(%q) = %d, want error", s, v)
			continue
		}
		if !errors.Is(err, ErrNotMapped) {
			t.Errorf("parseHexSuffix(%q): error is %v, want ErrNotMapped", s, err)
		}
		// Must NOT leak a strconv error — that's the crash we're fixing.
		var numErr *strconv.NumError
		if errors.As(err, &numErr) {
			t.Errorf("parseHexSuffix(%q) leaked strconv.NumError: %v", s, err)
		}
	}

	// Normal responses still parse — no regression.
	ok := map[string]uint64{
		"Physical address for 0x10000 is 0x3f10000":            0x3f10000,
		"Host virtual address for 0x0 (ram) is 0x7f0000000000": 0x7f0000000000,
		"0x1234": 0x1234,
	}
	for s, want := range ok {
		got, err := parseHexSuffix(s)
		if err != nil {
			t.Errorf("parseHexSuffix(%q) errored: %v", s, err)
			continue
		}
		if got != want {
			t.Errorf("parseHexSuffix(%q) = 0x%x, want 0x%x", s, got, want)
		}
	}

	// An empty response is a protocol problem, not a not-ready signal.
	if _, err := parseHexSuffix("  "); err == nil || errors.Is(err, ErrNotMapped) {
		t.Errorf("empty response should be a plain error, got %v", err)
	}
}

// The attach-when-ready gate: keep retrying while the guest reports not-ready,
// then proceed once it IS ready — so an early attach waits instead of failing
// or binding translations against unsettled memory.
func TestRetryWhileNotMappedWaitsThenSucceeds(t *testing.T) {
	calls := 0
	err := retryWhileNotMapped(context.Background(), func() error {
		calls++
		if calls < 4 {
			return notReadyErr()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success once the guest is ready, got %v", err)
	}
	if calls != 4 {
		t.Errorf("attempts = %d, want 4 (3 not-ready, then ready)", calls)
	}
}

// A real failure (dead process, missing socket) must fail FAST — waiting can't
// fix it, and blocking attach for the full budget would hide the cause.
func TestRetryWhileNotMappedFailsFastOnRealError(t *testing.T) {
	boom := errors.New("no such file or directory")
	calls := 0
	start := time.Now()
	err := retryWhileNotMapped(context.Background(), func() error {
		calls++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the underlying failure", err)
	}
	if calls != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on a real error)", calls)
	}
	if time.Since(start) > time.Second {
		t.Error("should not have backed off on a real error")
	}
}

// A guest that never becomes ready gives up on ctx, reporting the last
// not-ready error rather than a bare context error, so the log says why.
func TestRetryWhileNotMappedGivesUpOnContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	calls := 0
	err := retryWhileNotMapped(ctx, func() error {
		calls++
		return notReadyErr()
	})
	if err == nil {
		t.Fatal("expected an error when the guest never becomes ready")
	}
	if !errors.Is(err, ErrNotMapped) {
		t.Errorf("give-up error should still be ErrNotMapped, got %v", err)
	}
	if calls < 2 {
		t.Errorf("should have retried at least once before giving up, got %d", calls)
	}
}

// Normal attach — ready on the first try — must not sleep or retry at all.
func TestRetryWhileNotMappedNoRegressionWhenReady(t *testing.T) {
	calls := 0
	start := time.Now()
	if err := retryWhileNotMapped(context.Background(), func() error { calls++; return nil }); err != nil {
		t.Fatalf("ready-first-try should succeed: %v", err)
	}
	if calls != 1 {
		t.Errorf("attempts = %d, want exactly 1", calls)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Error("ready-first-try must not back off")
	}
}

// Init must not destroy working translations when a later address is still
// unmapped mid-boot — it commits the new set only on full success.
func TestInitKeepsPriorTranslationsOnPartialFailure(t *testing.T) {
	inst := &Instance{Name: "t", lowHVAs: map[uint32]int64{0x10000: 0xdead}}
	// A translate failure returns before inst.lowHVAs is reassigned, so the
	// previously-good mapping survives for the next retry to build on.
	if _, err := inst.LowHVA(0x10000); err != nil {
		t.Fatalf("precondition: %v", err)
	}
	inst.QMPSock = "/nonexistent/qmp.sock" // Init will fail early (no socket)
	if err := inst.Init([]uint32{0x10000, 0x2E4068}); err == nil {
		t.Fatal("expected Init to fail with no socket")
	}
	if hva, err := inst.LowHVA(0x10000); err != nil || hva != 0xdead {
		t.Errorf("prior translation lost after failed Init: hva=0x%x err=%v", hva, err)
	}
}
