package lansaves

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// buildResponse is the preview/validate payload: the generated SaveSet
// metadata (file names/sizes/sha1, re-parsed fields, digest status, warnings)
// plus the FATX on-disk footprint. No file bytes are returned — the editor
// uses this to validate and preview before triggering an actual download.
type buildResponse struct {
	*halosave.SaveSet
	TotalBytes     int    `json:"total_bytes"`     // raw sum of file sizes
	FootprintBytes uint64 `json:"footprint_bytes"` // FATX cluster-rounded on-disk size
	FATXCluster    int    `json:"fatx_cluster"`
}

// makeBuildResponse runs the generator and wraps the result. Round-trip
// validation is inherent: halosave.Build re-parses every generated payload and
// returns the decoded view in SaveSet.Parsed, so a structurally invalid file
// fails here rather than on the Xbox.
func makeBuildResponse(req halosave.BuildRequest, clusterSize int) (*buildResponse, error) {
	set, err := halosave.Build(req)
	if err != nil {
		return nil, err
	}
	sizes := make([]int, len(set.Files))
	for i, f := range set.Files {
		sizes[i] = f.Size
	}
	if clusterSize <= 0 {
		clusterSize = cfg.FATXCluster
	}
	return &buildResponse{
		SaveSet:        set,
		TotalBytes:     set.TotalBytes(),
		FootprintBytes: diskspace.FATXFootprint(sizes, clusterSize),
		FATXCluster:    clusterSize,
	}, nil
}

// POST /api/lan/saves/build — JSON BuildRequest in, JSON metadata out. The
// preview/validate endpoint the editors call on every form change.
func init() {
	register(func() {
		Group.POST("/build", func(e *core.RequestEvent) error {
			var body jsonBody
			if err := e.BindBody(&body); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
			}
			req, tr := specFromBody(body)
			resp, err := makeBuildResponse(req, tr.ClusterSize)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return e.JSON(http.StatusOK, resp)
		})

		// GET form of build, for quick checks / the nxdk client to pre-flight a
		// spec without downloading (same metadata, params via query string).
		Group.GET("/build", func(e *core.RequestEvent) error {
			req, tr := specFromQuery(e.Request.URL.Query())
			resp, err := makeBuildResponse(req, tr.ClusterSize)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return e.JSON(http.StatusOK, resp)
		})
	})
}
