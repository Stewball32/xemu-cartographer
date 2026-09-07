package pbtest

import (
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// FakeScraper is a canned scraperiface.Service. Infos feeds List / Inspect /
// InstanceState (InstanceExists + InstanceByConsole read Info.XboxName);
// Members feeds Membership() (RosteredIn). Everything else is inert. Tests
// mutate the slices directly between calls — the adapter never caches them.
type FakeScraper struct {
	Infos   []scraperiface.Info
	Members []scraperiface.ContainerMembership
}

var _ scraperiface.Service = (*FakeScraper)(nil)

// AddInstance appends a running instance with the given container name and
// Xbox console name.
func (f *FakeScraper) AddInstance(name, xboxName string) {
	f.Infos = append(f.Infos, scraperiface.Info{Name: name, XboxName: xboxName})
}

// SetMembership replaces the roster view of one container.
func (f *FakeScraper) SetMembership(container string, identities ...string) {
	for i := range f.Members {
		if f.Members[i].Container == container {
			f.Members[i].Identities = identities
			return
		}
	}
	f.Members = append(f.Members, scraperiface.ContainerMembership{Container: container, Identities: identities})
}

func (f *FakeScraper) Start(name, sock string) error { return nil }
func (f *FakeScraper) Stop(name string) error        { return nil }

func (f *FakeScraper) List() []scraperiface.Info {
	if f == nil {
		return nil
	}
	return append([]scraperiface.Info(nil), f.Infos...)
}

func (f *FakeScraper) Inspect(name string) (scraperiface.InspectState, bool) {
	if f == nil {
		return scraperiface.InspectState{}, false
	}
	for _, info := range f.Infos {
		if info.Name == name {
			return scraperiface.InspectState{Info: info, Running: true}, true
		}
	}
	return scraperiface.InspectState{}, false
}

func (f *FakeScraper) JoinReplayMessages() [][]byte                         { return nil }
func (f *FakeScraper) JoinReplayForInstance(name string) [][]byte           { return nil }
func (f *FakeScraper) JoinReplayForInstanceClass(name, cls string) [][]byte { return nil }
func (f *FakeScraper) JoinReplayForHostAll() [][]byte                       { return nil }

func (f *FakeScraper) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	return nil, false
}

func (f *FakeScraper) ProbeReply(instance string) ([]byte, bool) { return nil, false }

func (f *FakeScraper) InstanceState(name string) (scraperiface.InstanceState, bool) {
	if f == nil {
		return scraperiface.InstanceState{}, false
	}
	for _, info := range f.Infos {
		if info.Name == name {
			return scraperiface.InstanceState{
				Name: info.Name, TitleID: info.TitleID, Title: info.Title, XboxName: info.XboxName, Running: true,
			}, true
		}
	}
	return scraperiface.InstanceState{}, false
}

func (f *FakeScraper) Membership() []scraperiface.ContainerMembership {
	if f == nil {
		return nil
	}
	return append([]scraperiface.ContainerMembership(nil), f.Members...)
}
