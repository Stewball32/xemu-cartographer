package podman

// Ports holds the allocated port numbers for an xemu + browser container pair.
type Ports struct {
	XemuHTTP   int `json:"xemu_http"`
	XemuHTTPS  int `json:"xemu_https"`
	XemuWS     int `json:"xemu_ws"`
	BrowserWeb int `json:"browser_web"`
	BrowserVNC int `json:"browser_vnc"`
}

// AllocatePorts returns port assignments for the given index using a base port
// and stride. Each index occupies a stride-wide slot:
//
//	+0  xemu HTTP (CUSTOM_PORT)
//	+1  xemu HTTPS (CUSTOM_HTTPS_PORT)
//	+2  xemu WS (CUSTOM_WS_PORT)
//	+3  browser web (WEB_LISTENING_PORT)
//	+4  browser VNC (VNC_LISTENING_PORT)
func AllocatePorts(base, stride, index int) Ports {
	start := base + stride*index
	return Ports{
		XemuHTTP:   start,
		XemuHTTPS:  start + 1,
		XemuWS:     start + 2,
		BrowserWeb: start + 3,
		BrowserVNC: start + 4,
	}
}
