package podman

import "testing"

func TestAllocatePorts(t *testing.T) {
	tests := []struct {
		base, stride, index int
		want                Ports
	}{
		{3100, 10, 0, Ports{3100, 3101, 3102, 3103, 3104}},
		{3100, 10, 1, Ports{3110, 3111, 3112, 3113, 3114}},
		{3100, 10, 5, Ports{3150, 3151, 3152, 3153, 3154}},
		{4000, 20, 0, Ports{4000, 4001, 4002, 4003, 4004}},
		{4000, 20, 3, Ports{4060, 4061, 4062, 4063, 4064}},
	}

	for _, tt := range tests {
		got := AllocatePorts(tt.base, tt.stride, tt.index)
		if got != tt.want {
			t.Errorf("AllocatePorts(%d, %d, %d) = %+v, want %+v",
				tt.base, tt.stride, tt.index, got, tt.want)
		}
	}
}

func TestStateNextIndex(t *testing.T) {
	s := &State{Containers: map[string]*ContainerInfo{
		"a": {Index: 0},
		"b": {Index: 2},
		"c": {Index: 3},
	}}

	// Index 1 is the first gap.
	if got := s.NextIndex(); got != 1 {
		t.Errorf("NextIndex() = %d, want 1", got)
	}

	// Fill the gap.
	s.Containers["d"] = &ContainerInfo{Index: 1}
	if got := s.NextIndex(); got != 4 {
		t.Errorf("NextIndex() = %d, want 4", got)
	}
}
