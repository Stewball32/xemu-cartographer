// Package lansync holds the server-side helpers for the LAN box-provisioning
// contract (SPEC §4): turning uploaded game ISOs / app zips into the extracted,
// measured, downloadable artifacts the manifest advertises. It is the shared
// seam between the extraction hooks (internal/pocketbase/hooks), the seed
// scenario (internal/pocketbase/seed), and the download routes
// (internal/pocketbase/routes/lansync) — pure filesystem + exec, no PocketBase
// types, so each caller reads/writes records itself.
package lansync

import (
	"os"
	"strconv"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
)

// Config resolves the host paths + client-facing directory names the LAN-sync
// machinery needs. Loaded once from env by Load(); mirrors the podman ISODir so
// a single library dir serves both the emulator and the box-provisioning path.
type Config struct {
	// ISODir is the shared game-ISO library (bare `isos.filename` resolves
	// here). Matches podman Config.ISODir so uploads land in one place.
	ISODir string
	// ExtractDir is where extracted disc trees are cached (evictable). Each ISO
	// gets <ExtractDir>/isos/<recordID>/.
	ExtractDir string
	// FATXCluster is the cluster size for the footprint (drive-fill) math.
	FATXCluster int
	// HaloDir / AppsDir are the client-side destination roots the manifest's
	// dest_dir is expressed against (SPEC §3.1 defaults \Halo / \Apps).
	HaloDir string
	AppsDir string
	// ExtractXISOCmd / UnzipCmd are the tools shelled for extraction; overridable.
	ExtractXISOCmd string
}

// Load reads the LAN-sync config from the environment, applying defaults that
// match the rest of the project (podman ISODir, FATX cluster, client dir names).
func Load() Config {
	return Config{
		ISODir:         envDefault("CONTAINERS_ISO_DIR", "./containers/xemu/shared/isos"),
		ExtractDir:     envDefault("LAN_SYNC_EXTRACT_DIR", "./containers/xemu/shared/extracted"),
		FATXCluster:    envInt("LAN_SAVES_FATX_CLUSTER", diskspace.DefaultFATXCluster),
		HaloDir:        envDefault("LAN_SYNC_HALO_DIR", `\Halo`),
		AppsDir:        envDefault("LAN_SYNC_APPS_DIR", `\Apps`),
		ExtractXISOCmd: envDefault("LAN_SYNC_EXTRACT_XISO_CMD", "extract-xiso"),
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}
