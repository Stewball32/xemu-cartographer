package podman

import (
	"os"
	"strconv"
)

// LoadFromEnv builds a Config from CONTAINERS_* environment variables, falling
// back to legacy TOML defaults for any value not set.
func LoadFromEnv() Config {
	return Config{
		Enabled:          envBool("CONTAINERS_ENABLED", false),
		SocketDir:        envStr("CONTAINERS_SOCKET_DIR", "./containers/xemu/qmp"),
		SharedDir:        envStr("CONTAINERS_SHARED_DIR", "./containers/xemu/shared"),
		InitDir:          envStr("CONTAINERS_INIT_DIR", "./containers/xemu/init"),
		ConfigsDir:       envStr("CONTAINERS_CONFIGS_DIR", "./containers/xemu/configs"),
		BrowserDir:       envStr("CONTAINERS_BROWSER_DIR", "./containers/browser"),
		BrowserInitDir:   envStr("CONTAINERS_BROWSER_INIT_DIR", "./containers/browser/init"),
		PortBase:         envInt("CONTAINERS_PORT_BASE", 3100),
		PortStride:       envInt("CONTAINERS_PORT_STRIDE", 10),
		StateFile:        envStr("CONTAINERS_STATE_FILE", "./containers/state.json"),
		HostIP:           envStr("CONTAINERS_HOST_IP", "localhost"),
		PodmanCmd:        envStr("CONTAINERS_PODMAN_CMD", "podman"),
		Encoder:          envStr("CONTAINERS_ENCODER", "x264enc"),
		Framerate:        envInt("CONTAINERS_FRAMERATE", 60),
		CRF:              envInt("CONTAINERS_CRF", 20),
		Width:            envInt("CONTAINERS_WIDTH", 960),
		Height:           envInt("CONTAINERS_HEIGHT", 720),
		PixelfluxWayland: envBool("CONTAINERS_PIXELFLUX_WAYLAND", true),
		DRINode:          envStr("CONTAINERS_DRINODE", "/dev/dri/renderD128"),
		ShmSize:          envStr("CONTAINERS_SHM_SIZE", "1g"),
		BrowserShmSize:   envStr("CONTAINERS_BROWSER_SHM_SIZE", "2gb"),
	}
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
