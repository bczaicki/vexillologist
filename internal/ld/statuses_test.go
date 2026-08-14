package ld

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestGetFlagStatuses(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/flag-statuses/my-proj/staging" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"items": [
			{"name": "launched", "lastRequested": "2026-08-01T12:00:00Z",
			 "_links": {"parent": {"href": "/api/v2/flags/my-proj/old-flag"}}},
			{"name": "active", "lastRequested": null,
			 "_links": {"parent": {"href": "/api/v2/flags/my-proj/busy-flag"}}}
		]}`)
	}))

	statuses, err := c.GetFlagStatuses(context.Background(), "my-proj", "staging")
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %v", statuses)
	}
	if statuses["old-flag"].Name != "launched" || statuses["old-flag"].LastRequested == nil {
		t.Errorf("old-flag = %+v", statuses["old-flag"])
	}
	if statuses["busy-flag"].Name != "active" || statuses["busy-flag"].LastRequested != nil {
		t.Errorf("busy-flag = %+v", statuses["busy-flag"])
	}
}

func TestRestoreFlagPatch(t *testing.T) {
	var gotBody, gotContentType string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotContentType = r.Header.Get("Content-Type")
		fmt.Fprint(w, `{"key": "my-flag", "archived": false}`)
	}))

	flag, err := c.RestoreFlag(context.Background(), "my-proj", "my-flag", "bring it back")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"comment":"bring it back","instructions":[{"kind":"restoreFlag"}]}`
	if gotBody != want {
		t.Errorf("body = %s\nwant %s", gotBody, want)
	}
	if gotContentType != "application/json; domain-model=launchdarkly.semanticpatch" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if flag.Archived {
		t.Error("expected restored flag to be unarchived")
	}
}
