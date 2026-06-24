package diskspace

import (
	"os"
	"testing"
)

func TestFATXFootprintRoundsToClusters(t *testing.T) {
	cs := DefaultFATXCluster // 16384
	cases := []struct {
		name  string
		sizes []int
		want  uint64
	}{
		// A CE save dir: blam.lst(512) + SaveMeta(26) -> 2 file clusters + 1 dir
		// cluster = 3 clusters = 48 KiB.
		{"ce-gametype", []int{512, 26}, 3 * uint64(cs)},
		// Exactly one cluster boundary stays one cluster; one byte over spills.
		{"exact-cluster", []int{cs}, 2 * uint64(cs)},     // file 1 cluster + dir 1
		{"one-over", []int{cs + 1}, 3 * uint64(cs)},      // file 2 clusters + dir 1
		{"empty-files-skipped", []int{0, 0}, uint64(cs)}, // just the dir cluster
		{"h2-profile", []int{500, 42}, 3 * uint64(cs)},   // profile + savemeta + dir
	}
	for _, c := range cases {
		if got := FATXFootprint(c.sizes, cs); got != c.want {
			t.Errorf("%s: footprint = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestFATXFootprintDefaultsCluster(t *testing.T) {
	if FATXFootprint([]int{1}, 0) != 2*DefaultFATXCluster {
		t.Errorf("clusterSize<=0 should default to DefaultFATXCluster")
	}
}

func TestFits(t *testing.T) {
	if !Fits(100, 100) {
		t.Error("equal should fit")
	}
	if Fits(101, 100) {
		t.Error("over should not fit")
	}
	if !Fits(0, 0) {
		t.Error("zero into zero should fit")
	}
}

func TestFreeOnTempDir(t *testing.T) {
	free, err := Free(os.TempDir())
	if err != nil {
		t.Fatalf("Free: %v", err)
	}
	if free == 0 {
		t.Errorf("expected non-zero free space on %s", os.TempDir())
	}
}

func TestFreeMissingPath(t *testing.T) {
	if _, err := Free("/no/such/path/really/unlikely/to/exist/xyz"); err == nil {
		t.Error("expected error for missing path")
	}
}
