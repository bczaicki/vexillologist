package ld

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ListFlagsOptions filters ListFlags results.
type ListFlagsOptions struct {
	// EnvKeys scopes each flag's Environments map to these environments.
	EnvKeys []string
	// Filter is a substring match on flag key/name (LD "query" filter).
	Filter string
	// Archived, when true, returns archived flags instead of active ones.
	Archived bool
}

// ListFlags returns flags in a project, paginating as needed. Summary
// representations are requested: variations are included but per-environment
// rules/targets are elided (use GetFlag for those).
func (c *Client) ListFlags(ctx context.Context, projectKey string, opts ListFlagsOptions) ([]FeatureFlag, error) {
	var all []FeatureFlag
	for offset := 0; ; offset += pageLimit {
		q := url.Values{
			"limit":   {strconv.Itoa(pageLimit)},
			"offset":  {strconv.Itoa(offset)},
			"summary": {"true"},
		}
		for _, env := range opts.EnvKeys {
			q.Add("env", env)
		}
		var filters []string
		if opts.Filter != "" {
			filters = append(filters, "query:"+opts.Filter)
		}
		if opts.Archived {
			filters = append(filters, "state:archived")
		}
		if len(filters) > 0 {
			q.Set("filter", strings.Join(filters, ","))
		}
		path := fmt.Sprintf("/api/v2/flags/%s", url.PathEscape(projectKey))
		var page flagsResponse
		if err := c.do(ctx, http.MethodGet, path, q, "", nil, &page); err != nil {
			return nil, fmt.Errorf("listing flags for %q: %w", projectKey, err)
		}
		all = append(all, page.Items...)
		if len(all) >= page.TotalCount || len(page.Items) == 0 {
			return all, nil
		}
	}
}

// GetFlag returns one flag's full representation, optionally scoping the
// Environments map to envKeys.
func (c *Client) GetFlag(ctx context.Context, projectKey, flagKey string, envKeys []string) (*FeatureFlag, error) {
	q := url.Values{}
	for _, env := range envKeys {
		q.Add("env", env)
	}
	path := fmt.Sprintf("/api/v2/flags/%s/%s", url.PathEscape(projectKey), url.PathEscape(flagKey))
	var flag FeatureFlag
	if err := c.do(ctx, http.MethodGet, path, q, "", nil, &flag); err != nil {
		return nil, fmt.Errorf("getting flag %q: %w", flagKey, err)
	}
	return &flag, nil
}

// CreateFlag creates a new flag. Empty Variations defaults to boolean
// true/false server-side.
func (c *Client) CreateFlag(ctx context.Context, projectKey string, params CreateFlagParams) (*FeatureFlag, error) {
	path := fmt.Sprintf("/api/v2/flags/%s", url.PathEscape(projectKey))
	var flag FeatureFlag
	if err := c.do(ctx, http.MethodPost, path, nil, "", params, &flag); err != nil {
		return nil, fmt.Errorf("creating flag %q: %w", params.Key, err)
	}
	return &flag, nil
}

// ToggleFlag turns a flag's targeting on or off in one environment via
// semantic patch.
func (c *Client) ToggleFlag(ctx context.Context, projectKey, flagKey, envKey string, on bool, comment string) (*FeatureFlag, error) {
	patch := ToggleFlagPatch(envKey, on, comment)
	flag, err := c.semanticPatchFlag(ctx, projectKey, flagKey, patch)
	if err != nil {
		return nil, fmt.Errorf("toggling flag %q in %q: %w", flagKey, envKey, err)
	}
	return flag, nil
}

// ArchiveFlag archives a flag across all environments via semantic patch.
func (c *Client) ArchiveFlag(ctx context.Context, projectKey, flagKey, comment string) (*FeatureFlag, error) {
	patch := ArchiveFlagPatch(comment)
	flag, err := c.semanticPatchFlag(ctx, projectKey, flagKey, patch)
	if err != nil {
		return nil, fmt.Errorf("archiving flag %q: %w", flagKey, err)
	}
	return flag, nil
}

// RestoreFlag restores an archived flag via semantic patch.
func (c *Client) RestoreFlag(ctx context.Context, projectKey, flagKey, comment string) (*FeatureFlag, error) {
	patch := RestoreFlagPatch(comment)
	flag, err := c.semanticPatchFlag(ctx, projectKey, flagKey, patch)
	if err != nil {
		return nil, fmt.Errorf("restoring flag %q: %w", flagKey, err)
	}
	return flag, nil
}

// UpdateFlagSettings changes a flag's flag-wide settings (SDK availability,
// temporary/permanent) via semantic patch.
func (c *Client) UpdateFlagSettings(ctx context.Context, projectKey, flagKey string, u FlagSettingsUpdate, comment string) (*FeatureFlag, error) {
	patch := FlagSettingsPatch(u, comment)
	flag, err := c.semanticPatchFlag(ctx, projectKey, flagKey, patch)
	if err != nil {
		return nil, fmt.Errorf("updating settings of flag %q: %w", flagKey, err)
	}
	return flag, nil
}

func (c *Client) semanticPatchFlag(ctx context.Context, projectKey, flagKey string, patch SemanticPatch) (*FeatureFlag, error) {
	path := fmt.Sprintf("/api/v2/flags/%s/%s", url.PathEscape(projectKey), url.PathEscape(flagKey))
	var flag FeatureFlag
	if err := c.do(ctx, http.MethodPatch, path, nil, contentTypeSemanticPatch, patch, &flag); err != nil {
		return nil, err
	}
	return &flag, nil
}
