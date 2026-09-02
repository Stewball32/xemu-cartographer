package scraper

import (
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// A registered title whose game has no baseline set must bind as an error,
// never a panic, and must not invoke the factory — that is the runner-killing
// path this guard replaced (offsets.Baseline used to panic on unknown games).
func TestNewReaderForTitleErrorsWithoutBaseline(t *testing.T) {
	const titleID = 0xDEADBEEF
	if _, ok := registry[titleID]; ok {
		t.Fatalf("title 0x%08X already registered; pick another sentinel", titleID)
	}
	called := 0
	Register(titleID, "no-such-game", func(*xemu.Instance, string, *offsets.Set) (GameReader, error) {
		called++
		return nil, nil
	})
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, titleID)
		registryMu.Unlock()
	})

	for _, explicit := range []string{"", "no-such-set"} {
		r, err := NewReaderForTitle(titleID, nil, "unit", explicit)
		if err == nil || r != nil {
			t.Fatalf("explicit=%q: NewReaderForTitle = (%v, %v), want (nil, error)", explicit, r, err)
		}
		if !strings.Contains(err.Error(), "no baseline set") {
			t.Errorf("explicit=%q: err = %v, want the missing-baseline cause", explicit, err)
		}
	}
	if called != 0 {
		t.Errorf("factory called %d times with no set to bind", called)
	}
}
