package lansaves

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// Download endpoints — generate a save and serve it, gated by a disk-space
// pre-flight so we never hand a LAN client a save its target FATX partition
// cannot hold (a half-written save corrupts the FATX directory).
//
//	GET  /api/lan/saves/download?title=ce&kind=gametype&engine=slayer&name=TS+25
//	      &score_limit=25&time_minutes=7&free_bytes=33554432&format=tar
//	POST /api/lan/saves/download   (JSON body = BuildRequest + free_bytes/format)
//
// The nxdk client uses the GET form; the browser editor uses POST (fetch+blob).
func init() {
	register(func() {
		Group.GET("/download", func(e *core.RequestEvent) error {
			req, tr := specFromQuery(e.Request.URL.Query())
			return serveDownload(e, req, tr)
		})
		Group.POST("/download", func(e *core.RequestEvent) error {
			var body jsonBody
			if err := e.BindBody(&body); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
			}
			req, tr := specFromBody(body)
			return serveDownload(e, req, tr)
		})
	})
}

func serveDownload(e *core.RequestEvent, req halosave.BuildRequest, tr transport) error {
	set, err := halosave.Build(req)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// FATX on-disk footprint of the WHOLE save dir (payload + SaveMeta), since
	// the client writes the whole dir regardless of which file(s) it pulls.
	clusterSize := tr.ClusterSize
	if clusterSize <= 0 {
		clusterSize = cfg.FATXCluster
	}
	sizes := make([]int, len(set.Files))
	for i, f := range set.Files {
		sizes[i] = f.Size
	}
	footprint := diskspace.FATXFootprint(sizes, clusterSize)

	// 1. Target (Xbox) check: the client reports its free FATX bytes.
	if tr.HasFree && tr.FreeBytes != nil && !diskspace.Fits(footprint, *tr.FreeBytes) {
		return e.JSON(http.StatusInsufficientStorage, map[string]any{
			"error":           "insufficient storage on target",
			"footprint_bytes": footprint,
			"available_bytes": *tr.FreeBytes,
			"fatx_cluster":    clusterSize,
			"fatx_dir":        set.FatxDir,
		})
	}
	// 2. Server staging check: don't generate into a server that is itself out
	//    of space. Best-effort — a statfs error is logged, not fatal.
	if free, ferr := diskspace.Free(cfg.StagingDir); ferr != nil {
		log.Printf("lansaves: staging-dir statfs %q: %v", cfg.StagingDir, ferr)
	} else if !diskspace.Fits(footprint, free) {
		return e.JSON(http.StatusInsufficientStorage, map[string]any{
			"error":           "insufficient storage on server staging area",
			"footprint_bytes": footprint,
			"available_bytes": free,
			"staging_dir":     cfg.StagingDir,
		})
	}

	data, contentType, filename, err := selectFormat(set, tr)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	h := e.Response.Header()
	h.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	h.Set("X-Fatx-Dir", set.FatxDir)
	h.Set("X-Fatx-Footprint-Bytes", strconv.FormatUint(footprint, 10))
	h.Set("X-Save-Title", set.Title)
	h.Set("X-Save-Kind", set.Kind)
	h.Set("X-Digest-Mode", string(set.Digest.Mode))
	h.Set("X-Digest-Edited", strconv.FormatBool(set.Digest.Edited))
	h.Set("X-Digest-Resolved", strconv.FormatBool(set.Digest.Resolved))
	return e.Blob(http.StatusOK, contentType, data)
}

// selectFormat resolves the transport format into the bytes to serve, the
// content type, and a download filename. Defaults to a tar of the whole save
// dir (the complete, ready-to-unpack unit).
func selectFormat(set *halosave.SaveSet, tr transport) (data []byte, contentType, filename string, err error) {
	switch strings.ToLower(tr.Format) {
	case "", "tar":
		b, e := archiveTar(set)
		return b, "application/x-tar", set.DirName + ".tar", e
	case "zip":
		b, e := archiveZip(set)
		return b, "application/zip", set.DirName + ".zip", e
	case "payload":
		f := set.Files[0] // payload is always the first file
		return f.Data, "application/octet-stream", f.Name, nil
	case "savemeta":
		if f := fileByName(set, "SaveMeta.xbx"); f != nil {
			return f.Data, "application/octet-stream", f.Name, nil
		}
		return nil, "", "", errNoFile("SaveMeta.xbx")
	case "file":
		if f := fileByName(set, tr.File); f != nil {
			return f.Data, "application/octet-stream", f.Name, nil
		}
		return nil, "", "", errNoFile(tr.File)
	default:
		return nil, "", "", errBadFormat(tr.Format)
	}
}

func fileByName(set *halosave.SaveSet, name string) *halosave.SaveFile {
	for i := range set.Files {
		if set.Files[i].Name == name {
			return &set.Files[i]
		}
	}
	return nil
}

type downloadError struct{ msg string }

func (d downloadError) Error() string { return d.msg }

func errNoFile(name string) error { return downloadError{"no such file in save set: " + name} }
func errBadFormat(f string) error {
	return downloadError{"unknown format: " + f + " (want tar|zip|payload|savemeta|file)"}
}
