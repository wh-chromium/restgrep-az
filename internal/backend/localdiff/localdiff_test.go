package localdiff

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/wh-chromium/restgrep-az/internal/models"
)

func TestSearch_NoGitRepo(t *testing.T) {
	// Create a temp directory that is NOT a git repository
	dir, err := os.MkdirTemp("", "localdiff_nogit")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	b := New("main")
	_, err = b.Search(context.Background(), "query", models.SearchOptions{})
	if err == nil {
		t.Error("expected error when running search outside of git repo, got nil")
	}
}

func TestSearch_MissingBranch(t *testing.T) {
	// Create a temp git repository
	dir, err := os.MkdirTemp("", "localdiff_missing_branch")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Add a dummy commit to establish HEAD
	filename := "file.txt"
	os.WriteFile(filename, []byte("content\n"), 0644)
	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Add(filename)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}

	b := New("nonexistent-branch")
	_, err = b.Search(context.Background(), "query", models.SearchOptions{})
	if err == nil {
		t.Error("expected error when target branch is missing, got nil")
	}
}

func TestSearch_CorruptedOrMissingCommit(t *testing.T) {
	dir, err := os.MkdirTemp("", "localdiff_corrupted_commit")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create a reference pointing to a non-existent hash (e.g. all zeros or a random hash)
	fakeHash := plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	ref := plumbing.NewHashReference(plumbing.HEAD, fakeHash)
	err = repo.Storer.SetReference(ref)
	if err != nil {
		t.Fatal(err)
	}

	b := New("main")
	_, err = b.Search(context.Background(), "query", models.SearchOptions{})
	if err == nil {
		t.Error("expected error when HEAD commit does not exist, got nil")
	}
}
