package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// setupTestRepo creates a temporary git repository with some commits for testing
func setupTestRepo(t *testing.T) (string, *git.Repository, string, string) {
	t.Helper()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize git repository
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create initial commit
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create first file
	file1Path := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1Path, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	_, err = wt.Add("file1.txt")
	if err != nil {
		t.Fatalf("Failed to add file1: %v", err)
	}

	commit1, err := wt.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to create commit1: %v", err)
	}

	// Create second file
	file2Path := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file2Path, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	_, err = wt.Add("file2.txt")
	if err != nil {
		t.Fatalf("Failed to add file2: %v", err)
	}

	commit2, err := wt.Commit("Second commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to create commit2: %v", err)
	}

	return tmpDir, repo, commit1.String(), commit2.String()
}

func TestGetChangedFiles(t *testing.T) {
	tmpDir, _, commit1, commit2 := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name          string
		beforeCommit  string
		afterCommit   string
		expectedFiles []string
		shouldError   bool
	}{
		{
			name:          "Changes between two commits",
			beforeCommit:  commit1,
			afterCommit:   commit2,
			expectedFiles: []string{"file2.txt"},
			shouldError:   false,
		},
		{
			name:          "Invalid before commit",
			beforeCommit:  "invalid-hash",
			afterCommit:   commit2,
			expectedFiles: nil,
			shouldError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changedFiles, err := GetChangedFiles(tmpDir, tt.beforeCommit, tt.afterCommit)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check that expected files are in the result (now with absolute paths)
			for _, expectedFile := range tt.expectedFiles {
				expectedAbsPath := filepath.Join(tmpDir, expectedFile)
				if _, found := changedFiles[expectedAbsPath]; !found {
					t.Errorf("Expected file %s not found in changed files (looking for %s)", expectedFile, expectedAbsPath)
				}
			}

			// Check that we don't have extra files
			if len(changedFiles) != len(tt.expectedFiles) {
				t.Errorf("Expected %d changed files, got %d", len(tt.expectedFiles), len(changedFiles))
			}
		})
	}
}

func TestResolveCommitHash(t *testing.T) {
	tmpDir, repo, commit1, _ := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		ref         string
		shouldError bool
	}{
		{
			name:        "Valid commit hash",
			ref:         commit1,
			shouldError: false,
		},
		{
			name:        "HEAD reference",
			ref:         "HEAD",
			shouldError: false,
		},
		{
			name:        "HEAD^ reference",
			ref:         "HEAD^",
			shouldError: false,
		},
		{
			name:        "Invalid reference",
			ref:         "invalid-ref",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := resolveCommitHash(repo, tt.ref)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if hash.IsZero() {
				t.Error("Expected non-zero hash")
			}
		})
	}
}

func TestGetChangedFilesBetweenCommits(t *testing.T) {
	tmpDir, _, commit1, commit2 := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	changedFiles, err := GetChangedFilesBetweenCommits(tmpDir, commit1, commit2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedAbsPath := filepath.Join(tmpDir, "file2.txt")
	if _, found := changedFiles[expectedAbsPath]; !found {
		t.Errorf("Expected file2.txt to be in changed files (looking for %s)", expectedAbsPath)
	}
}

func TestGetCommitForRef(t *testing.T) {
	tmpDir, _, _, _ := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	commit, err := GetCommitForRef(tmpDir, "HEAD")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if commit == nil {
		t.Error("Expected non-nil commit")
	} else {
		if commit.Message == "" {
			t.Error("Expected commit to have a message")
		}
	}
}
