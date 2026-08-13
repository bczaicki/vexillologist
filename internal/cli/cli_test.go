package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
