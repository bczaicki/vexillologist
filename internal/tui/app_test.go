package tui

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	listEnvKeys   []string // the EnvKeys of each ListFlags call, comma-joined
}

func (f *fakeAPI) ListProjects(ctx context.Context) ([]ld.Project, error) {
	return []ld.Project{{Key: "horizon", Name: "Horizon"}}, nil
}

func (f *fakeAPI) ListEnvironments(ctx context.Context, projectKey string) ([]ld.Environment, error) {
	return f.envs, nil
}

// ListFlags mirrors LaunchDarkly's env scoping: the Environments map only
// carries the environments that were actually asked for. Requesting none is
// what produced an all-dashes matrix, so the fake reproduces it faithfully.
func (f *fakeAPI) ListFlags(ctx context.Context, projectKey string, opts ld.ListFlagsOptions) ([]ld.FeatureFlag, error) {
	f.mu.Lock()
	f.listEnvKeys = append(f.listEnvKeys, strings.Join(opts.EnvKeys, ","))
	f.mu.Unlock()

	src := f.flags
	if opts.Archived {
		src = f.archivedFlags
	}
	scoped := make([]ld.FeatureFlag, len(src))
	for i, flag := range src {
		flag.Environments = scopeEnvs(flag.Environments, opts.EnvKeys)
		scoped[i] = flag
	}
	return scoped, nil
}

// scopeEnvs keeps only the requested environments, as the LD API does.
func scopeEnvs(envs map[string]ld.FlagEnvConfig, keys []string) map[string]ld.FlagEnvConfig {
	out := map[string]ld.FlagEnvConfig{}
	for _, key := range keys {
		if cfg, ok := envs[key]; ok {
			out[key] = cfg
		}
	}
	return out
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

// The list is full-width until enter opens the detail view; the camelCase key
// only exists in the detail view.
func TestDetailViewOpensOnEnterAndShowsCamelCaseKey(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
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

func TestEnterOpensDetailEscReturnsToList(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForOutput(t, tm, "esc back to list")

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	// The list footer is back, which the detail view never renders.
	waitForOutput(t, tm, "enter open detail")

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	m, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatal("final model is not tui.Model")
	}
	if m.flags.detailOpened() {
		t.Error("detail view still open after esc")
	}
}

// Toggling must stay one keypress away from the list, without opening detail.
func TestSpaceTogglesFromList(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	tm.Send(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
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
	waitForOutput(t, tm, "old-legacy-flag ⚑")

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

// The matrix shows every environment as a column: the header carries the env
// keys and one row carries that flag's state in each.
func TestMatrixRendersEnvironmentColumns(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	m, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatal("final model is not tui.Model")
	}
	view := m.View()
	for _, want := range []string{"staging", "product", "connect1-rollout-geofencing"} {
		if !strings.Contains(view, want) {
			t.Errorf("matrix view missing %q:\n%s", want, view)
		}
	}
	// staging is off and production is on for this flag: both dots present.
	if !strings.Contains(view, "○") || !strings.Contains(view, "●") {
		t.Errorf("expected both an on and an off cell in one row:\n%s", view)
	}
}

// Regression: the matrix must ask for every environment by key. Relying on
// LaunchDarkly to default to "all environments" returned flags with an empty
// Environments map, so every cell rendered as "–" (no config).
func TestFlagsAreFetchedForEveryEnvironment(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.listEnvKeys) == 0 {
		t.Fatal("ListFlags was never called")
	}
	for _, got := range fake.listEnvKeys {
		if got != "staging,production" {
			t.Errorf("ListFlags EnvKeys = %q, want every environment (%q)", got, "staging,production")
		}
	}

	m, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatal("final model is not tui.Model")
	}
	if n := len(m.rawFlags[0].Environments); n != 2 {
		t.Errorf("flag carries %d environment configs, want 2 — the matrix would render dashes", n)
	}
}

// Moving the column cursor changes which environment `t` acts on — including
// which guardrail applies. staging is unguarded, production is critical.
func TestEnvColumnCursorDrivesToggleTarget(t *testing.T) {
	fake := newFake()
	tm := startApp(t, fake, "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	// → moves the cursor to production, so `t` must now demand the typed key.
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	waitForOutput(t, tm, "in production?")

	// "y" alone must not confirm a guarded env.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	fake.mu.Lock()
	if len(fake.toggles) != 0 {
		t.Errorf("toggle fired without typed confirmation: %v", fake.toggles)
	}
	fake.mu.Unlock()

	tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("production")})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForOutput(t, tm, "is now off in production")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.toggles) != 1 || fake.toggles[0] != "connect1-rollout-geofencing:production:off" {
		t.Errorf("toggles = %v, want a single production toggle", fake.toggles)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// The cursor must not run off either end of the environment list.
func TestEnvColumnCursorClampsAtEnds(t *testing.T) {
	tm := startApp(t, newFake(), "staging")
	waitForOutput(t, tm, "connect1-rollout-geofencing")

	// staging is first; ← should stay put rather than wrap to production.
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	// production is last; two → moves land on it and stop.
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	m, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatal("final model is not tui.Model")
	}
	if m.env != "production" {
		t.Errorf("env = %q, want production", m.env)
	}
}

// A flag with no config in an environment is a different fact from "off".
func TestMissingEnvConfigRendersDash(t *testing.T) {
	layout := computeLayout(100, 2, 0)
	d := &matrixDelegate{
		envs:   []ld.Environment{{Key: "staging"}, {Key: "production"}},
		layout: layout,
	}
	flag := ld.FeatureFlag{
		Key:          "only-in-staging",
		Environments: map[string]ld.FlagEnvConfig{"staging": {On: true}},
	}
	cells := d.renderCells(flag)
	if !strings.Contains(cells, "●") {
		t.Errorf("expected an on dot for staging, got %q", cells)
	}
	if !strings.Contains(cells, "–") {
		t.Errorf("expected a dash for the missing production config, got %q", cells)
	}
	if strings.Contains(cells, "○") {
		t.Errorf("missing env config rendered as off rather than absent: %q", cells)
	}
}

// The header and every row must agree on column widths at any terminal size,
// and no row may exceed the width it was given.
func TestMatrixHeaderAndRowsAlign(t *testing.T) {
	envs := []ld.Environment{
		{Key: "dev"}, {Key: "test"}, {Key: "staging"},
		{Key: "production"}, {Key: "sandbox"}, {Key: "demo"},
	}
	flag := ld.FeatureFlag{
		Key: "connect-1-usage-summary-previous-average-series",
		Environments: map[string]ld.FlagEnvConfig{
			"dev": {On: true}, "test": {On: false}, "staging": {On: true},
		},
	}
	for _, width := range []int{40, 60, 80, 100, 120, 200} {
		for _, n := range []int{1, 2, 4, 6} {
			d := &matrixDelegate{envs: envs[:n]}
			d.layout = computeLayout(width, n, 0)

			header := d.header()
			row := strings.Repeat(" ", gutterWidth) +
				cell(truncateMiddle(flag.Key, d.layout.keyWidth), d.layout.keyWidth, lipgloss.Left) +
				d.renderCells(flag)
			if d.layout.showStatus {
				row += cell("launched", statusWidth, lipgloss.Left)
			}

			if lipgloss.Width(header) != lipgloss.Width(row) {
				t.Errorf("w=%d n=%d: header width %d != row width %d\nheader=%q\nrow=%q",
					width, n, lipgloss.Width(header), lipgloss.Width(row), header, row)
			}
			if got := lipgloss.Width(row); got > width {
				t.Errorf("w=%d n=%d: row width %d overflows terminal", width, n, got)
			}
			if strings.Contains(row, "\n") || strings.Contains(header, "\n") {
				t.Errorf("w=%d n=%d: matrix line wrapped; a Height()==1 row must stay one line", width, n)
			}
		}
	}
}

func TestTruncateMiddleKeepsBothEnds(t *testing.T) {
	const key = "connect-1-usage-summary-previous-average-series"
	got := truncateMiddle(key, 28)
	if lipgloss.Width(got) != 28 {
		t.Errorf("truncateMiddle width = %d, want 28 (%q)", lipgloss.Width(got), got)
	}
	if !strings.HasPrefix(got, "connect-1-") {
		t.Errorf("lost the identifying prefix: %q", got)
	}
	if !strings.HasSuffix(got, "series") {
		t.Errorf("lost the distinguishing tail: %q", got)
	}
	if short := truncateMiddle("tiny", 28); short != "tiny" {
		t.Errorf("short key was altered: %q", short)
	}
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
