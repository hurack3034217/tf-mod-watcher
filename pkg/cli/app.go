package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/urfave/cli/v3"

	"github.com/hurack3034217/tf-mod-watcher/internal/analyzer"
	gitpkg "github.com/hurack3034217/tf-mod-watcher/internal/git"
)

// NewApp creates and configures the CLI application
func NewApp() *cli.Command {
	return &cli.Command{
		Name:  "tf-module-analyzer",
		Usage: "Analyzes updated Terraform root modules based on git diff",
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Required: true,
				Flags: [][]cli.Flag{
					{
						&cli.StringFlag{
							Name:  "before-commit",
							Value: "HEAD^",
							Usage: "Old commit hash or reference",
						},
						&cli.StringFlag{
							Name:  "after-commit",
							Value: "HEAD",
							Usage: "New commit hash or reference",
						},
						&cli.StringFlag{
							Name:  "git-repository-root-path",
							Usage: "Git repository root path for git operations (default: auto-detected git repository root)",
						},
					},
					{
						&cli.StringSliceFlag{
							Name:  "changed-file",
							Usage: "List of changed file paths (can be specified multiple times)",
						},
					},
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:     "root-module-dir",
				Usage:    "Paths to root module directories (can be specified multiple times)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "base-path",
				Usage: "Base path for relative path calculation in output (default: same as git-repository-root-path)",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Value: "info",
				Usage: "Log level (debug, info, warn, error)",
			},
		},
		Action: runAnalysis,
	}
}

// runAnalysis is the main action that executes the analysis
func runAnalysis(ctx context.Context, cmd *cli.Command) error {
	// Setup logger
	logLevel := parseLogLevel(cmd.String("log-level"))
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Parse arguments
	beforeCommit := cmd.String("before-commit")
	afterCommit := cmd.String("after-commit")
	rootModuleDirs := cmd.StringSlice("root-module-dir")
	gitRepoRootPath := cmd.String("git-repository-root-path")
	basePath := cmd.String("base-path")
	changedFiles := cmd.StringSlice("changed-file")

	// Validate base-path exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Errorf("base-path does not exist: %s", basePath)
	}

	var changedFilesMap map[string]struct{}

	if len(changedFiles) > 0 {
		// Use provided changed files
		logger.Info("Using provided changed files", "count", len(changedFiles))
		changedFilesMap = make(map[string]struct{})
		for _, filePath := range changedFiles {
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for changed file %s: %w", filePath, err)
			}
			changedFilesMap[absPath] = struct{}{}
		}
		if basePath == "" {
			// If base-path is still not set, use current working directory
			currentDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
			basePath = currentDir
			logger.Info("Using current working directory as base-path", "basePath", basePath)
		}
	} else {
		// Search for changed files using git
		var err error
		changedFilesMap, err = searchChangedFiles(gitRepoRootPath, beforeCommit, afterCommit, logger)
		if err != nil {
			return fmt.Errorf("failed to search for changed files: %w", err)
		}
		// If base-path is not specified, use git-repository-root-path
		if basePath == "" {
			basePath = gitRepoRootPath
			logger.Info("Using git-repository-root-path as base-path", "basePath", basePath)
		}
	}

	logger.Info("Found changed files", "count", len(changedFilesMap))
	logger.Debug("Changed files", "files", changedFilesMap)

	// changedFiles already contains absolute paths from GetChangedFiles
	// Find all root modules in the specified directories
	logger.Info("Searching for root modules in specified directories")
	foundRootModuleDirs := make([]string, 0)
	for _, dir := range rootModuleDirs {
		// Recursively find all root modules in this directory
		logger.Info("Searching for root modules", "directory", dir)
		foundModules, err := findRootModules(dir, logger)
		if err != nil {
			return fmt.Errorf("failed to find root modules in %s: %w", dir, err)
		}

		if len(foundModules) == 0 {
			logger.Warn("No root modules found in directory", "directory", dir)
		} else {
			logger.Info("Found root modules", "directory", dir, "count", len(foundModules))
			foundRootModuleDirs = append(foundRootModuleDirs, foundModules...)
		}
	}

	logger.Info("Starting analysis",
		"before", beforeCommit,
		"after", afterCommit,
		"gitRepoRootPath", gitRepoRootPath,
		"basePath", basePath,
		"rootModuleDirs", rootModuleDirs,
		"changedFiles", changedFilesMap,
	)

	// Analyze root modules
	logger.Info("Analyzing root modules")
	updatedModules, err := analyzer.AnalyzeRootModules(
		foundRootModuleDirs,
		changedFilesMap,
		basePath,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to analyze root modules: %w", err)
	}

	logger.Info("Analysis complete", "updatedModules", len(updatedModules))

	// Output results as JSON
	output, err := json.Marshal(updatedModules)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(output))

	return nil
}

func searchChangedFiles(gitRepoRootPath, beforeCommit, afterCommit string, logger *slog.Logger) (map[string]struct{}, error) {
	// If git-repository-root-path is not specified, find git repository root
	if gitRepoRootPath == "" {
		logger.Debug("git-repository-root-path not specified, searching for git repository root")
		repoRoot, err := findGitRepositoryRoot()
		if err != nil {
			return nil, fmt.Errorf("failed to find git repository root: %w (please specify --git-repository-root-path)", err)
		}
		gitRepoRootPath = repoRoot
		logger.Info("Using auto-detected git repository root", "gitRepoRootPath", gitRepoRootPath)
	}

	// Validate git-repository-root-path exists
	if _, err := os.Stat(gitRepoRootPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("git-repository-root-path does not exist: %s", gitRepoRootPath)
	}

	// Get changed files from git
	logger.Info("Getting changed files from git")
	changedFiles, err := gitpkg.GetChangedFiles(gitRepoRootPath, beforeCommit, afterCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}
	return changedFiles, nil
}

// parseLogLevel parses the log level string and returns the corresponding slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// findGitRepositoryRoot finds the root directory of the git repository
// by starting from the current directory and searching upwards
func findGitRepositoryRoot() (string, error) {
	// Start from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try to open git repository from current directory
	repo, err := git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}

	// Get the worktree to find the root directory
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	return worktree.Filesystem.Root(), nil
}

// findRootModules recursively searches for Terraform root modules in the given directory
// A directory is considered a root module if it contains .tf files
func findRootModules(searchDir string, logger *slog.Logger) ([]string, error) {
	rootModules := make([]string, 0)

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			logger.Warn("Failed to access path", "path", path, "error", err)
			return nil // Continue walking even if some paths fail
		}

		// Skip non-directories
		if !d.IsDir() {
			return nil
		}

		// Check if this directory contains .tf files
		hasTerraformFiles, err := containsTerraformFiles(path)
		if err != nil {
			logger.Warn("Failed to check for Terraform files", "path", path, "error", err)
			return nil
		}

		if hasTerraformFiles {
			logger.Debug("Found root module", "path", path)
			rootModules = append(rootModules, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", searchDir, err)
	}

	return rootModules, nil
}

// containsTerraformFiles checks if a directory contains .tf files
func containsTerraformFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".tf" {
			return true, nil
		}
	}

	return false, nil
}
