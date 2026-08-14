package tui

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/bczaicki/vexillologist/internal/cli"
	"github.com/bczaicki/vexillologist/internal/ld"
)

// fakeAPI is an in-memory ld.API recording mutation calls.
type fakeAPI struct {
	mu            sync.Mutex
	envs          []ld.Environment
	flags         []ld.FeatureFlag
	archivedFlags []ld.FeatureFlag
	statuses      map[string]ld.FlagStatus
	toggles       []string // "key:env:on"
	archives      []string
	restores      []string
}

func (f *fakeAPI) ListProjects(ctx context.Context) ([]ld.Project, error) {
	return []ld.Project{{Key: "horizon", Name: "Horizon"}}, nil
}

func (f *fakeAPI) ListEnvironments(ctx context.Context, projectKey string) ([]ld.Environment, error) {
	return f.envs, nil
}

func (f *fakeAPI) ListFlags(ctx context.Context, projectKey string, opts ld.ListFlagsOptions) ([]ld.FeatureFlag, error) {
	if opts.Archived {
		return f.archivedFlags, nil
	}
	return f.flags, nil
}

func (f *fakeAPI) GetFlag(ctx context.Context, projectKey, flagKey string, envKeys []string) (*ld.FeatureFlag, error) {
	for i := range f.flags {
		if f.flags[i].Key == flagKey {
			return &f.flags[i], nil
		}
	}
	return nil, &ld.APIError{StatusCode: 404, Code: "not_found", Message: "flag not found"}
}

func (f *fakeAPI) CreateFlag(ctx context.Context, projectKey string, params ld.CreateFlagParams) (*ld.FeatureFlag, error) {
	flag := ld.FeatureFlag{Key: params.Key, Name: params.Name}
	f.mu.Lock()
	f.flags = append(f.flags, flag)
	f.mu.Unlock()
	return &flag, nil
}

func (f *fakeAPI) ToggleFlag(ctx context.Context, projectKey, flagKey, envKey string, on bool, comment string) (*ld.FeatureFlag, error) {
	f.mu.Lock()
	f.toggles = append(f.toggles, flagKey+":"+envKey+":"+onOff(on))
	f.mu.Unlock()
	return &ld.FeatureFlag{
		Key:          flagKey,
		Name:         flagKey,
		Environments: map[string]ld.FlagEnvConfig{envKey: {On: on}},
	}, nil
}

func (f *fakeAPI) ArchiveFlag(ctx context.Context, projectKey, flagKey, comment string) (*ld.FeatureFlag, error) {
	f.mu.Lock()
	f.archives = append(f.archives, flagKey)
	f.mu.Unlock()
	return &ld.FeatureFlag{Key: flagKey, Archived: true}, nil
}

func (f *fakeAPI) RestoreFlag(ctx context.Context, projectKey, flagKey, comment string) (*ld.FeatureFlag, error) {
	f.mu.Lock()
	f.restores = append(f.restores, flagKey)
	f.mu.Unlock()
	return &ld.FeatureFlag{Key: flagKey, Archived: false}, nil
}

func (f *fakeAPI) GetFlagStatuses(ctx context.Context, projectKey, envKey string) (map[string]ld.FlagStatus, error) {
	return f.statuses, nil
}

func newFake() *fakeAPI {
	return &fakeAPI{
		envs: []ld.Environment{
			{Key: "staging", Name: "Staging"},
			{Key: "production", Name: "Production", Critical: true},
		},
		flags: []ld.FeatureFlag{
			{
				Key:  "connect1-rollout-geofencing",
				Name: "Rollout Geofencing",
				Kind: "boolean",
				Environments: map[string]ld.FlagEnvConfig{
					"staging":    {On: false},
					"production": {On: true},
				},
			},
		},
	}
}

func startApp(t *testing.T, fake *fakeAPI, env string) *teatest.TestModel {
	t.Helper()
	m := New(fake, cli.TUIOptions{Project: "horizon", Environment: env})
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
}

func waitForOutput(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(want))
	}, teatest.WithDuration(5*time.Second))
}

func TestFlagListRenders(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestDetailPaneShowsCamelCaseKey(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1RolloutGeofencing")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestToggleUnguardedEnvWithYN(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	waitForOutput(t, tm, "Turn ON")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	waitForOutput(t, tm, "is now on in staging")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.toggles) != 1 || fake.toggles[0] != "connect1-rollout-geofencing:staging:on" {
		t.Errorf("toggles = %v", fake.toggles)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestToggleGuardedEnvRequiresTypedKey(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "production")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	waitForOutput(t, tm, "guarded")

	// "y" alone must NOT confirm a guarded env; it lands in the text input.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	fake.mu.Lock()
	if len(fake.toggles) != 0 {
		t.Errorf("toggle fired without typed confirmation: %v", fake.toggles)
	}
	fake.mu.Unlock()

	// Clear the stray "y", type the env key, and confirm.
	tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("production")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForOutput(t, tm, "is now off in production")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.toggles) != 1 || fake.toggles[0] != "connect1-rollout-geofencing:production:off" {
		t.Errorf("toggles = %v", fake.toggles)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestArchiveRequiresTypedFlagKey(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	waitForOutput(t, tm, "ALL environments")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("connect1-rollout-geofencing")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForOutput(t, tm, "archived connect1-rollout-geofencing")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.archives) != 1 {
		t.Errorf("archives = %v", fake.archives)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestEnterFocusesDetailEscReturnsToList(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForOutput(t, tm, "esc back to list")

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	m, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatal("final model is not tui.Model")
	}
	if m.flags.detailFocused() {
		t.Error("detail pane still focused after esc")
	}
}

func TestArchivedViewShowsBadge(t *testing.T) {
	fake := newFake()
	fake.archivedFlags = []ld.FeatureFlag{{
		Key:      "old-legacy-flag",
		Name:     "Old Legacy Flag",
		Kind:     "boolean",
		Archived: true,
		Environments: map[string]ld.FlagEnvConfig{
			"staging": {On: false},
		},
	}}
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	// The row and its badge land in the same frame; wait for the badge.
	waitForOutput(t, tm, "⚑ archived")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestSortCycleReordersList(t *testing.T) {
	fake := newFake()
	// Two extra flags so sort order is observable: zzz created most recently.
	fake.flags = append(fake.flags,
		ld.FeatureFlag{Key: "aaa-flag", Name: "AAA", CreationDate: 100,
			Environments: map[string]ld.FlagEnvConfig{"staging": {On: false}}},
		ld.FeatureFlag{Key: "zzz-flag", Name: "ZZZ", CreationDate: 999,
			Environments: map[string]ld.FlagEnvConfig{"staging": {On: false}}},
	)
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	// key → name → created: after two presses zzz-flag (newest) sorts first.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	waitForOutput(t, tm, "sort: name")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	waitForOutput(t, tm, "sort: created")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	m, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatal("final model is not tui.Model")
	}
	if m.rawFlags[0].Key != "zzz-flag" {
		t.Errorf("first flag after created-sort = %s, want zzz-flag", m.rawFlags[0].Key)
	}
}

func TestStatusAnnotationRenders(t *testing.T) {
	fake := newFake()
	fake.statuses = map[string]ld.FlagStatus{
		"connect1-rollout-geofencing": {Name: "launched"},
	}
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "launched")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestRestoreArchivedFlag(t *testing.T) {
	fake := newFake()
	fake.archivedFlags = []ld.FeatureFlag{{
		Key:      "old-legacy-flag",
		Name:     "Old Legacy Flag",
		Archived: true,
		Environments: map[string]ld.FlagEnvConfig{
			"staging": {On: false},
		},
	}}
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	waitForOutput(t, tm, "old-legacy-flag")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	waitForOutput(t, tm, "Restore archived flag old-legacy-flag?")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	waitForOutput(t, tm, "restored old-legacy-flag")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.restores) != 1 || fake.restores[0] != "old-legacy-flag" {
		t.Errorf("restores = %v", fake.restores)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestRestoreOnActiveFlagErrors(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	waitForOutput(t, tm, "is not archived")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.restores) != 0 {
		t.Errorf("restores = %v, want none", fake.restores)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestKebabToCamel(t *testing.T) {
	cases := map[string]string{
		"connect-1-device-tag-filters": "connect1DeviceTagFilters",
		"connect1-rollout-geofencing":  "connect1RolloutGeofencing",
		"simple":                       "simple",
	}
	for in, want := range cases {
		if got := kebabToCamel(in); got != want {
			t.Errorf("kebabToCamel(%q) = %q, want %q", in, got, want)
		}
	}
}
