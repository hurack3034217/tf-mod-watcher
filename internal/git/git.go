package git

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetChangedFiles returns a set of file paths that have changed between two commits.
// File paths are absolute paths.
func GetChangedFiles(repoPath, beforeCommit, afterCommit string) (map[string]struct{}, error) {
	// Convert repoPath to absolute path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of repo: %w", err)
	}
	repoPath = absRepoPath

	// Open the repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", repoPath, err)
	}

	// Resolve before commit hash
	beforeHash, err := resolveCommitHash(repo, beforeCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve before commit %s: %w", beforeCommit, err)
	}

	// Resolve after commit hash
	afterHash, err := resolveCommitHash(repo, afterCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve after commit %s: %w", afterCommit, err)
	}

	// Get commit objects
	beforeCommitObj, err := repo.CommitObject(beforeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get before commit object: %w", err)
	}

	afterCommitObj, err := repo.CommitObject(afterHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get after commit object: %w", err)
	}

	// Get trees
	beforeTree, err := beforeCommitObj.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get before tree: %w", err)
	}

	afterTree, err := afterCommitObj.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get after tree: %w", err)
	}

	// Get changes between trees
	changes, err := beforeTree.Diff(afterTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Create a map of changed files with absolute paths
	changedFiles := make(map[string]struct{})
	for _, change := range changes {
		// Add "from" file if it exists (not empty, i.e., not a file addition)
		if change.From.Name != "" {
			absFromPath := filepath.Join(repoPath, change.From.Name)
			absFromPath = filepath.Clean(absFromPath)
			changedFiles[absFromPath] = struct{}{}
		}
		// Add "to" file if it exists (not empty, i.e., not a file deletion)
		if change.To.Name != "" {
			absToPath := filepath.Join(repoPath, change.To.Name)
			absToPath = filepath.Clean(absToPath)
			changedFiles[absToPath] = struct{}{}
		}
	}

	return changedFiles, nil
}

// resolveCommitHash resolves a commit reference (like HEAD, HEAD^, branch name, or hash) to a commit hash
func resolveCommitHash(repo *git.Repository, ref string) (plumbing.Hash, error) {
	// Try to parse as a hash first
	if plumbing.IsHash(ref) {
		return plumbing.NewHash(ref), nil
	}

	// Try to resolve as a reference
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to resolve revision %s: %w", ref, err)
	}

	return *hash, nil
}

// GetChangedFilesBetweenCommits is a convenience function that returns changed files
// between two commits. It handles HEAD^ style references.
func GetChangedFilesBetweenCommits(repoPath string, beforeCommit, afterCommit string) (map[string]struct{}, error) {
	return GetChangedFiles(repoPath, beforeCommit, afterCommit)
}

// GetCommitForRef returns the commit object for a given reference
func GetCommitForRef(repoPath, ref string) (*object.Commit, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	hash, err := resolveCommitHash(repo, ref)
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	return commit, nil
}
