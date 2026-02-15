package contentful

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	servicekit "github.com/alberto-moreno-sa/go-service-kit/contentful"
)

// Client embeds the SDK client and adds project-specific methods.
type Client struct {
	*servicekit.Client
}

// NewClient creates a new Contentful client with SDK and project support.
func NewClient(spaceID, token string) *Client {
	return &Client{
		Client: servicekit.NewClient(spaceID, token),
	}
}

// GetProjects fetches the projects siteSection entry.
// First tries by direct entry ID. If that fails with 404, falls back to
// querying by content_type=siteSection and fields.sectionId=projects.
func (c *Client) GetProjects(ctx context.Context, entryID string) (*ProjectsResult, error) {
	entry, err := c.GetEntry(ctx, entryID)
	if err != nil {
		// Fallback: query by sectionId (in case entryID is the sectionId, not the real ID)
		entry, err = c.findProjectsBySectionID(ctx, entryID)
		if err != nil {
			return nil, fmt.Errorf("get projects entry: %w", err)
		}
	}

	contentField, ok := entry.Fields["content"]
	if !ok {
		return &ProjectsResult{
			EntryID:   entry.Sys.ID,
			Version:   entry.Sys.Version,
			RawFields: entry.Fields,
		}, nil
	}

	localeMap, ok := contentField.(map[string]interface{})
	if !ok {
		return &ProjectsResult{
			EntryID:   entry.Sys.ID,
			Version:   entry.Sys.Version,
			RawFields: entry.Fields,
		}, fmt.Errorf("content field is not locale-wrapped")
	}

	rawContent, ok := localeMap["en-US"]
	if !ok {
		for _, v := range localeMap {
			rawContent = v
			break
		}
	}

	contentBytes, err := json.Marshal(rawContent)
	if err != nil {
		return nil, fmt.Errorf("marshal content: %w", err)
	}

	var projects []Project
	if err := json.Unmarshal(contentBytes, &projects); err != nil {
		return nil, fmt.Errorf("unmarshal projects: %w", err)
	}

	return &ProjectsResult{
		Projects:  projects,
		EntryID:   entry.Sys.ID,
		Version:   entry.Sys.Version,
		RawFields: entry.Fields,
	}, nil
}

// UpdateProjects updates the projects entry using the fetch-mutate-put pattern.
func (c *Client) UpdateProjects(ctx context.Context, result *ProjectsResult, projects []Project) (int, error) {
	endpoint := fmt.Sprintf("%s/spaces/%s/environments/master/entries/%s",
		servicekit.CMABaseURL, c.SpaceID, result.EntryID)

	fields := make(map[string]interface{})
	for k, v := range result.RawFields {
		fields[k] = v
	}
	fields["content"] = map[string]interface{}{
		"en-US": projects,
	}

	body := map[string]interface{}{
		"fields": fields,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	req.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", result.Version))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, fmt.Errorf("CMA update failed (%d): could not read body: %w", resp.StatusCode, err)
		}
		return 0, fmt.Errorf("CMA update failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var updated servicekit.EntryItem
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return 0, fmt.Errorf("decode update response: %w", err)
	}

	return updated.Sys.Version, nil
}

// findProjectsBySectionID queries for a siteSection entry by sectionId field.
func (c *Client) findProjectsBySectionID(ctx context.Context, sectionID string) (*servicekit.EntryItem, error) {
	endpoint := fmt.Sprintf("%s/spaces/%s/environments/master/entries", servicekit.CMABaseURL, c.SpaceID)

	params := url.Values{}
	params.Set("content_type", "siteSection")
	params.Set("fields.sectionId", sectionID)
	params.Set("limit", "1")

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("CMA query failed (%d): could not read body: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("CMA query failed (%d): %s", resp.StatusCode, string(body))
	}

	var result servicekit.EntriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no siteSection entry found with sectionId=%q", sectionID)
	}

	return &result.Items[0], nil
}
