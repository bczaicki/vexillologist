package ld

import (
	"testing"
	"time"
)

func flagWithEnv(key string, on bool, lastModified int64) FeatureFlag {
	return FeatureFlag{
		Key:  key,
		Name: key,
		Environments: map[string]FlagEnvConfig{
			"staging": {On: on, LastModified: lastModified},
		},
	}
}

func keysOf(flags []FeatureFlag) []string {
	out := make([]string, len(flags))
	for i, f := range flags {
		out[i] = f.Key
	}
	return out
}

func assertOrder(t *testing.T, flags []FeatureFlag, want ...string) {
	t.Helper()
	got := keysOf(flags)
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSortLongestOn(t *testing.T) {
	now := time.Now().UnixMilli()
	flags := []FeatureFlag{
		flagWithEnv("off-old", false, now-1000000),
		flagWithEnv("on-recent", true, now-1000),
		flagWithEnv("on-ancient", true, now-9000000),
		flagWithEnv("on-unknown-age", true, 0),
	}
	SortFlags(flags, SortLongestOn, "staging", nil)
	// On-flags first, oldest change first; unknown ages last within the on
	// partition; off-flags after.
	assertOrder(t, flags, "on-ancient", "on-recent", "on-unknown-age", "off-old")
}

func TestSortRecent(t *testing.T) {
	now := time.Now().UnixMilli()
	flags := []FeatureFlag{
		flagWithEnv("older", true, now-5000),
		flagWithEnv("newest", false, now-100),
		flagWithEnv("unknown", true, 0),
	}
	SortFlags(flags, SortRecent, "staging", nil)
	assertOrder(t, flags, "newest", "older", "unknown")
}

func TestSortCreated(t *testing.T) {
	flags := []FeatureFlag{
		{Key: "old", CreationDate: 100},
		{Key: "new", CreationDate: 300},
		{Key: "mid", CreationDate: 200},
	}
	SortFlags(flags, SortCreated, "staging", nil)
	assertOrder(t, flags, "new", "mid", "old")
}

func TestSortStatus(t *testing.T) {
	flags := []FeatureFlag{
		{Key: "busy"},
		{Key: "dead"},
		{Key: "done"},
		{Key: "mystery"},
	}
	statuses := map[string]FlagStatus{
		"busy": {Name: "active"},
		"dead": {Name: "inactive"},
		"done": {Name: "launched"},
	}
	SortFlags(flags, SortStatus, "staging", statuses)
	// Archive candidates first: inactive, launched; unknown statuses last.
	assertOrder(t, flags, "dead", "done", "busy", "mystery")
}

func TestParseSortMode(t *testing.T) {
	if _, err := ParseSortMode("longest-on"); err != nil {
		t.Errorf("longest-on rejected: %v", err)
	}
	if _, err := ParseSortMode("bogus"); err == nil {
		t.Error("bogus accepted")
	}
}
