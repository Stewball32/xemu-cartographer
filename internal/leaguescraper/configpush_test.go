package leaguescraper

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/xemu-cartographer/xc-scraper/capture"
	"github.com/xemu-cartographer/xc-scraper/daemon"
	"github.com/xemu-cartographer/xemu-cartographer/internal/podman"
)

// fakeCtl records PutConfig / PutInstance calls (ControlClient).
type fakeCtl struct {
	mu    sync.Mutex
	docs  []daemon.Document
	insts []instPut
	err   error
	calls chan struct{}
}

type instPut struct {
	name string
	cfg  daemon.InstanceConfig
}

func newFakeCtl() *fakeCtl { return &fakeCtl{calls: make(chan struct{}, 64)} }

func (f *fakeCtl) PutConfig(_ context.Context, doc daemon.Document) (daemon.Document, error) {
	f.mu.Lock()
	f.docs = append(f.docs, doc)
	err := f.err
	f.mu.Unlock()
	f.calls <- struct{}{}
	return doc, err
}

func (f *fakeCtl) PutInstance(_ context.Context, name string, cfg daemon.InstanceConfig) (daemon.Document, error) {
	f.mu.Lock()
	f.insts = append(f.insts, instPut{name, cfg})
	err := f.err
	f.mu.Unlock()
	f.calls <- struct{}{}
	return daemon.Document{}, err
}

func (f *fakeCtl) configs() []daemon.Document {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]daemon.Document(nil), f.docs...)
}

func (f *fakeCtl) instances() []instPut {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]instPut(nil), f.insts...)
}

// waitCalls blocks until n control calls happened (or fails).
func (f *fakeCtl) waitCalls(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-f.calls:
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for control call %d/%d", i+1, n)
		}
	}
}

// expectQuiet fails when another control call lands within d.
func (f *fakeCtl) expectQuiet(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case <-f.calls:
		t.Fatalf("unexpected extra control call")
	case <-time.After(d):
	}
}

// fakePods is a PodSource over a fixed container list; overlays exist for
// names present in overlays.
type fakePods struct {
	infos    []podman.ContainerInfo
	overlays map[string]string
}

func (p *fakePods) List() ([]podman.ContainerInfo, error) { return p.infos, nil }
func (p *fakePods) OverlayPath(name string) (string, bool) {
	path, ok := p.overlays[name]
	return path, ok
}

func ensureCollection(t *testing.T, app core.App, name string, fields ...core.Field) *core.Collection {
	t.Helper()
	c := core.NewBaseCollection(name)
	c.Fields.Add(fields...)
	if err := app.Save(c); err != nil {
		t.Fatalf("save %s collection: %v", name, err)
	}
	return c
}

func saveRecord(t *testing.T, app core.App, col *core.Collection, values map[string]any) *core.Record {
	t.Helper()
	r := core.NewRecord(col)
	for k, v := range values {
		if k == "id" {
			r.Id = v.(string)
			continue
		}
		r.Set(k, v)
	}
	if err := app.Save(r); err != nil {
		t.Fatalf("save %s record: %v", col.Name, err)
	}
	return r
}

const isoRecordID = "abcdefghijklmno" // 15-char PB id

func fileFromBytes(t *testing.T, b []byte, name string) *filesystem.File {
	t.Helper()
	f, err := filesystem.NewFileFromBytes(b, name)
	if err != nil {
		t.Fatalf("NewFileFromBytes: %v", err)
	}
	return f
}

// pushFixture builds the four PB collections + an isos row and a podman
// view of two containers (play-1 with a disc + overlay + neutral flag,
// admin-a plain).
func pushFixture(t *testing.T) (core.App, *fakePods) {
	t.Helper()
	app := newPolicyApp(t)
	pol := ensurePolicyCollection(t, app)
	savePolicy(t, app, pol, capture.Policy{Instance: "*", Class: "tick", Mode: capture.ModeAlways, Cadence: capture.Cadence1s, Sink: "pb:game_events"})
	savePolicy(t, app, pol, capture.Policy{Instance: "play-1", Class: "event", Mode: capture.ModeAuto, Sink: "file:/tmp/x.ndjson"})
	savePolicy(t, app, pol, capture.Policy{Instance: "*", Class: "objects", Mode: capture.Mode("off")}) // daemon rejects
	tags := ensureCollection(t, app, DummyGamertagsCollection, &core.TextField{Name: "gamertag"})
	saveRecord(t, app, tags, map[string]any{"gamertag": "zeta"})
	saveRecord(t, app, tags, map[string]any{"gamertag": "alpha"})
	containers := ensureCollection(t, app, ContainersCollection, &core.TextField{Name: "name"}, &core.BoolField{Name: "is_neutral_host"})
	saveRecord(t, app, containers, map[string]any{"name": "play-1", "is_neutral_host": true})
	saveRecord(t, app, containers, map[string]any{"name": "admin-a", "is_neutral_host": false})
	sets := ensureCollection(t, app, OffsetSetsCollection, &core.TextField{Name: "set_id"}, &core.FileField{Name: "file", MaxSelect: 1, MaxSize: 1 << 20})
	good := []byte(`{"game":"ce","id":"ce-v2","description":"t","offsets":{"x":{"value":"0x10","type":"u32"}}}`)
	saveRecord(t, app, sets, map[string]any{"set_id": "ce-v2", "file": fileFromBytes(t, good, "set.json")})
	saveRecord(t, app, sets, map[string]any{"set_id": "broken", "file": fileFromBytes(t, []byte(`{"game":""}`), "bad.json")})
	isos := ensureCollection(t, app, ISOsCollection, &core.TextField{Name: "offset_set"})
	saveRecord(t, app, isos, map[string]any{"id": isoRecordID, "offset_set": "ce-v2"})

	overlay := filepath.Join(t.TempDir(), "play-1.qcow2")
	if err := os.WriteFile(overlay, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	pods := &fakePods{
		infos: []podman.ContainerInfo{
			{Name: "play-1", Ports: podman.Ports{BrowserWeb: 3103}, GameISO: "/isos/" + isoRecordID + ".iso"},
			{Name: "admin-a", Ports: podman.Ports{BrowserWeb: 3108}},
		},
		overlays: map[string]string{"play-1": overlay},
	}
	return app, pods
}

func newTestPusher(t *testing.T, app core.App, ctl ControlClient, pods PodSource, debounce time.Duration) *ConfigPusher {
	t.Helper()
	p := NewConfigPusher(PushOptions{App: app, Ctl: ctl, Pods: pods, Debounce: debounce, Logf: t.Logf})
	t.Cleanup(p.Close)
	return p
}

func boolPtr(b bool) *bool { return &b }

// TestConfigPusherDocument: the full document is built from PB + podman —
// pb: sinks stripped (mode/cadence kept), daemon-invalid rows skipped,
// sorted tags, offset-set file bytes (bad sets skipped), per-instance meta.
func TestConfigPusherDocument(t *testing.T) {
	app, pods := pushFixture(t)
	p := newTestPusher(t, app, newFakeCtl(), pods, 0)
	doc := p.Document()

	wantPolicies := []capture.Policy{
		{Instance: "*", Class: "tick", Mode: capture.ModeAlways, Cadence: capture.Cadence1s},
		{Instance: "play-1", Class: "event", Mode: capture.ModeAuto, Sink: "file:/tmp/x.ndjson"},
	}
	if !reflect.DeepEqual(doc.CapturePolicies, wantPolicies) {
		t.Fatalf("policies = %+v\nwant %+v", doc.CapturePolicies, wantPolicies)
	}
	if got := doc.DummyGamertags; !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("dummy tags = %v", got)
	}
	if len(doc.OffsetSets) != 1 {
		t.Fatalf("offset sets = %v (broken set must be skipped)", doc.OffsetSets)
	}
	if !json.Valid(doc.OffsetSets["ce-v2"]) || !strings.Contains(string(doc.OffsetSets["ce-v2"]), `"id":"ce-v2"`) {
		t.Fatalf("offset set bytes = %s", doc.OffsetSets["ce-v2"])
	}
	wantInst := map[string]daemon.InstanceConfig{
		"play-1":  {OffsetSet: "ce-v2", OverlayPath: pods.overlays["play-1"], VNCURL: "ws://127.0.0.1:3103/websockify", NeutralHost: true, HostDrive: boolPtr(true)},
		"admin-a": {VNCURL: "ws://127.0.0.1:3108/websockify", HostDrive: boolPtr(false)},
	}
	if !reflect.DeepEqual(doc.Instances, wantInst) {
		t.Fatalf("instances = %+v\nwant %+v", doc.Instances, wantInst)
	}
	if err := daemon.Validate(doc); err != nil {
		t.Fatalf("daemon rejects the document: %v", err)
	}
	if doc.Version != 1 {
		t.Fatalf("version = %d", doc.Version)
	}
}

// TestConfigPusherMarker: HostDriveMarker is env-tunable; the default is the
// daemon's.
func TestConfigPusherMarker(t *testing.T) {
	app, pods := pushFixture(t)
	p := NewConfigPusher(PushOptions{App: app, Pods: pods, HostDriveMarker: "admin-", Logf: t.Logf})
	ic := p.InstanceConfig(pods.infos[1])
	if ic.HostDrive == nil || !*ic.HostDrive {
		t.Fatalf("admin-a with marker admin- → host drive: %+v", ic)
	}
	if p.opt.HostDriveMarker != "admin-" || NewConfigPusher(PushOptions{}).opt.HostDriveMarker != daemon.DefaultHostDriveMarker {
		t.Fatalf("marker defaults wrong")
	}
	if NewConfigPusher(PushOptions{}).Document().Version != 1 {
		t.Fatalf("nil app document must still carry version 1")
	}
}

// TestConfigPusherHookDebounce: a burst of PB writes across the four hook
// collections collapses into one full push after the window; a later write
// pushes again.
func TestConfigPusherHookDebounce(t *testing.T) {
	app, pods := pushFixture(t)
	ctl := newFakeCtl()
	p := newTestPusher(t, app, ctl, pods, 40*time.Millisecond)
	p.RegisterHooks(app)

	pol, _ := app.FindCollectionByNameOrId(CapturePoliciesCollection)
	tags, _ := app.FindCollectionByNameOrId(DummyGamertagsCollection)
	rec := savePolicy(t, app, pol, capture.Policy{Instance: "*", Class: "debug", Mode: capture.ModeNever})
	saveRecord(t, app, tags, map[string]any{"gamertag": "mid"})
	rec.Set("mode", string(capture.ModeAuto))
	if err := app.Save(rec); err != nil {
		t.Fatal(err)
	}
	ctl.waitCalls(t, 1)
	ctl.expectQuiet(t, 150*time.Millisecond)
	docs := ctl.configs()
	if len(docs) != 1 || p.Pushes() != 1 {
		t.Fatalf("pushes = %d (%d docs)", p.Pushes(), len(docs))
	}
	if got := docs[0].DummyGamertags; !reflect.DeepEqual(got, []string{"alpha", "mid", "zeta"}) {
		t.Fatalf("pushed tags = %v", got)
	}
	found := false
	for _, r := range docs[0].CapturePolicies {
		if r.Class == "debug" && r.Mode == capture.ModeAuto {
			found = true
		}
	}
	if !found {
		t.Fatalf("pushed policies miss the updated debug row: %+v", docs[0].CapturePolicies)
	}

	if err := app.Delete(rec); err != nil {
		t.Fatal(err)
	}
	ctl.waitCalls(t, 1)
	if docs := ctl.configs(); len(docs) != 2 || len(docs[1].CapturePolicies) != 2 {
		t.Fatalf("second push = %+v", docs)
	}
	p.Close()
	saveRecord(t, app, tags, map[string]any{"gamertag": "late"})
	ctl.expectQuiet(t, 150*time.Millisecond)
}

// TestConfigPusherPodHooks: podman Created PUTs the instance synchronously
// (overlay, VNC URL, host-drive, offset set, neutral host); Removed schedules
// a full push whose instances map no longer carries the key.
func TestConfigPusherPodHooks(t *testing.T) {
	app, pods := pushFixture(t)
	ctl := newFakeCtl()
	p := newTestPusher(t, app, ctl, pods, 20*time.Millisecond)
	hooks := p.PodHooks()

	hooks.Created(pods.infos[0])
	if p.InstancePushes() != 1 || len(ctl.instances()) != 1 {
		t.Fatalf("Created must PUT synchronously: %d", p.InstancePushes())
	}
	<-ctl.calls
	got := ctl.instances()[0]
	want := instPut{"play-1", daemon.InstanceConfig{OffsetSet: "ce-v2", OverlayPath: pods.overlays["play-1"], VNCURL: "ws://127.0.0.1:3103/websockify", NeutralHost: true, HostDrive: boolPtr(true)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("instance put = %+v\nwant %+v", got, want)
	}

	pods.infos = pods.infos[1:] // play-1 removed
	hooks.Removed("play-1")
	ctl.waitCalls(t, 1)
	docs := ctl.configs()
	if len(docs) != 1 {
		t.Fatalf("Removed must schedule one full push, got %d", len(docs))
	}
	if _, ok := docs[0].Instances["play-1"]; ok || len(docs[0].Instances) != 1 {
		t.Fatalf("removed key still pushed: %+v", docs[0].Instances)
	}
}

// TestConfigPusherOnConnectAndErrors: OnConnect pushes off the caller's
// goroutine; a control error is logged + kept in LastError and cleared by
// the next success; a nil Ctl is a no-op.
func TestConfigPusherOnConnectAndErrors(t *testing.T) {
	app, pods := pushFixture(t)
	ctl := newFakeCtl()
	p := newTestPusher(t, app, ctl, pods, 0)

	p.OnConnect()
	ctl.waitCalls(t, 1)
	if p.Pushes() != 1 || p.LastError() != nil {
		t.Fatalf("after connect: pushes=%d err=%v", p.Pushes(), p.LastError())
	}

	boom := errors.New("upstream down")
	ctl.mu.Lock()
	ctl.err = boom
	ctl.mu.Unlock()
	if err := p.Push(); !errors.Is(err, boom) || !errors.Is(p.LastError(), boom) {
		t.Fatalf("push error not surfaced: %v / %v", err, p.LastError())
	}
	<-ctl.calls
	ctl.mu.Lock()
	ctl.err = nil
	ctl.mu.Unlock()
	if err := p.Push(); err != nil || p.LastError() != nil {
		t.Fatalf("error must clear on success: %v / %v", err, p.LastError())
	}
	<-ctl.calls

	none := NewConfigPusher(PushOptions{App: app, Pods: pods, Logf: t.Logf})
	if err := none.Push(); err != nil || none.Pushes() != 0 {
		t.Fatalf("nil ctl must be a no-op: %v", err)
	}
	if err := none.PushInstance(pods.infos[0]); err != nil || none.InstancePushes() != 0 {
		t.Fatalf("nil ctl instance push must be a no-op: %v", err)
	}
}

// TestPodmanHooksFire: the podman Manager fires Created/Removed outside its
// lock (the hook may call back into the Manager).
func TestPodmanHooksFire(t *testing.T) {
	var _ PodSource = (*podman.Manager)(nil)
	var h podman.Hooks
	called := 0
	h.Created = func(podman.ContainerInfo) { called++ }
	h.Removed = func(string) { called++ }
	if h.Created == nil || h.Removed == nil {
		t.Fatal("hooks unset")
	}
	h.Created(podman.ContainerInfo{})
	h.Removed("x")
	if called != 2 {
		t.Fatalf("called = %d", called)
	}
}
