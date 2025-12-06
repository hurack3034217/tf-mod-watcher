package analyzer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/hurack3034217/tf-mod-watcher/internal/terraform"
)

// Analyzer analyzes Terraform modules and determines which ones have been updated
type Analyzer struct {
	changedFiles  map[string]struct{} // Set of changed file absolute paths
	analysisCache map[string]bool     // Cache of analysis results, key: absolute module path, value: isUpdated
	logger        *slog.Logger
}

// NewAnalyzer creates a new Analyzer instance
func NewAnalyzer(changedFiles map[string]struct{}, logger *slog.Logger) (*Analyzer, error) {
	absChangedFiles := make(map[string]struct{})
	for path := range changedFiles {
		absPath, err := filepath.Abs(path)
		if err != nil {
			logger.Error("Failed to get absolute path for changed file, using original", "file", path, "error", err)
			return nil, err
		}
		absChangedFiles[absPath] = struct{}{}
	}
	return &Analyzer{
		changedFiles:  absChangedFiles,
		analysisCache: make(map[string]bool),
		logger:        logger,
	}, nil
}

func (a *Analyzer) getAnalysisCache(moduleDir string) (bool, bool, error) {
	if !filepath.IsAbs(moduleDir) {
		absModuleDir, err := filepath.Abs(moduleDir)
		if err != nil {
			return false, false, err
		}
		moduleDir = absModuleDir
	}
	moduleDir = filepath.Clean(moduleDir)
	updated, found := a.analysisCache[moduleDir]
	return updated, found, nil
}

func (a *Analyzer) setAnalysisCache(moduleDir string, updated bool) error {
	if !filepath.IsAbs(moduleDir) {
		absModuleDir, err := filepath.Abs(moduleDir)
		if err != nil {
			return err
		}
		moduleDir = absModuleDir
	}
	moduleDir = filepath.Clean(moduleDir)
	a.analysisCache[moduleDir] = updated
	return nil
}

// IsModuleUpdated checks if a module has been updated (directly or indirectly)
// It uses memoization to cache results and avoid redundant analysis
func (a *Analyzer) IsModuleUpdated(moduleDir string) (bool, error) {
	// Clean the path for consistent cache keys
	moduleDir = filepath.Clean(moduleDir)

	// Check cache first
	updated, found, err := a.getAnalysisCache(moduleDir)
	if err != nil {
		return false, err
	}
	if found {
		a.logger.Debug("Cache hit", "module", moduleDir, "updated", updated)
		return updated, nil
	}

	a.logger.Debug("Analyzing module", "module", moduleDir)

	// Check if the module directory exists
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		a.logger.Warn("Module directory does not exist", "module", moduleDir)
		err = a.setAnalysisCache(moduleDir, false)
		if err != nil {
			return false, err
		}
		return false, nil
	}

	// (A) Check for direct file changes in the module
	isDirectlyUpdated, err := a.hasDirectFileChanges(moduleDir)
	if err != nil {
		return false, fmt.Errorf("failed to check direct changes in %s: %w", moduleDir, err)
	}

	if isDirectlyUpdated {
		a.logger.Debug("Module has direct file changes", "module", moduleDir)
		err = a.setAnalysisCache(moduleDir, true)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	// (B) Check for indirect changes via child modules
	a.logger.Debug("Checking child modules", "parent", moduleDir)
	childModules, err := terraform.FindChildModules(moduleDir)
	if err != nil {
		// If we can't parse the module, we assume it's not updated
		// but log the error for debugging
		a.logger.Warn("Failed to find child modules", "module", moduleDir, "error", err)
		err = a.setAnalysisCache(moduleDir, false)
		if err != nil {
			return false, err
		}
		return false, nil
	}

	a.logger.Debug("Found child modules", "parent", moduleDir, "children", childModules)

	// Recursively check each child module
	for _, childPath := range childModules {
		updated, err := a.IsModuleUpdated(childPath)
		if err != nil {
			return false, fmt.Errorf("failed to analyze child module %s: %w", childPath, err)
		}

		if updated {
			a.logger.Debug("Child module is updated", "parent", moduleDir, "child", childPath)
			err = a.setAnalysisCache(moduleDir, true)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}

	// No changes found
	a.logger.Debug("Module is not updated", "module", moduleDir)
	err = a.setAnalysisCache(moduleDir, false)
	if err != nil {
		return false, err
	}
	return false, nil
}

// hasDirectFileChanges checks if any files in the module directory have changed
func (a *Analyzer) hasDirectFileChanges(moduleDir string) (bool, error) {
	moduleDir = filepath.Clean(moduleDir)

	// Check files in the root of the module directory (non-recursive)
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return false, fmt.Errorf("failed to read directory %s: %w", moduleDir, err)
	}

	// Check files in the module directory itself
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(moduleDir, entry.Name())
		filePath = filepath.Clean(filePath)

		// Convert to absolute path for comparison
		absFilePath, err := filepath.Abs(filePath)
		if err != nil {
			return false, fmt.Errorf("failed to get absolute path for %s: %w", filePath, err)
		}

		if _, found := a.changedFiles[absFilePath]; found {
			a.logger.Debug("Found changed file in module root", "file", absFilePath, "module", moduleDir)
			return true, nil
		}
	}

	return false, nil
}

// GetAnalysisCache returns the analysis cache (useful for testing)
func (a *Analyzer) GetAnalysisCache() map[string]bool {
	return a.analysisCache
}

// ClearCache clears the analysis cache
func (a *Analyzer) ClearCache() {
	a.analysisCache = make(map[string]bool)
}

// ConvertToRelativePath converts an absolute path to a relative path from basePath
func ConvertToRelativePath(basePath, absolutePath string) (string, error) {
	if !filepath.IsAbs(basePath) {
		return "", fmt.Errorf("base path %s is not absolute", basePath)
	}
	basePath = filepath.Clean(basePath)
	if !filepath.IsAbs(absolutePath) {
		return "", fmt.Errorf("path %s is not absolute", absolutePath)
	}
	absolutePath = filepath.Clean(absolutePath)

	// If path doesn't start with basePath, try to make it relative
	if !strings.HasPrefix(absolutePath, basePath) {
		// Try to compute relative path
		relPath, err := filepath.Rel(basePath, absolutePath)
		if err != nil {
			return "", fmt.Errorf("failed to compute relative path: %w", err)
		}
		return relPath, nil
	}

	// Remove basePath prefix
	relPath := strings.TrimPrefix(absolutePath, basePath)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

	return relPath, nil
}

// AnalyzeRootModules analyzes multiple root modules and returns the list of updated ones
func AnalyzeRootModules(rootModuleDirs []string, changedFiles map[string]struct{}, basePath string, logger *slog.Logger) ([]string, error) {
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		return nil, err
	}
	updatedModules := make([]string, 0)

	for _, moduleDir := range rootModuleDirs {
		updated, err := analyzer.IsModuleUpdated(moduleDir)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze module %s: %w", moduleDir, err)
		}

		if updated {
			absoluteBasePath, err := filepath.Abs(basePath)
			if err != nil {
				logger.Error("Failed to get absolute path for base path", "path", basePath, "error", err)
				return nil, err
			}
			// Convert to relative path for output
			absoluteModuleDir, err := filepath.Abs(moduleDir)
			if err != nil {
				logger.Error("Failed to get absolute path for module directory", "path", moduleDir, "error", err)
				return nil, err
			}
			relPath, err := ConvertToRelativePath(absoluteBasePath, absoluteModuleDir)
			if err != nil {
				logger.Error("Failed to convert to relative path, using original", "path", moduleDir, "error", err)
				return nil, err
			}
			updatedModules = append(updatedModules, relPath)
		}
	}

	return updatedModules, nil
}
