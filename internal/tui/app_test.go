package tui

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/phillips-connect/vexillologist/internal/cli"
	"github.com/phillips-connect/vexillologist/internal/ld"
)

// fakeAPI is an in-memory ld.API recording mutation calls.
type fakeAPI struct {
	mu       sync.Mutex
	envs     []ld.Environment
	flags    []ld.FeatureFlag
	toggles  []string // "key:env:on"
	archives []string
}

func (f *fakeAPI) ListProjects(ctx context.Context) ([]ld.Project, error) {
	return []ld.Project{{Key: "horizon", Name: "Horizon"}}, nil
}

func (f *fakeAPI) ListEnvironments(ctx context.Context, projectKey string) ([]ld.Environment, error) {
	return f.envs, nil
}

func (f *fakeAPI) ListFlags(ctx context.Context, projectKey string, opts ld.ListFlagsOptions) ([]ld.FeatureFlag, error) {
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
