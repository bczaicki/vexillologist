package ld

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListProjects returns every project in the account, paginating as needed.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var all []Project
	for offset := 0; ; offset += pageLimit {
		q := url.Values{
			"limit":  {strconv.Itoa(pageLimit)},
			"offset": {strconv.Itoa(offset)},
		}
		var page projectsResponse
		if err := c.do(ctx, http.MethodGet, "/api/v2/projects", q, "", nil, &page); err != nil {
			return nil, fmt.Errorf("listing projects: %w", err)
		}
		all = append(all, page.Items...)
		if len(all) >= page.TotalCount || len(page.Items) == 0 {
			return all, nil
		}
	}
}

// ListEnvironments returns every environment in a project, paginating as needed.
func (c *Client) ListEnvironments(ctx context.Context, projectKey string) ([]Environment, error) {
	var all []Environment
	for offset := 0; ; offset += pageLimit {
		q := url.Values{
			"limit":  {strconv.Itoa(pageLimit)},
			"offset": {strconv.Itoa(offset)},
		}
		path := fmt.Sprintf("/api/v2/projects/%s/environments", url.PathEscape(projectKey))
		var page environmentsResponse
		if err := c.do(ctx, http.MethodGet, path, q, "", nil, &page); err != nil {
			return nil, fmt.Errorf("listing environments for %q: %w", projectKey, err)
		}
		all = append(all, page.Items...)
		if len(all) >= page.TotalCount || len(page.Items) == 0 {
			return all, nil
		}
	}
}
