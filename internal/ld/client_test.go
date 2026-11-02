package ld

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// newTestClient returns a client pointed at a httptest server, with sleeping
// stubbed out.
func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("test-token", WithBaseURL(srv.URL))
	c.sleep = func(time.Duration) {}
	return c, srv
}

func TestAuthAndVersionHeaders(t *testing.T) {
	var gotAuth, gotVersion string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("LD-API-Version")
		fmt.Fprint(w, `{"items": [], "totalCount": 0}`)
	}))

	if _, err := c.ListProjects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "test-token" {
		t.Errorf("Authorization = %q, want raw token (no Bearer prefix)", gotAuth)
	}
	if gotVersion != "20240415" {
		t.Errorf("LD-API-Version = %q, want 20240415", gotVersion)
	}
}

func TestListProjectsPaginates(t *testing.T) {
	var offsets []string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		offset := r.URL.Query().Get("offset")
		offsets = append(offsets, offset)
		n, _ := strconv.Atoi(offset)
		items := make([]Project, 0, pageLimit)
		for i := n; i < n+pageLimit && i < 150; i++ {
			items = append(items, Project{Key: fmt.Sprintf("proj-%d", i)})
		}
		_ = json.NewEncoder(w).Encode(projectsResponse{Items: items, TotalCount: 150})
	}))

	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 150 {
		t.Errorf("got %d projects, want 150", len(projects))
	}
	if len(offsets) != 2 || offsets[0] != "0" || offsets[1] != "100" {
		t.Errorf("offsets = %v, want [0 100]", offsets)
	}
}

func TestListFlagsQueryParams(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/flags/my-proj" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("env") != "staging" {
			t.Errorf("env = %q, want staging", q.Get("env"))
		}
		if q.Get("summary") != "true" {
			t.Errorf("summary = %q, want true", q.Get("summary"))
		}
		if q.Get("filter") != "query:geofence,state:archived" {
			t.Errorf("filter = %q", q.Get("filter"))
		}
		_ = json.NewEncoder(w).Encode(flagsResponse{
			Items:      []FeatureFlag{{Key: "connect1-rollout-geofencing"}},
			TotalCount: 1,
		})
	}))

	flags, err := c.ListFlags(context.Background(), "my-proj", ListFlagsOptions{
		EnvKeys:  []string{"staging"},
		Filter:   "geofence",
		Archived: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0].Key != "connect1-rollout-geofencing" {
		t.Errorf("flags = %+v", flags)
	}
}

func TestToggleFlagSemanticPatch(t *testing.T) {
	var gotMethod, gotPath, gotContentType, gotBody string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		fmt.Fprint(w, `{"key": "my-flag", "environments": {"staging": {"on": true}}}`)
	}))

	flag, err := c.ToggleFlag(context.Background(), "my-proj", "my-flag", "staging", true, "via vex")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/api/v2/flags/my-proj/my-flag" {
		t.Errorf("path = %q", gotPath)
	}
	if gotContentType != "application/json; domain-model=launchdarkly.semanticpatch" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	want := `{"environmentKey":"staging","comment":"via vex","instructions":[{"kind":"turnFlagOn"}]}`
	if gotBody != want {
		t.Errorf("body = %s\nwant %s", gotBody, want)
	}
	if !flag.Environments["staging"].On {
		t.Error("expected returned flag to be on in staging")
	}
}

func TestArchiveFlagPatchIsFlagWide(t *testing.T) {
	var gotBody string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		fmt.Fprint(w, `{"key": "my-flag", "archived": true}`)
	}))

	flag, err := c.ArchiveFlag(context.Background(), "my-proj", "my-flag", "cleanup")
	if err != nil {
		t.Fatal(err)
	}
	// archiveFlag is flag-wide: no environmentKey in the patch.
	want := `{"comment":"cleanup","instructions":[{"kind":"archiveFlag"}]}`
	if gotBody != want {
		t.Errorf("body = %s\nwant %s", gotBody, want)
	}
	if !flag.Archived {
		t.Error("expected returned flag to be archived")
	}
}

func TestCreateFlag(t *testing.T) {
	var gotMethod, gotPath string
	var gotParams CreateFlagParams
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotParams)
		fmt.Fprint(w, `{"key": "new-flag", "name": "New Flag"}`)
	}))

	_, err := c.CreateFlag(context.Background(), "my-proj", CreateFlagParams{
		Key: "new-flag", Name: "New Flag", Tags: []string{"vex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v2/flags/my-proj" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if gotParams.Key != "new-flag" || len(gotParams.Tags) != 1 {
		t.Errorf("params = %+v", gotParams)
	}
}

// LD defaults an omitted "temporary" to true, so a permanent flag only comes
// out permanent if false is actually sent.
func TestCreateFlagSendsPermanentAndAvailability(t *testing.T) {
	var gotBody map[string]any
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		fmt.Fprint(w, `{"key": "new-flag"}`)
	}))

	_, err := c.CreateFlag(context.Background(), "my-proj", CreateFlagParams{
		Key: "new-flag", Name: "New Flag", Temporary: false,
		ClientSideAvailability: &ClientSideAvailability{UsingEnvironmentID: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if temporary, ok := gotBody["temporary"]; !ok || temporary != false {
		t.Errorf("temporary = %v (present: %v), want explicit false", temporary, ok)
	}
	avail, _ := gotBody["clientSideAvailability"].(map[string]any)
	if avail["usingEnvironmentId"] != true || avail["usingMobileKey"] != false {
		t.Errorf("clientSideAvailability = %v", gotBody["clientSideAvailability"])
	}
}

func TestUpdateFlagSettingsPatch(t *testing.T) {
	var gotMethod, gotContentType, gotBody string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		fmt.Fprint(w, `{"key": "my-flag", "temporary": false,
			"clientSideAvailability": {"usingEnvironmentId": false, "usingMobileKey": true}}`)
	}))

	off, on := false, true
	flag, err := c.UpdateFlagSettings(context.Background(), "my-proj", "my-flag",
		FlagSettingsUpdate{ClientSide: &off, Permanent: &on}, "via vex")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPatch || gotContentType != "application/json; domain-model=launchdarkly.semanticpatch" {
		t.Errorf("request = %s (%s)", gotMethod, gotContentType)
	}
	// Flag-wide, and only the settings that were set: mobile is untouched.
	want := `{"comment":"via vex","instructions":[` +
		`{"kind":"turnOffClientSideAvailability","value":"usingEnvironmentId"},` +
		`{"kind":"makeFlagPermanent"}]}`
	if gotBody != want {
		t.Errorf("body = %s\nwant %s", gotBody, want)
	}
	if got := flag.SettingsSummary(); got != "client-side SDKs off · mobile SDKs on · permanent" {
		t.Errorf("SettingsSummary = %q", got)
	}
}

func TestFlagSettingsUpdateHelpers(t *testing.T) {
	off, on := false, true
	cases := []struct {
		update      FlagSettingsUpdate
		disablesSDK bool
		describe    string
	}{
		{FlagSettingsUpdate{}, false, ""},
		{FlagSettingsUpdate{ClientSide: &on, Permanent: &off}, false, "client-side SDKs on, temporary"},
		{FlagSettingsUpdate{Mobile: &off}, true, "mobile SDKs off"},
		{FlagSettingsUpdate{ClientSide: &off, Mobile: &on}, true, "client-side SDKs off, mobile SDKs on"},
	}
	for _, tc := range cases {
		if got := tc.update.DisablesSDK(); got != tc.disablesSDK {
			t.Errorf("%q: DisablesSDK = %v", tc.describe, got)
		}
		if got := tc.update.Describe(); got != tc.describe {
			t.Errorf("Describe = %q, want %q", got, tc.describe)
		}
	}
	if !(FlagSettingsUpdate{}).Empty() || (FlagSettingsUpdate{Mobile: &off}).Empty() {
		t.Error("Empty misreports")
	}
}

func TestRateLimitRetriesOnce(t *testing.T) {
	attempts := 0
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("X-Ratelimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).UnixMilli(), 10))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"items": [], "totalCount": 0}`)
	}))

	if _, err := c.ListProjects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestRateLimitGivesUpAfterRetry(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	_, err := c.ListProjects(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("err = %v, want APIError 429", err)
	}
}

func TestAPIErrorDecoding(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"code": "not_found", "message": "flag not found"}`)
	}))

	_, err := c.GetFlag(context.Background(), "my-proj", "nope", nil)
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("not an APIError")
	}
	if apiErr.Code != "not_found" || apiErr.Message != "flag not found" {
		t.Errorf("decoded = %+v", apiErr)
	}
}
