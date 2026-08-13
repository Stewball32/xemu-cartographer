package podman

import "testing"

func TestParsePodmanStatus(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "running", "running"},
		{"trailing newline", "running\n", "running"},
		{"json quoted", `"running"`, "running"},
		{"json quoted with newline", "\"exited\"\n", "exited"},
		{"whitespace", "  created  ", "created"},
		{"empty", "", ""},
		{"created", "created\n", "created"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parsePodmanStatus([]byte(c.in)); got != c.want {
				t.Errorf("parsePodmanStatus(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}
