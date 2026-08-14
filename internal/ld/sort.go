package ld

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// SortMode orders a flag list. All sorting is client-side over the already
// fetched flags.
type SortMode string

const (
	// SortKey orders by flag key A→Z (the default).
	SortKey SortMode = "key"
	// SortName orders by flag name A→Z.
	SortName SortMode = "name"
	// SortCreated orders newest-created first.
	SortCreated SortMode = "created"
	// SortLongestOn puts flags that are ON in the environment first, the
	// longest-unchanged first — "what has been on the longest".
	SortLongestOn SortMode = "longest-on"
	// SortRecent orders by most recent change in the environment.
	SortRecent SortMode = "recent"
	// SortStatus surfaces archive candidates first: inactive (no evaluations
	// for 7+ days), then launched (fully rolled out), then new, then active.
	SortStatus SortMode = "status"
)

// SortModes lists every mode in TUI-cycle order.
var SortModes = []SortMode{SortKey, SortName, SortCreated, SortLongestOn, SortRecent, SortStatus}

// ParseSortMode validates a --sort value.
func ParseSortMode(s string) (SortMode, error) {
	for _, mode := range SortModes {
		if string(mode) == s {
			return mode, nil
		}
	}
	names := make([]string, len(SortModes))
	for i, mode := range SortModes {
		names[i] = string(mode)
	}
	return "", fmt.Errorf("unknown sort mode %q (valid: %s)", s, strings.Join(names, ", "))
}

// statusRank orders statuses by how strongly they suggest archiving.
var statusRank = map[string]int{"inactive": 0, "launched": 1, "new": 2, "active": 3}

// SortFlags sorts flags in place. env scopes the per-environment modes;
// statuses (from GetFlagStatuses) is only consulted by SortStatus and may be
// nil for the other modes.
func SortFlags(flags []FeatureFlag, mode SortMode, env string, statuses map[string]FlagStatus) {
	less := func(a, b *FeatureFlag) bool { return a.Key < b.Key }

	switch mode {
	case SortName:
		less = func(a, b *FeatureFlag) bool {
			if !strings.EqualFold(a.Name, b.Name) {
				return strings.ToLower(a.Name) < strings.ToLower(b.Name)
			}
			return a.Key < b.Key
		}

	case SortCreated:
		less = func(a, b *FeatureFlag) bool {
			if a.CreationDate != b.CreationDate {
				return a.CreationDate > b.CreationDate
			}
			return a.Key < b.Key
		}

	case SortLongestOn:
		less = func(a, b *FeatureFlag) bool {
			aOn, aAge := envState(a, env, math.MaxInt64)
			bOn, bAge := envState(b, env, math.MaxInt64)
			if aOn != bOn {
				return aOn // on-flags first
			}
			if aAge != bAge {
				return aAge < bAge // older change first; unknowns last
			}
			return a.Key < b.Key
		}

	case SortRecent:
		less = func(a, b *FeatureFlag) bool {
			_, aAge := envState(a, env, 0)
			_, bAge := envState(b, env, 0)
			if aAge != bAge {
				return aAge > bAge // newest change first; unknowns last
			}
			return a.Key < b.Key
		}

	case SortStatus:
		less = func(a, b *FeatureFlag) bool {
			aRank := rankOf(a.Key, statuses)
			bRank := rankOf(b.Key, statuses)
			if aRank != bRank {
				return aRank < bRank
			}
			return a.Key < b.Key
		}
	}

	sort.SliceStable(flags, func(i, j int) bool { return less(&flags[i], &flags[j]) })
}

// envState returns whether the flag is on in env and its last-modified epoch
// millis there; unknown substitutes for a missing timestamp so each mode can
// pick the sentinel that sorts unknowns last.
func envState(f *FeatureFlag, env string, unknown int64) (on bool, lastModified int64) {
	envCfg, ok := f.Environments[env]
	if !ok || envCfg.LastModified == 0 {
		return envCfg.On, unknown
	}
	return envCfg.On, envCfg.LastModified
}

func rankOf(key string, statuses map[string]FlagStatus) int {
	status, ok := statuses[key]
	if !ok {
		return len(statusRank) // unknown status sorts last
	}
	rank, ok := statusRank[status.Name]
	if !ok {
		return len(statusRank)
	}
	return rank
}
