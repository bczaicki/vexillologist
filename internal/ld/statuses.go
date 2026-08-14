package ld

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
)

// flagStatusesResponse is the body of GET /api/v2/flag-statuses/{proj}/{env}.
// Each item identifies its flag only via the parent link.
type flagStatusesResponse struct {
	Items []struct {
		Name          string     `json:"name"`
		LastRequested *time.Time `json:"lastRequested"`
		Links         struct {
			Parent struct {
				Href string `json:"href"`
			} `json:"parent"`
		} `json:"_links"`
	} `json:"items"`
}

// GetFlagStatuses returns every flag's evaluation status in one environment,
// keyed by flag key.
func (c *Client) GetFlagStatuses(ctx context.Context, projectKey, envKey string) (map[string]FlagStatus, error) {
	p := fmt.Sprintf("/api/v2/flag-statuses/%s/%s", url.PathEscape(projectKey), url.PathEscape(envKey))
	var resp flagStatusesResponse
	if err := c.do(ctx, http.MethodGet, p, nil, "", nil, &resp); err != nil {
		return nil, fmt.Errorf("getting flag statuses for %s/%s: %w", projectKey, envKey, err)
	}
	statuses := make(map[string]FlagStatus, len(resp.Items))
	for _, item := range resp.Items {
		// The flag key is the last segment of the parent link,
		// e.g. /api/v2/flags/my-proj/my-flag.
		key := path.Base(item.Links.Parent.Href)
		if key == "" || key == "." || key == "/" {
			continue
		}
		statuses[key] = FlagStatus{Name: item.Name, LastRequested: item.LastRequested}
	}
	return statuses, nil
}
