package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bczaicki/vexillologist/internal/ld"
)

// runVex executes the command tree with args and returns stdout and the error.
func runVex(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Isolate from the developer's real env/config.
	for _, v := range []string{"LD_API_TOKEN", "LD_ACCESS_TOKEN", "LAUNCHDARKLY_API_TOKEN", "LD_PROJECT", "LD_ENVIRONMENT"} {
		t.Setenv(v, "")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	root := NewRootCmd(nil)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), err
}

func newLDServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestMissingTokenIsUsageError(t *testing.T) {
	_, err := runVex(t, "projects", "list")
	if err == nil {
		t.Fatal("want error")
	}
	if code := exitCodeFor(err); code != ExitUsage {
		t.Errorf("exit code = %d, want %d (err: %v)", code, ExitUsage, err)
	}
}

func TestMissingProjectIsUsageError(t *testing.T) {
	_, err := runVex(t, "flags", "list", "--api-token", "tok")
	if err == nil {
		t.Fatal("want error")
	}
	if code := exitCodeFor(err); code != ExitUsage {
		t.Errorf("exit code = %d, want %d (err: %v)", code, ExitUsage, err)
	}
	if !strings.Contains(err.Error(), "no project") {
		t.Errorf("err = %v", err)
	}
}

func TestProjectsListJSON(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items": [{"key": "horizon", "name": "Horizon"}], "totalCount": 1}`)
	})

	out, err := runVex(t, "projects", "list", "--json", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var projects []map[string]any
	if err := json.Unmarshal([]byte(out), &projects); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if len(projects) != 1 || projects[0]["key"] != "horizon" {
		t.Errorf("projects = %v", projects)
	}
}

func TestFlagsListTable(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items": [{"key": "my-flag", "name": "My Flag",
			"environments": {"staging": {"on": true}}}], "totalCount": 1}`)
	})

	out, err := runVex(t, "flags", "list", "-p", "horizon", "-e", "staging",
		"--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "my-flag") || !strings.Contains(out, "on") {
		t.Errorf("table output missing flag row:\n%s", out)
	}
}

func TestToggleRequiresOnOrOff(t *testing.T) {
	_, err := runVex(t, "flags", "toggle", "my-flag", "-p", "horizon", "-e", "staging", "--api-token", "tok")
	if err == nil || exitCodeFor(err) != ExitUsage {
		t.Errorf("err = %v, want usage error", err)
	}
}

func TestToggleWithoutTTYRequiresYes(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items": [{"key": "staging", "name": "Staging"}], "totalCount": 1}`)
	})

	_, err := runVex(t, "flags", "toggle", "my-flag", "--on",
		"-p", "horizon", "-e", "staging", "--api-token", "tok", "--base-url", srv.URL)
	if err == nil || exitCodeFor(err) != ExitConfirmation {
		t.Errorf("err = %v (code %d), want confirmation error", err, exitCodeFor(err))
	}
}

func TestToggleWithYesSendsSemanticPatch(t *testing.T) {
	var patchBody string
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/environments"):
			fmt.Fprint(w, `{"items": [{"key": "staging", "name": "Staging", "critical": false}], "totalCount": 1}`)
		case r.Method == http.MethodPatch:
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r.Body)
			patchBody = buf.String()
			fmt.Fprint(w, `{"key": "my-flag", "environments": {"staging": {"on": true}}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	out, err := runVex(t, "flags", "toggle", "my-flag", "--on", "--yes", "--json",
		"-p", "horizon", "-e", "staging", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patchBody, `"turnFlagOn"`) || !strings.Contains(patchBody, `"environmentKey":"staging"`) {
		t.Errorf("patch = %s", patchBody)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["action"] != "turnFlagOn" || result["environment"] != "staging" {
		t.Errorf("result = %v", result)
	}
}

func TestToggleCriticalEnvEchoesCritical(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"items": [{"key": "production", "name": "Production", "critical": true}], "totalCount": 1}`)
			return
		}
		fmt.Fprint(w, `{"key": "my-flag"}`)
	})

	out, err := runVex(t, "flags", "toggle", "my-flag", "--off", "--yes", "--json",
		"-p", "horizon", "-e", "production", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["critical"] != true {
		t.Errorf("result = %v, want critical: true", result)
	}
}

func TestToggleDryRunSendsNothing(t *testing.T) {
	requests := 0
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	})

	out, err := runVex(t, "flags", "toggle", "my-flag", "--on", "--dry-run", "--json",
		"-p", "horizon", "-e", "staging", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Errorf("dry run sent %d requests, want 0", requests)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["dryRun"] != true {
		t.Errorf("result = %v, want dryRun: true", result)
	}
	patch, _ := json.Marshal(result["patch"])
	if !strings.Contains(string(patch), "turnFlagOn") {
		t.Errorf("patch = %s", patch)
	}
}

func TestArchiveWithoutTTYRequiresYes(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := runVex(t, "flags", "archive", "my-flag",
		"-p", "horizon", "--api-token", "tok", "--base-url", srv.URL)
	if err == nil || exitCodeFor(err) != ExitConfirmation {
		t.Errorf("err = %v, want confirmation error", err)
	}
}

func TestNotFoundExitCode(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"code": "not_found", "message": "flag not found"}`)
	})

	_, err := runVex(t, "flags", "get", "nope", "-p", "horizon",
		"--api-token", "tok", "--base-url", srv.URL)
	if err == nil || exitCodeFor(err) != ExitNotFound {
		t.Errorf("err = %v (code %d), want not-found", err, exitCodeFor(err))
	}
}

func TestFlagsListRejectsBogusSort(t *testing.T) {
	_, err := runVex(t, "flags", "list", "-p", "horizon", "-e", "staging",
		"--api-token", "tok", "--sort", "bogus")
	if err == nil || exitCodeFor(err) != ExitUsage {
		t.Errorf("err = %v, want usage error", err)
	}
}

func TestFlagsListSortLongestOn(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "flag-statuses") {
			fmt.Fprint(w, `{"items": []}`)
			return
		}
		fmt.Fprint(w, `{"items": [
			{"key": "recently-toggled", "name": "Recent", "environments": {"staging": {"on": true, "lastModified": 1770000000000}}},
			{"key": "ancient-on", "name": "Ancient", "environments": {"staging": {"on": true, "lastModified": 1600000000000}}},
			{"key": "off-flag", "name": "Off", "environments": {"staging": {"on": false, "lastModified": 1500000000000}}}
		], "totalCount": 3}`)
	})

	out, err := runVex(t, "flags", "list", "-p", "horizon", "-e", "staging", "--json",
		"--api-token", "tok", "--base-url", srv.URL, "--sort", "longest-on")
	if err != nil {
		t.Fatal(err)
	}
	var flags []map[string]any
	if err := json.Unmarshal([]byte(out), &flags); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	got := []string{flags[0]["key"].(string), flags[1]["key"].(string), flags[2]["key"].(string)}
	want := []string{"ancient-on", "recently-toggled", "off-flag"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestFlagsStatusesJSON(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "flag-statuses") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"items": [
			{"name": "inactive", "lastRequested": "2026-06-01T00:00:00Z",
			 "_links": {"parent": {"href": "/api/v2/flags/horizon/dead-flag"}}}
		]}`)
	})

	out, err := runVex(t, "flags", "statuses", "-p", "horizon", "-e", "staging", "--json",
		"--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var statuses map[string]map[string]any
	if err := json.Unmarshal([]byte(out), &statuses); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if statuses["dead-flag"]["name"] != "inactive" {
		t.Errorf("statuses = %v", statuses)
	}
}

func TestRestoreWithYes(t *testing.T) {
	var patchBody string
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r.Body)
			patchBody = buf.String()
			fmt.Fprint(w, `{"key": "old-flag", "archived": false}`)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	out, err := runVex(t, "flags", "restore", "old-flag", "--yes", "--json",
		"-p", "horizon", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patchBody, `"restoreFlag"`) {
		t.Errorf("patch = %s", patchBody)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || result["action"] != "restoreFlag" {
		t.Errorf("result = %v", result)
	}
}

func TestRestoreWithoutTTYRequiresYes(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := runVex(t, "flags", "restore", "old-flag",
		"-p", "horizon", "--api-token", "tok", "--base-url", srv.URL)
	if err == nil || exitCodeFor(err) != ExitConfirmation {
		t.Errorf("err = %v, want confirmation error", err)
	}
}

func TestCreateFlagJSON(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		fmt.Fprint(w, `{"key": "new-flag", "name": "New Flag"}`)
	})

	out, err := runVex(t, "flags", "create", "new-flag", "--name", "New Flag", "--json",
		"-p", "horizon", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || result["action"] != "createFlag" {
		t.Errorf("result = %v", result)
	}
}

// createBody runs `flags create` with extra args and returns the JSON body
// LaunchDarkly received.
func createBody(t *testing.T, extra ...string) map[string]any {
	t.Helper()
	var body map[string]any
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, `{"key": "new-flag", "name": "new-flag"}`)
	})
	args := append([]string{"flags", "create", "new-flag", "-p", "horizon", "--api-token", "tok", "--base-url", srv.URL}, extra...)
	if _, err := runVex(t, args...); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCreateFlagDefaultsToClientSideTemporary(t *testing.T) {
	body := createBody(t)
	avail, _ := body["clientSideAvailability"].(map[string]any)
	if avail["usingEnvironmentId"] != true || avail["usingMobileKey"] != false {
		t.Errorf("clientSideAvailability = %v, want client-side on, mobile off", body["clientSideAvailability"])
	}
	if body["temporary"] != true {
		t.Errorf("temporary = %v, want true", body["temporary"])
	}
}

func TestCreateFlagSettingsFlags(t *testing.T) {
	body := createBody(t, "--client-side=false", "--mobile", "--permanent")
	avail, _ := body["clientSideAvailability"].(map[string]any)
	if avail["usingEnvironmentId"] != false || avail["usingMobileKey"] != true {
		t.Errorf("clientSideAvailability = %v", body["clientSideAvailability"])
	}
	if body["temporary"] != false {
		t.Errorf("temporary = %v, want explicit false", body["temporary"])
	}
}

func TestCreateFlagTemporaryAndPermanentConflict(t *testing.T) {
	_, err := runVex(t, "flags", "create", "new-flag", "--temporary", "--permanent",
		"-p", "horizon", "--api-token", "tok")
	if err == nil || exitCodeFor(err) != ExitUsage {
		t.Errorf("err = %v, want usage error", err)
	}
}

func TestFlagSettingsDryRunSendsOnlyPassedSettings(t *testing.T) {
	out, err := runVex(t, "flags", "settings", "my-flag", "--mobile", "--permanent=false",
		"--dry-run", "--json", "-p", "horizon", "--api-token", "tok")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Action string           `json:"action"`
		Patch  ld.SemanticPatch `json:"patch"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	want := []ld.Instruction{
		{Kind: "turnOnClientSideAvailability", Value: "usingMobileKey"},
		{Kind: "makeFlagTemporary"},
	}
	if result.Action != "updateFlagSettings" || !reflect.DeepEqual(result.Patch.Instructions, want) {
		t.Errorf("result = %+v", result)
	}
}

func TestFlagSettingsRequiresASetting(t *testing.T) {
	_, err := runVex(t, "flags", "settings", "my-flag", "-p", "horizon", "--api-token", "tok")
	if err == nil || exitCodeFor(err) != ExitUsage {
		t.Errorf("err = %v, want usage error", err)
	}
}

func TestFlagSettingsHeadlessRequiresYes(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := runVex(t, "flags", "settings", "my-flag", "--client-side=false",
		"-p", "horizon", "--api-token", "tok", "--base-url", srv.URL)
	if err == nil || exitCodeFor(err) != ExitConfirmation {
		t.Errorf("err = %v, want confirmation error", err)
	}
}

func TestFlagSettingsJSON(t *testing.T) {
	srv := newLDServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v2/flags/horizon/my-flag" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"key": "my-flag", "temporary": false}`)
	})

	out, err := runVex(t, "flags", "settings", "my-flag", "--permanent", "--yes", "--json",
		"-p", "horizon", "--api-token", "tok", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || result["action"] != "updateFlagSettings" {
		t.Errorf("result = %v", result)
	}
}
