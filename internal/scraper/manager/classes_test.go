package manager

import (
	"reflect"
	"testing"

	"github.com/xemu-cartographer/xc-scraper/wire"
)

// TestHelloClassesAreTheSharedRegistry: hello's handshake Classes list is a
// COPY of allClasses — same contents, distinct backing array — so the two
// surfaces can't drift and a payload consumer can't mutate the registry.
func TestHelloClassesAreTheSharedRegistry(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	p := m.BuildHelloPayload()
	if !reflect.DeepEqual(p.Classes, allClasses) {
		t.Fatalf("hello classes = %v, want allClasses %v", p.Classes, allClasses)
	}
	if len(p.Classes) > 0 && &p.Classes[0] == &allClasses[0] {
		t.Fatal("hello classes share allClasses' backing array — must be a copy")
	}
}

// TestClassRegistryIsTheWireRegistry: the manager's allClasses and its
// envelopeType* constants are derived from / re-declared against the wire
// contract package. Pin both so a class added on either side without the
// other is caught, and so the order hello announces stays the wire order.
func TestClassRegistryIsTheWireRegistry(t *testing.T) {
	if !reflect.DeepEqual(allClasses, wire.AllClasses()) {
		t.Fatalf("allClasses = %v, want wire.AllClasses() %v", allClasses, wire.AllClasses())
	}
	want := map[string]string{
		envelopeTypeXbox:          wire.ClassXbox,
		envelopeTypeScenario:      wire.ClassScenario,
		envelopeTypeGame:          wire.ClassGame,
		envelopeTypeGameFiltered:  wire.ClassGameFiltered,
		envelopeTypeTick:          wire.ClassTick,
		envelopeTypeObjects:       wire.ClassObjects,
		envelopeTypeDebug:         wire.ClassDebug,
		envelopeTypeSummary:       wire.ClassSummary,
		envelopeTypePreviousGame:  wire.ClassPreviousGame,
		envelopeTypeEvent:         wire.ClassEvent,
		envelopeTypeEventFiltered: wire.ClassEventFiltered,
		envelopeTypeHello:         wire.ClassHello,
		envelopeTypeEvents:        wire.ClassEvents,
		envelopeTypeProbe:         wire.ClassProbe,
	}
	for have, wire := range want {
		if have != wire {
			t.Errorf("envelopeType constant %q != wire class %q", have, wire)
		}
	}
	// Every announced class has a producer-side constant; the reply-only
	// kinds do not appear in the announced list.
	for _, class := range allClasses {
		if _, ok := want[class]; !ok {
			t.Errorf("announced class %q has no envelopeType* constant", class)
		}
		switch class {
		case envelopeTypeHello, envelopeTypeEvents, envelopeTypeProbe:
			t.Errorf("reply-only kind %q must not be announced", class)
		}
	}
	// The join-replay order in classEnvelopeMessages is the wire's
	// StateClasses order — pin the sequence the runner replays.
	if got, want := wire.StateClasses(), []string{
		envelopeTypeXbox, envelopeTypeScenario, envelopeTypeGame, envelopeTypeGameFiltered,
		envelopeTypePreviousGame, envelopeTypeTick, envelopeTypeObjects, envelopeTypeDebug,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wire.StateClasses() = %v, want replay order %v", got, want)
	}
}
