package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/restgrep-az/restgrep/internal/backend"
)

type Backend struct {
	Organization string
	Project      string
	BaseURL      string
	HTTPClient   *http.Client
	TokenFetcher func(ctx context.Context) (string, error)
}

func New(org, project string) *Backend {
	return &Backend{
		Organization: org,
		Project:      project,
		BaseURL:      "https://almsearch.dev.azure.com",
		HTTPClient:   http.DefaultClient,
		TokenFetcher: defaultTokenFetcher,
	}
}

func defaultTokenFetcher(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "az", "account", "get-access-token", "--query", "accessToken", "-o", "tsv")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch az token (make sure you are logged in using az login): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (b *Backend) Name() string {
	return "azure"
}

type SearchRequest struct {
	SearchText string              `json:"searchText"`
	Top        int                 `json:"$top"`
	Skip       int                 `json:"$skip"`
	Filters    map[string][]string `json:"filters,omitempty"`
}

type SearchResponse struct {
	Count   int `json:"count"`
	Results []struct {
		FileName string `json:"fileName"`
		Path     string `json:"path"`
		ContentId string `json:"contentId"`
		Matches  struct {
			Content []struct {
				CharOffset int `json:"charOffset"`
				Length     int `json:"length"`
			} `json:"content"`
		} `json:"matches"`
	} `json:"results"`
}

func (b *Backend) Search(ctx context.Context, query string, opts backend.SearchOptions) ([]backend.SearchResult, error) {
	token, err := b.TokenFetcher(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/_apis/search/codesearchresults?api-version=7.1", b.BaseURL, b.Organization)

	searchText := query
	if !opts.WordRegexp && !strings.ContainsAny(searchText, "*?") {
		searchText = "*" + searchText + "*"
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	reqBody := SearchRequest{
		SearchText: searchText,
		Top:        limit,
		Skip:       0,
	}
	if b.Project != "" {
		reqBody.Filters = map[string][]string{
			"Project": {b.Project},
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []backend.SearchResult
	for _, res := range searchResp.Results {
		if len(res.Matches.Content) > 0 {
			for _, match := range res.Matches.Content {
				results = append(results, backend.SearchResult{
					File:       res.Path,
					Line:       1, // Line numbers would require fetching file content
					Content:    fmt.Sprintf("[Match at char offset %d, length %d]", match.CharOffset, match.Length),
					ContentId:  res.ContentId,
					CharOffset: match.CharOffset,
					Length:     match.Length,
				})
			}
		} else {
			results = append(results, backend.SearchResult{
				File:      res.Path,
				Line:      1,
				Content:   "[File match]",
				ContentId: res.ContentId,
			})
		}
	}

	return results, nil
}
