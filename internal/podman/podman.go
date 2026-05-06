// Package podman manages xemu + browser container pairs via the podman CLI.
package podman

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	cfg        Config
	store      Store
	containers map[string]*ContainerInfo
	mu         sync.Mutex
}

// NewManager loads persisted state via store and returns a ready Manager.
func NewManager(cfg Config, store Store) (*Manager, error) {
	containers, err := store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("podman: load state: %w", err)
	}
	if containers == nil {
		containers = make(map[string]*ContainerInfo)
	}
	return &Manager{cfg: cfg, store: store, containers: containers}, nil
}

// Create provisions a new xemu + browser container pair without starting them.
func (m *Manager) Create(name string) (*ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[name]; exists {
		return nil, fmt.Errorf("container %q already exists", name)
	}

	idx := nextIndex(m.containers)
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

	// Pre-generate a CA + server-cert pair for xemu's HTTPS listener.
	// nginx serves the server leaf (with SAN for localhost), and the
	// firefox container imports the CA as a trusted root via
	// containers/browser/init/60-trust-xemu-cert.sh. See cert.go for why
	// two certs instead of one self-signed leaf-as-its-own-root.
	sslDir := filepath.Join(configDir, "ssl")
	if err := generateXemuCerts(sslDir, name); err != nil {
		return nil, fmt.Errorf("generate ssl certs: %w", err)
	}

	// Generate labwc autostart with QMP enabled (only if it doesn't exist).
	autostartDir := filepath.Join(configDir, ".config", "labwc")
	autostartPath := filepath.Join(autostartDir, "autostart")
	if _, err := os.Stat(autostartPath); os.IsNotExist(err) {
		if err := os.MkdirAll(autostartDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", autostartDir, err)
		}
		qmpArg := fmt.Sprintf("-qmp unix:/qmp/%s.sock,server,nowait", name)
		autostart := fmt.Sprintf("#!/bin/bash\nfoot -e /opt/xemu/AppRun %s -full-screen\n", qmpArg)
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

	m.containers[name] = info
	if err := m.store.Upsert(info); err != nil {
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
		"-e", fmt.Sprintf("PUID=%d", os.Getuid()),
		"-e", fmt.Sprintf("PGID=%d", os.Getgid()),
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
	// Both containers run on `--network host`. xemu requires it for pcap
	// netplay (binds to wlan0); the browser piggybacks so the kiosk Firefox
	// can reach xemu via `localhost` without crossing podman's bridge →
	// host firewall (which silently drops SYNs to host ports even with no
	// firewalld running, courtesy of netavark's default rules).
	//
	// X auth caveat: jlesage's Xvnc starts without `-auth`, and the image
	// ships no xauth binary or Xauthority setup. Clients (xcompmgr,
	// hsetroot, Firefox) then die with "Authorization required, but no
	// authorization protocol specified". Passing `-ac` to Xvnc disables
	// access control entirely — safe here because the X server only
	// listens on a unix socket inside the container's namespace, even
	// with `--network host`. Without `-ac`, the container restart-loops.
	//
	// Trade-off: the kiosk's WEB_LISTENING_PORT and VNC_LISTENING_PORT
	// listen on 0.0.0.0 on the host. Single-public-port goal is preserved
	// at the host firewall layer for prod deploys; the JWT-gated reverse-
	// proxy + WS relay (proxy.go, vnc.go) remain the only intended public
	// path through PocketBase :8090.
	xemuTargetHost := "localhost"

	args := []string{
		"create",
		"--name", browserName,
		"--hostname", browserName,
		"--network", "host",
		"--add-host", fmt.Sprintf("%s:127.0.0.1", browserName),
		"--shm-size", shmSize,
		"--restart", "unless-stopped",
		// Environment
		"-e", fmt.Sprintf("USER_ID=%d", os.Getuid()),
		"-e", fmt.Sprintf("GROUP_ID=%d", os.Getgid()),
		"-e", fmt.Sprintf("WEB_LISTENING_PORT=%d", ports.BrowserWeb),
		"-e", fmt.Sprintf("VNC_LISTENING_PORT=%d", ports.BrowserVNC),
		"-e", "XVNC_SERVER_CUSTOM_PARAMS=-ac",
		// HTTPS is required by linuxserver/xemu — selkies's video/audio
		// pipeline uses WebCodecs, which browsers gate behind HTTPS even
		// on localhost. Per the image's official docs:
		// https://docs.linuxserver.io/images/docker-xemu/
		// "HTTPS is required for full functionality. Modern browser
		// features such as WebCodecs, used for video and audio, will not
		// function over an insecure HTTP connection." Trust on the cert
		// is solved separately (see /etc/cont-init.d/60-trust-xemu-cert.sh
		// in containers/browser/init/), not by switching to HTTP.
		"-e", fmt.Sprintf("FF_OPEN_URL=https://%s:%d", xemuTargetHost, ports.XemuHTTPS),
		"-e", "FF_KIOSK=1",
		"-e", "FF_CUSTOM_ARGS=",
		"-e", "FF_PREF_AUTOPLAY=media.autoplay.default=0",
		// Suppress Firefox first-run / session-restore / "set as default" prompts
		// that otherwise overlay the kiosk on every restart.
		"-e", "FF_PREF_DISABLE_RESUME_FROM_CRASH=browser.sessionstore.resume_from_crash=false",
		"-e", "FF_PREF_DISABLE_MAX_RESUMED_CRASHES=browser.sessionstore.max_resumed_crashes=0",
		"-e", "FF_PREF_DISABLE_SESSION_RESTORE=browser.sessionstore.resume_session_once=false",
		"-e", `FF_PREF_DISABLE_WHATSNEW=browser.startup.homepage_override.mstone="ignore"`,
		"-e", "FF_PREF_DISABLE_WELCOME=browser.aboutwelcome.enabled=false",
		"-e", "FF_PREF_DISABLE_DEFAULT_BROWSER=browser.shell.checkDefaultBrowser=false",
		// Suppress the "Firefox automatically sends some data to Mozilla"
		// banner that appears on first run.
		"-e", "FF_PREF_DISABLE_DATAREPORTING_NOTICE=datareporting.policy.dataSubmissionPolicyBypassNotification=true",
		"-e", "FF_PREF_DATAREPORTING_ACCEPTED=datareporting.policy.dataSubmissionPolicyAcceptedVersion=2",
		"-e", "FF_PREF_DISABLE_TELEMETRY_FIRSTRUN=toolkit.telemetry.reportingpolicy.firstRun=false",
		"-e", fmt.Sprintf("DISPLAY_WIDTH=%d", width),
		"-e", fmt.Sprintf("DISPLAY_HEIGHT=%d", height),
		"-e", "KEEP_APP_RUNNING=1",
		// Volumes
		"-v", fmt.Sprintf("%s:/config:rw", abs(browserCfgDir)),
		// Read-only access to xemu's HTTPS cert. The cont-init script
		// 60-trust-xemu-cert.sh imports this into Firefox's NSS DB
		// (cert9.db) as a trusted root, so the kiosk loads
		// https://localhost:<XemuHTTPS> without a cert warning.
		"-v", fmt.Sprintf("%s:/xemu-cert:ro", abs(filepath.Join(m.cfg.ConfigsDir, name, "ssl"))),
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

	if _, ok := m.containers[name]; !ok {
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

	if _, ok := m.containers[name]; !ok {
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

	if _, ok := m.containers[name]; !ok {
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

	delete(m.containers, name)
	if err := m.store.Delete(name); err != nil {
		log.Printf("podman: warning: failed to delete state: %v", err)
	}
	return nil
}

// List returns all managed containers enriched with live podman status.
func (m *Manager) List() ([]ContainerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]ContainerInfo, 0, len(m.containers))
	for _, info := range m.containers {
		out = append(out, *info)
	}
	return out, nil
}

// Status returns the podman status of the xemu container (e.g. "running",
// "exited", "created").
func (m *Manager) Status(name string) (string, error) {
	m.mu.Lock()
	if _, ok := m.containers[name]; !ok {
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

// Logs shells `podman logs --tail N` against either the xemu (which="xemu") or
// browser (which="browser") container of the named pair. tail is clamped to
// [1, 1000]. The returned string is the combined stdout+stderr of podman.
func (m *Manager) Logs(name string, tail int, which string) (string, error) {
	m.mu.Lock()
	if _, ok := m.containers[name]; !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("container %q not found", name)
	}
	m.mu.Unlock()

	if tail <= 0 {
		tail = 200
	}
	if tail > 1000 {
		tail = 1000
	}

	target := name
	if which == "browser" {
		target = name + "-browser"
	}

	out, err := m.run("logs", "--tail", strconv.Itoa(tail), target)
	if err != nil {
		return string(out), fmt.Errorf("podman logs %s: %w", target, err)
	}
	return string(out), nil
}

// Get returns the ContainerInfo for name, or false if unknown.
func (m *Manager) Get(name string) (ContainerInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.containers[name]
	if !ok {
		return ContainerInfo{}, false
	}
	return *info, true
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
