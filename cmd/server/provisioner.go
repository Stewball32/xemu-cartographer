package main

import (
	playroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/play"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

// podmanProvisioner adapts *podman.Manager to playroutes.HostProvisioner, the
// wire behind POST /api/play/request. Keeping this adapter in cmd/server (not
// the play package) is what lets routes/play stay free of a compile-time podman
// dependency — it depends only on the small HostProvisioner interface, and
// main.go injects this concrete implementation.
type podmanProvisioner struct{ m *podman.Manager }

// Provision creates AND starts a fresh xemu + browser pair named `name` with the
// library ISO `filename` attached as its DVD, so it boots straight into that
// game (ADR-0004). If Start fails after a successful Create the partial info is
// still returned alongside the error so the caller can surface both.
func (p podmanProvisioner) Provision(name, filename string) (playroutes.ProvisionResult, error) {
	info, err := p.m.CreateWithOptions(name, podman.CreateOptions{GameISO: filename})
	if err != nil {
		return playroutes.ProvisionResult{}, err
	}
	res := playroutes.ProvisionResult{Name: info.Name, Index: info.Index, GameISO: info.GameISO}
	if err := p.m.Start(name); err != nil {
		return res, err
	}
	return res, nil
}

// Exists reports whether a container of this name is already provisioned.
func (p podmanProvisioner) Exists(name string) bool {
	_, ok := p.m.Get(name)
	return ok
}

// NamePrefix exposes the manager's configured container-name namespace so the
// play flow can prefix per-player box names (empty in prod).
func (p podmanProvisioner) NamePrefix() string { return p.m.NamePrefix() }
