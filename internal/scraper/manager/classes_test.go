package manager

import (
	"reflect"
	"testing"
)

// TestHelloClassesAreTheSharedRegistry: hello's handshake Classes list is a
// COPY of allClasses — same contents, distinct backing array — so the two
// surfaces can't drift and a payload consumer can't mutate the registry.
func TestHelloClassesAreTheSharedRegistry(t *testing.T) {
	m := New(nil)
	defer m.Close()

	p := m.BuildHelloPayload()
	if !reflect.DeepEqual(p.Classes, allClasses) {
		t.Fatalf("hello classes = %v, want allClasses %v", p.Classes, allClasses)
	}
	if len(p.Classes) > 0 && &p.Classes[0] == &allClasses[0] {
		t.Fatal("hello classes share allClasses' backing array — must be a copy")
	}
}
