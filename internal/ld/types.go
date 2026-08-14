package ld

import "time"

// Project is a LaunchDarkly project.
type Project struct {
	Key  string   `json:"key"`
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

// Environment is a LaunchDarkly environment within a project.
type Environment struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Color          string `json:"color,omitempty"`
	Critical       bool   `json:"critical"`
	ConfirmChanges bool   `json:"confirmChanges"`
}

// Variation is one possible value a flag can serve.
type Variation struct {
	ID          string `json:"_id,omitempty"`
	Value       any    `json:"value"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Rollout describes a percentage rollout across variations.
type Rollout struct {
	Variations []WeightedVariation `json:"variations,omitempty"`
}

// WeightedVariation is one slice of a percentage rollout. Weight is in
// hundredths of a percent (0-100000).
type WeightedVariation struct {
	Variation int `json:"variation"`
	Weight    int `json:"weight"`
}

// Rule is a targeting rule (displayed read-only in v1).
type Rule struct {
	ID          string   `json:"_id,omitempty"`
	Description string   `json:"description,omitempty"`
	Clauses     []Clause `json:"clauses,omitempty"`
	Variation   *int     `json:"variation,omitempty"`
	Rollout     *Rollout `json:"rollout,omitempty"`
}

// Clause is one condition within a targeting rule.
type Clause struct {
	Attribute string `json:"attribute"`
	Op        string `json:"op"`
	Values    []any  `json:"values"`
	Negate    bool   `json:"negate"`
}

// Target is an individual-target list for one variation.
type Target struct {
	Values    []string `json:"values"`
	Variation int      `json:"variation"`
}

// Fallthrough is the default rule served when targeting is on and no rule matches.
type Fallthrough struct {
	Variation *int     `json:"variation,omitempty"`
	Rollout   *Rollout `json:"rollout,omitempty"`
}

// FlagEnvConfig is a flag's per-environment targeting state.
type FlagEnvConfig struct {
	On             bool         `json:"on"`
	Archived       bool         `json:"archived"`
	Targets        []Target     `json:"targets,omitempty"`
	Rules          []Rule       `json:"rules,omitempty"`
	Fallthrough    *Fallthrough `json:"fallthrough,omitempty"`
	OffVariation   *int         `json:"offVariation,omitempty"`
	LastModified   int64        `json:"lastModified,omitempty"`
	EnvironmentKey string       `json:"-"`
}

// FeatureFlag is a LaunchDarkly feature flag. Environments holds only the
// environments the API was asked for (via ?env=...) unless the full resource
// was requested.
type FeatureFlag struct {
	Key          string                   `json:"key"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"`
	Kind         string                   `json:"kind"`
	Archived     bool                     `json:"archived"`
	Temporary    bool                     `json:"temporary"`
	Tags         []string                 `json:"tags,omitempty"`
	Variations   []Variation              `json:"variations,omitempty"`
	Maintainer   *Member                  `json:"_maintainer,omitempty"`
	Environments map[string]FlagEnvConfig `json:"environments,omitempty"`
	CreationDate int64                    `json:"creationDate,omitempty"`
}

// Member is a LaunchDarkly account member (used for flag maintainer).
type Member struct {
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
}

// FlagStatus is LaunchDarkly's per-environment evaluation status of a flag:
// "new" (never requested), "active" (receiving requests), "launched"
// (serving one variation to everyone — retirement candidate), or "inactive"
// (no requests for 7+ days — archive candidate).
type FlagStatus struct {
	Name          string     `json:"name"`
	LastRequested *time.Time `json:"lastRequested,omitempty"`
}

// CreateFlagParams is the body for creating a new flag. Variations defaults
// to boolean true/false when empty.
type CreateFlagParams struct {
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Variations  []Variation `json:"variations,omitempty"`
	Temporary   bool        `json:"temporary,omitempty"`
}

// paginated wrappers for LD collection responses.

type projectsResponse struct {
	Items      []Project `json:"items"`
	TotalCount int       `json:"totalCount"`
}

type environmentsResponse struct {
	Items      []Environment `json:"items"`
	TotalCount int           `json:"totalCount"`
}

type flagsResponse struct {
	Items      []FeatureFlag `json:"items"`
	TotalCount int           `json:"totalCount"`
}
