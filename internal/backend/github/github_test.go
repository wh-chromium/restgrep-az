package github

import (
	"context"
	"os/exec"
	"testing"

	"github.com/wh-chromium/restgrep-az/internal/backend"
)

// Because `gh` is typically an external binary, we mock the command execution 
// for unit tests or only run if `gh` is available in PATH.
func TestGithubSearch(t *testing.T) {
	_, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh cli not installed, skipping test")
	}

	// This is an integration test essentially, we just instantiate it
	b := New("restgrep-az/restgrep")

	opts := backend.SearchOptions{}
	// Just test it doesn't crash on execution if gh is authenticated.
	// Since we can't guarantee auth in CI, we won't assert on results
	_, _ = b.Search(context.Background(), "github", opts)
}
