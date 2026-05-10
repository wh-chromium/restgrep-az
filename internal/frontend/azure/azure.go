package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/wh-chromium/restgrep-az/internal/models"
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
		FileName  string `json:"fileName"`
		Path      string `json:"path"`
		ContentId string `json:"contentId"`
		Matches   struct {
			Content []struct {
				CharOffset int `json:"charOffset"`
				Length     int `json:"length"`
			} `json:"content"`
		} `json:"matches"`
	} `json:"results"`
}

func (b *Backend) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.IntermediateResult, error) {
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

	filters := make(map[string][]string)
	if b.Project != "" {
		filters["Project"] = []string{b.Project}
	}
	if len(opts.Paths) > 0 {
		var paths []string
		for _, p := range opts.Paths {
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			paths = append(paths, p)
		}
		filters["Path"] = paths
	}
	if len(filters) > 0 {
		reqBody.Filters = filters
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	if opts.Debug {
		fmt.Printf("[DEBUG][azure] Outgoing POST: %s\n", url)
		fmt.Printf("[DEBUG][azure] Body: %s\n", string(bodyBytes))
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

	var respBody []byte
	if opts.Debug {
		respBody, _ = io.ReadAll(resp.Body)
		fmt.Printf("[DEBUG][azure] Response Status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG][azure] Response Body: %s\n", string(respBody))
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []models.IntermediateResult
	for _, res := range searchResp.Results {
		if len(res.Matches.Content) > 0 {
			for _, match := range res.Matches.Content {
				results = append(results, models.IntermediateResult{
					File:        res.Path,
					RemoteSHA:   res.ContentId,
					CharOffset:  match.CharOffset,
					Length:      match.Length,
					RawFragment: fmt.Sprintf("[Match at char offset %d, length %d]", match.CharOffset, match.Length),
					LineNumber:  1,
				})
			}
		} else {
			results = append(results, models.IntermediateResult{
				File:        res.Path,
				RemoteSHA:   res.ContentId,
				CharOffset:  -1,
				RawFragment: "[File match]",
				LineNumber:  1,
			})
		}
	}

	if opts.Debug {
		for _, r := range results {
			fmt.Printf("[DEBUG][azure] Translated Intermediate: %+v\n", r)
		}
	}

	return results, nil
}

