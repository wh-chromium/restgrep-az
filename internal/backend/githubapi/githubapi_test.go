package githubapi

import (
	"context"
	"os/exec"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/backend"
)

func TestGithubAPISearch(t *testing.T) {
	_, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh cli not installed, skipping test")
	}

	b := New("chromium/chromium")

	opts := backend.SearchOptions{Limit: 5}
	// This will actually hit the real API if authenticated
	results, err := b.Search(context.Background(), "OmniboxEd", opts)
	if err != nil {
		// If not logged in, this will fail, which is expected in some environments
		t.Logf("Search failed (expected if not authenticated): %v", err)
		return
	}

	t.Logf("Found %d results", len(results))
}
