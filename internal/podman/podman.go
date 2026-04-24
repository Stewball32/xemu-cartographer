// Package podman manages xemu + browser container pairs via the podman CLI.
package podman

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config holds settings for the podman Manager.
type Config struct {
	Enabled        bool
	SocketDir      string
	SharedDir      string
	InitDir        string
	ConfigsDir     string
	BrowserDir     string
	BrowserInitDir string
	PortBase       int
	PortStride     int
	StateFile      string
	HostIP         string // IP/hostname for remote access; defaults to "localhost"

	// PodmanCmd is the command (with optional leading args) used to invoke
	// podman. Default "podman" runs rootless under the server user.
	// Set to e.g. "sudo -n podman" to escalate when device passthrough is
	// required (KVM/DRI/NET_ADMIN); requires a passwordless sudoers rule.
	PodmanCmd string

	// Defaults for container environment variables.
	Encoder          string
	Framerate        int
	CRF              int
	Width            int
	Height           int
	PixelfluxWayland bool
	DRINode          string
	ShmSize          string
	BrowserShmSize   string
}

const (
	xemuImage    = "lscr.io/linuxserver/xemu:latest"
	browserImage = "docker.io/jlesage/firefox"
)

// Manager creates and controls podman containers.
type Manager struct {
	cfg   Config
	state *State
	mu    sync.Mutex
}

// NewManager loads persisted state and returns a ready Manager.
func NewManager(cfg Config) (*Manager, error) {
	state, err := LoadState(cfg.StateFile)
	if err != nil {
		return nil, fmt.Errorf("podman: load state: %w", err)
	}
	return &Manager{cfg: cfg, state: state}, nil
}

// Create provisions a new xemu + browser container pair without starting them.
func (m *Manager) Create(name string) (*ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.state.Containers[name]; exists {
		return nil, fmt.Errorf("container %q already exists", name)
	}

	idx := m.state.NextIndex()
	ports := AllocatePorts(m.cfg.PortBase, m.cfg.PortStride, idx)
	info := &ContainerInfo{
		Name:    name,
		Index:   idx,
		Ports:   ports,
		Created: time.Now(),
	}

	// Ensure per-container config directories exist.
	configDir := filepath.Join(m.cfg.ConfigsDir, name)
	browserCfgDir := filepath.Join(m.cfg.BrowserDir, "config-"+name)
	for _, d := range []string{configDir, browserCfgDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}

	// Generate labwc autostart with QMP enabled (only if it doesn't exist).
	autostartDir := filepath.Join(configDir, ".config", "labwc")
	autostartPath := filepath.Join(autostartDir, "autostart")
	if _, err := os.Stat(autostartPath); os.IsNotExist(err) {
		if err := os.MkdirAll(autostartDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", autostartDir, err)
		}
		qmpArg := fmt.Sprintf("-qmp unix:/qmp/%s.sock,server,nowait", name)
		autostart := fmt.Sprintf("#!/bin/bash\nfoot -e /opt/xemu/AppRun %s\n", qmpArg)
		if err := os.WriteFile(autostartPath, []byte(autostart), 0o755); err != nil {
			return nil, fmt.Errorf("write autostart: %w", err)
		}
	}

	// --- xemu container ---
	if err := m.createXemu(name, ports, configDir); err != nil {
		return nil, fmt.Errorf("create xemu container: %w", err)
	}

	// --- browser container ---
	if err := m.createBrowser(name, ports, browserCfgDir); err != nil {
		// Best-effort cleanup of the xemu container.
		_, _ = m.run("rm", "-f", name)
		return nil, fmt.Errorf("create browser container: %w", err)
	}

	m.state.Containers[name] = info
	if err := m.state.Save(); err != nil {
		log.Printf("podman: warning: failed to save state: %v", err)
	}
	return info, nil
}

func (m *Manager) createXemu(name string, ports Ports, configDir string) error {
	encoder := m.cfg.Encoder
	if encoder == "" {
		encoder = "x264enc"
	}
	framerate := m.cfg.Framerate
	if framerate == 0 {
		framerate = 60
	}
	crf := m.cfg.CRF
	if crf == 0 {
		crf = 20
	}
	width := m.cfg.Width
	if width == 0 {
		width = 960
	}
	height := m.cfg.Height
	if height == 0 {
		height = 720
	}
	driNode := m.cfg.DRINode
	if driNode == "" {
		driNode = "/dev/dri/renderD128"
	}
	shmSize := m.cfg.ShmSize
	if shmSize == "" {
		shmSize = "1g"
	}

	args := []string{
		"create",
		"--name", name,
		"--hostname", name,
		"--network", "host",
		"--device", "/dev/kvm",
		"--device", "/dev/dri",
		"--cap-add", "NET_ADMIN",
		"--cap-add", "NET_RAW",
		"--security-opt", "seccomp=unconfined",
		"--shm-size", shmSize,
		"--restart", "unless-stopped",
		// Environment
		"-e", fmt.Sprintf("CUSTOM_PORT=%d", ports.XemuHTTP),
		"-e", fmt.Sprintf("CUSTOM_HTTPS_PORT=%d", ports.XemuHTTPS),
		"-e", fmt.Sprintf("CUSTOM_WS_PORT=%d", ports.XemuWS),
		"-e", fmt.Sprintf("PIXELFLUX_WAYLAND=%v", m.cfg.PixelfluxWayland),
		"-e", fmt.Sprintf("DRINODE=%s", driNode),
		"-e", fmt.Sprintf("DRI_NODE=%s", driNode),
		"-e", fmt.Sprintf("SELKIES_ENCODER=%s", encoder),
		"-e", "SELKIES_H264_STREAMING_MODE=true",
		"-e", fmt.Sprintf("SELKIES_FRAMERATE=%d", framerate),
		"-e", fmt.Sprintf("SELKIES_H264_CRF=%d", crf),
		"-e", "SELKIES_H264_PAINTOVER_BURST_FRAMES=1",
		"-e", fmt.Sprintf("SELKIES_MANUAL_WIDTH=%d", width),
		"-e", fmt.Sprintf("SELKIES_MANUAL_HEIGHT=%d", height),
		"-e", fmt.Sprintf("MAX_RESOLUTION=%dx%d", width*2, height*2),
		"-e", "PUID=0",
		"-e", "PGID=0",
		"-e", "TZ=America/Los_Angeles",
		// Volumes
		"-v", fmt.Sprintf("%s:/config", abs(configDir)),
		"-v", fmt.Sprintf("%s:/custom-cont-init.d:ro", abs(m.cfg.InitDir)),
		"-v", fmt.Sprintf("%s:/qmp", abs(m.cfg.SocketDir)),
		"-v", fmt.Sprintf("%s:/shared", abs(m.cfg.SharedDir)),
		xemuImage,
	}

	out, err := m.run(args...)
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func (m *Manager) createBrowser(name string, ports Ports, browserCfgDir string) error {
	browserName := name + "-browser"
	shmSize := m.cfg.BrowserShmSize
	if shmSize == "" {
		shmSize = "2gb"
	}
	width := m.cfg.Width
	if width == 0 {
		width = 960
	}
	height := m.cfg.Height
	if height == 0 {
		height = 720
	}
	hostIP := m.cfg.HostIP
	if hostIP == "" {
		hostIP = "localhost"
	}

	// Build the insecure-fallback list: always include localhost, add hostIP if different.
	certFallbackHosts := "localhost"
	if hostIP != "localhost" {
		certFallbackHosts = "localhost," + hostIP
	}

	args := []string{
		"create",
		"--name", browserName,
		"--hostname", browserName,
		"--network", "host",
		"--shm-size", shmSize,
		"--restart", "unless-stopped",
		// Environment
		"-e", fmt.Sprintf("WEB_LISTENING_PORT=%d", ports.BrowserWeb),
		"-e", fmt.Sprintf("VNC_LISTENING_PORT=%d", ports.BrowserVNC),
		"-e", fmt.Sprintf("FF_OPEN_URL=https://%s:%d", hostIP, ports.XemuHTTPS),
		"-e", "FF_KIOSK=1",
		"-e", "FF_CUSTOM_ARGS=",
		"-e", "FF_PREF_AUTOPLAY=media.autoplay.default=0",
		"-e", fmt.Sprintf(`FF_PREF_CERT=security.tls.insecure_fallback_hosts="%s"`, certFallbackHosts),
		"-e", fmt.Sprintf("DISPLAY_WIDTH=%d", width),
		"-e", fmt.Sprintf("DISPLAY_HEIGHT=%d", height),
		"-e", "KEEP_APP_RUNNING=1",
		// Volumes
		"-v", fmt.Sprintf("%s:/config:rw", abs(browserCfgDir)),
	}

	// Mount browser init scripts as individual files in /etc/cont-init.d/
	// (jlesage's s6-overlay only scans this directory, not subdirectories).
	if m.cfg.BrowserInitDir != "" {
		entries, err := os.ReadDir(abs(m.cfg.BrowserInitDir))
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					args = append(args, "-v", fmt.Sprintf("%s/%s:/etc/cont-init.d/%s:ro",
						abs(m.cfg.BrowserInitDir), e.Name(), e.Name()))
				}
			}
		}
	}

	args = append(args, browserImage)

	out, err := m.run(args...)
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

// Start starts both the xemu and browser containers.
func (m *Manager) Start(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.state.Containers[name]; !ok {
		return fmt.Errorf("container %q not found", name)
	}
	if out, err := m.run("start", name); err != nil {
		return fmt.Errorf("start %s: %s: %s", name, err, out)
	}
	if out, err := m.run("start", name+"-browser"); err != nil {
		return fmt.Errorf("start %s-browser: %s: %s", name, err, out)
	}
	return nil
}

// Stop stops both the browser and xemu containers.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.state.Containers[name]; !ok {
		return fmt.Errorf("container %q not found", name)
	}
	// Stop browser first (depends on xemu).
	if out, err := m.run("stop", name+"-browser"); err != nil {
		log.Printf("podman: stop %s-browser: %s: %s", name, err, out)
	}
	if out, err := m.run("stop", name); err != nil {
		return fmt.Errorf("stop %s: %s: %s", name, err, out)
	}
	return nil
}

// Remove stops and removes both containers, then removes them from state.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.state.Containers[name]; !ok {
		return fmt.Errorf("container %q not found", name)
	}

	// Stop + remove browser.
	_, _ = m.run("stop", name+"-browser")
	if out, err := m.run("rm", "-f", name+"-browser"); err != nil {
		log.Printf("podman: rm %s-browser: %s: %s", name, err, out)
	}
	// Stop + remove xemu.
	_, _ = m.run("stop", name)
	if out, err := m.run("rm", "-f", name); err != nil {
		log.Printf("podman: rm %s: %s: %s", name, err, out)
	}

	delete(m.state.Containers, name)
	if err := m.state.Save(); err != nil {
		log.Printf("podman: warning: failed to save state: %v", err)
	}
	return nil
}

// List returns all managed containers enriched with live podman status.
func (m *Manager) List() ([]ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]ContainerInfo, 0, len(m.state.Containers))
	for _, info := range m.state.Containers {
		out = append(out, *info)
	}
	return out, nil
}

// Status returns the podman status of the xemu container (e.g. "running",
// "exited", "created").
func (m *Manager) Status(name string) (string, error) {
	m.mu.Lock()
	if _, ok := m.state.Containers[name]; !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("container %q not found", name)
	}
	m.mu.Unlock()

	out, err := m.run("inspect", "--format", "{{.State.Status}}", name)
	if err != nil {
		return "unknown", nil
	}

	// podman inspect --format returns JSON-escaped string; try to unquote.
	s := string(out)
	var unquoted string
	if json.Unmarshal(out, &unquoted) == nil {
		s = unquoted
	}
	// Trim whitespace/newlines.
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s, nil
}

// run executes the configured podman command with the given arguments.
// PodmanCmd may contain leading args (e.g. "sudo -n podman") which are split
// on whitespace and prepended to args.
func (m *Manager) run(args ...string) ([]byte, error) {
	parts := strings.Fields(m.cfg.PodmanCmd)
	if len(parts) == 0 {
		parts = []string{"podman"}
	}
	full := append(parts[1:], args...)
	cmd := exec.Command(parts[0], full...)
	return cmd.CombinedOutput()
}

// abs resolves path to absolute, ignoring errors (returns original on failure).
func abs(path string) string {
	a, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return a
}
