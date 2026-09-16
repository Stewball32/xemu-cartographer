package podman

// Hooks are the Manager's lifecycle callbacks (DESIGN-STEP8 §10, F5): the
// league's config push PUTs the per-instance daemon config on Created (before
// xemu boots) and drops the key on Removed. Both fire after the Manager's own
// work succeeded and outside its lock, so a hook may call back into it.
type Hooks struct {
	Created func(info ContainerInfo)
	Removed func(name string)
}

// SetHooks installs h, replacing any previous hooks. Safe to call at any time.
func (m *Manager) SetHooks(h Hooks) {
	m.hooksMu.Lock()
	m.hooks = h
	m.hooksMu.Unlock()
}

func (m *Manager) getHooks() Hooks {
	m.hooksMu.RLock()
	defer m.hooksMu.RUnlock()
	return m.hooks
}

// CreateWithOptions is createWithOptions + Hooks.Created on success.
func (m *Manager) CreateWithOptions(name string, opts CreateOptions) (*ContainerInfo, error) {
	info, err := m.createWithOptions(name, opts)
	if err == nil {
		if h := m.getHooks(); h.Created != nil {
			h.Created(*info)
		}
	}
	return info, err
}

// Remove is remove + Hooks.Removed on success.
func (m *Manager) Remove(name string) error {
	err := m.remove(name)
	if err == nil {
		if h := m.getHooks(); h.Removed != nil {
			h.Removed(name)
		}
	}
	return err
}
