package lansaves

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// transport carries the non-content download parameters: the target free space
// to check against, which file(s)/format to serve, and an optional FATX cluster
// override for the footprint math.
type transport struct {
	FreeBytes   *uint64 // target (Xbox) free bytes; nil = client did not report
	HasFree     bool
	Format      string // "tar" | "zip" | "payload" | "savemeta" | "file"
	File        string // for Format=="file": the exact filename
	ClusterSize int    // FATX cluster override; 0 = default
}

// jsonBody is the POST shape: a BuildRequest plus the transport fields. The
// embedded BuildRequest's json tags are promoted, so a single flat JSON object
// like {"title":"ce","kind":"gametype","name":"TS 25","free_bytes":8388608}
// binds correctly.
type jsonBody struct {
	halosave.BuildRequest
	FreeBytes   *uint64 `json:"free_bytes"`
	Format      string  `json:"format"`
	File        string  `json:"file"`
	ClusterSize int     `json:"cluster_size"`
}

// specFromQuery maps URL query params onto a BuildRequest + transport. Used by
// the GET form (the nxdk LAN client). Unknown params are ignored.
//
//	title, kind, name, internal_name, dir_name, engine    (strings)
//	teams, radar, recompute                                (bools: 1/true/yes/on)
//	options, scoring_subtype, time_limit, time_limit2,
//	  score_limit, option2, respawn, engine_union          (uint32)
//	time_minutes                                           (float)
//	app_<key>=<int>                                        (H2 appearance bytes)
//	free_bytes (uint64), format, file, cluster_size (int)
func specFromQuery(q url.Values) (halosave.BuildRequest, transport) {
	req := halosave.BuildRequest{
		Title:        q.Get("title"),
		Kind:         q.Get("kind"),
		Name:         q.Get("name"),
		InternalName: q.Get("internal_name"),
		DirName:      q.Get("dir_name"),
		Engine:       q.Get("engine"),
	}
	req.Teams = boolPtr(q, "teams")
	req.Radar = boolPtr(q, "radar")
	if b := boolPtr(q, "recompute"); b != nil {
		req.Recompute = *b
	}
	req.Options = u32Ptr(q, "options")
	req.ScoringSubtype = u32Ptr(q, "scoring_subtype")
	req.TimeLimit = u32Ptr(q, "time_limit")
	req.TimeLimit2 = u32Ptr(q, "time_limit2")
	req.ScoreLimit = u32Ptr(q, "score_limit")
	req.Option2 = u32Ptr(q, "option2")
	req.Respawn = u32Ptr(q, "respawn")
	req.EngineUnion = u32Ptr(q, "engine_union")
	req.TimeMinutes = floatPtr(q, "time_minutes")

	for key := range q {
		if strings.HasPrefix(key, "app_") {
			if v, err := strconv.Atoi(q.Get(key)); err == nil {
				if req.Appearance == nil {
					req.Appearance = map[string]int{}
				}
				req.Appearance[strings.TrimPrefix(key, "app_")] = v
			}
		}
	}

	tr := transport{
		Format:      q.Get("format"),
		File:        q.Get("file"),
		ClusterSize: atoiDefault(q.Get("cluster_size"), 0),
	}
	if fb := u64Ptr(q, "free_bytes"); fb != nil {
		tr.FreeBytes = fb
		tr.HasFree = true
	}
	return req, tr
}

// specFromBody maps a decoded jsonBody onto a BuildRequest + transport.
func specFromBody(b jsonBody) (halosave.BuildRequest, transport) {
	tr := transport{Format: b.Format, File: b.File, ClusterSize: b.ClusterSize}
	if b.FreeBytes != nil {
		tr.FreeBytes = b.FreeBytes
		tr.HasFree = true
	}
	return b.BuildRequest, tr
}

func boolPtr(q url.Values, key string) *bool {
	if !q.Has(key) {
		return nil
	}
	switch strings.ToLower(q.Get(key)) {
	case "1", "true", "yes", "on":
		t := true
		return &t
	default:
		f := false
		return &f
	}
}

func u32Ptr(q url.Values, key string) *uint32 {
	if !q.Has(key) {
		return nil
	}
	// base 0 auto-detects a 0x prefix (handy for the options bitfield / union).
	v, err := strconv.ParseUint(q.Get(key), 0, 32)
	if err != nil {
		return nil
	}
	u := uint32(v)
	return &u
}

func u64Ptr(q url.Values, key string) *uint64 {
	if !q.Has(key) {
		return nil
	}
	v, err := strconv.ParseUint(q.Get(key), 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func floatPtr(q url.Values, key string) *float64 {
	if !q.Has(key) {
		return nil
	}
	v, err := strconv.ParseFloat(q.Get(key), 64)
	if err != nil {
		return nil
	}
	return &v
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}
