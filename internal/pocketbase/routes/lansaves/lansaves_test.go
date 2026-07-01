package lansaves

import (
	"archive/tar"
	"bytes"
	"net/url"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

func TestLanAccessAllowed(t *testing.T) {
	cases := []struct {
		name     string
		env      string
		provided string
		admin    bool
		want     bool
	}{
		{"admin always", "secret", "", true, true},
		{"admin no token configured", "", "", true, true},
		{"open when unconfigured", "", "", false, true},
		{"correct token", "secret", "secret", false, true},
		{"wrong token", "secret", "nope", false, false},
		{"missing token when required", "secret", "", false, false},
	}
	for _, c := range cases {
		if got, _ := lanAccessAllowed(c.env, c.provided, c.admin); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestSpecFromQueryCE(t *testing.T) {
	q, _ := url.ParseQuery("title=ce&kind=gametype&engine=slayer&name=TS+25&score_limit=25&time_minutes=7&teams=1&radar=true&options=0x23&free_bytes=33554432&format=tar")
	req, tr := specFromQuery(q)
	if req.Title != "ce" || req.Kind != "gametype" || req.Engine != "slayer" || req.Name != "TS 25" {
		t.Fatalf("req strings: %+v", req)
	}
	if req.ScoreLimit == nil || *req.ScoreLimit != 25 {
		t.Errorf("score_limit not parsed")
	}
	if req.TimeMinutes == nil || *req.TimeMinutes != 7 {
		t.Errorf("time_minutes not parsed")
	}
	if req.Teams == nil || !*req.Teams {
		t.Errorf("teams not parsed")
	}
	if req.Radar == nil || !*req.Radar {
		t.Errorf("radar not parsed")
	}
	if req.Options == nil || *req.Options != 0x23 {
		t.Errorf("options hex not parsed: %v", req.Options)
	}
	if !tr.HasFree || tr.FreeBytes == nil || *tr.FreeBytes != 33554432 {
		t.Errorf("free_bytes not parsed")
	}
	if tr.Format != "tar" {
		t.Errorf("format = %q", tr.Format)
	}
}

func TestSpecFromQueryH2Appearance(t *testing.T) {
	q, _ := url.ParseQuery("title=h2&kind=profile&name=CARTOG&app_armor_primary=7&app_emblem_foreground=3")
	req, _ := specFromQuery(q)
	if req.Appearance["armor_primary"] != 7 || req.Appearance["emblem_foreground"] != 3 {
		t.Errorf("appearance not parsed: %+v", req.Appearance)
	}
}

func TestArchiveTarLayout(t *testing.T) {
	set, err := halosave.Build(halosave.BuildRequest{Title: "ce", Kind: "gametype", Name: "TS 25", Engine: "slayer"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := archiveTar(set)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(data))
	got := map[string]int64{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		got[h.Name] = h.Size
	}
	wantBlam := "UDATA/4d530004/G-TS 25/blam.lst"
	wantMeta := "UDATA/4d530004/G-TS 25/SaveMeta.xbx"
	if got[wantBlam] != 512 {
		t.Errorf("tar missing %q (size 512); entries=%v", wantBlam, got)
	}
	if _, ok := got[wantMeta]; !ok {
		t.Errorf("tar missing %q; entries=%v", wantMeta, got)
	}
}

func TestArchiveZipRoundTrips(t *testing.T) {
	set, _ := halosave.Build(halosave.BuildRequest{Title: "h2", Kind: "profile", Name: "CARTOG"})
	b, err := archiveZip(set)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty zip")
	}
}

func TestSelectFormat(t *testing.T) {
	set, _ := halosave.Build(halosave.BuildRequest{Title: "ce", Kind: "gametype", Name: "TS 25", Engine: "slayer"})
	// payload
	data, ct, fn, err := selectFormat(set, transport{Format: "payload"})
	if err != nil || fn != "blam.lst" || len(data) != 512 || ct != "application/octet-stream" {
		t.Errorf("payload: fn=%q ct=%q len=%d err=%v", fn, ct, len(data), err)
	}
	// savemeta
	_, _, fn, err = selectFormat(set, transport{Format: "savemeta"})
	if err != nil || fn != "SaveMeta.xbx" {
		t.Errorf("savemeta: fn=%q err=%v", fn, err)
	}
	// default = tar
	_, ct, fn, err = selectFormat(set, transport{})
	if err != nil || ct != "application/x-tar" || fn != "G-TS 25.tar" {
		t.Errorf("default: ct=%q fn=%q err=%v", ct, fn, err)
	}
	// bad format
	if _, _, _, err := selectFormat(set, transport{Format: "bogus"}); err == nil {
		t.Errorf("expected error for bad format")
	}
	// file by name
	_, _, fn, err = selectFormat(set, transport{Format: "file", File: "blam.lst"})
	if err != nil || fn != "blam.lst" {
		t.Errorf("file: fn=%q err=%v", fn, err)
	}
	if _, _, _, err := selectFormat(set, transport{Format: "file", File: "nope"}); err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestMakeBuildResponseFootprint(t *testing.T) {
	resp, err := makeBuildResponse(halosave.BuildRequest{Title: "ce", Kind: "gametype", Name: "TS 25", Engine: "slayer"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 2 files (each <1 cluster) + 1 dir cluster = 3 * 16384.
	if resp.FootprintBytes != 3*16384 {
		t.Errorf("footprint = %d, want %d", resp.FootprintBytes, 3*16384)
	}
	if resp.TotalBytes <= 0 || resp.FATXCluster != 16384 {
		t.Errorf("totals: total=%d cluster=%d", resp.TotalBytes, resp.FATXCluster)
	}
}
