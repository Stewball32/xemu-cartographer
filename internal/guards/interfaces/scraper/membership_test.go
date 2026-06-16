package scraper

import "testing"

func TestSanitizeIdentity(t *testing.T) {
	cases := map[string]string{
		"Stewie":    "stewie",
		"  STEW  ":  "stew",
		"MixedCase": "mixedcase",
		"":          "",
		"   ":       "",
	}
	for in, want := range cases {
		if got := SanitizeIdentity(in); got != want {
			t.Errorf("SanitizeIdentity(%q) = %q, want %q", in, got, want)
		}
	}
}

func sampleView() []ContainerMembership {
	return []ContainerMembership{
		{Container: "pod-a", Identities: []string{"console-a", "stewie", "red-machine"}},
		{Container: "pod-b", Identities: []string{"console-b", "bluefox"}},
		{Container: "pod-c", Identities: nil},
	}
}

func TestMatchContainer(t *testing.T) {
	view := sampleView()

	tests := []struct {
		name      string
		tags      []string
		wantName  string
		wantFound bool
	}{
		{"player name match", []string{"stewie"}, "pod-a", true},
		{"console name match", []string{"console-b"}, "pod-b", true},
		{"machine name match", []string{"red-machine"}, "pod-a", true},
		{"case insensitive caller tag", []string{"STEWIE"}, "pod-a", true},
		{"first deterministic match wins", []string{"bluefox", "stewie"}, "pod-a", true},
		{"no match", []string{"nobody"}, "", false},
		{"empty tags", nil, "", false},
		{"empty string tag ignored", []string{""}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotFound := MatchContainer(view, tc.tags)
			if gotName != tc.wantName || gotFound != tc.wantFound {
				t.Errorf("MatchContainer(%v) = (%q, %v), want (%q, %v)",
					tc.tags, gotName, gotFound, tc.wantName, tc.wantFound)
			}
		})
	}
}

func TestContainerHasGamertag(t *testing.T) {
	view := sampleView()

	tests := []struct {
		name      string
		container string
		tags      []string
		want      bool
	}{
		{"member of the named container", "pod-a", []string{"stewie"}, true},
		{"member but wrong container", "pod-b", []string{"stewie"}, false},
		{"case-insensitive", "pod-b", []string{"BlueFox"}, true},
		{"unknown container", "pod-z", []string{"stewie"}, false},
		{"container with no identities", "pod-c", []string{"stewie"}, false},
		{"empty tags", "pod-a", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContainerHasGamertag(view, tc.container, tc.tags); got != tc.want {
				t.Errorf("ContainerHasGamertag(%q, %v) = %v, want %v",
					tc.container, tc.tags, got, tc.want)
			}
		})
	}
}
