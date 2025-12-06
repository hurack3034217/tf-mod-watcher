package analyzer

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors during tests
	}))
}

func TestIsModuleUpdated_NoChanges(t *testing.T) {
	// Setup: No changed files
	changedFiles := make(map[string]struct{})
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")
	moduleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev")

	updated, err := analyzer.IsModuleUpdated(moduleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if updated {
		t.Error("Expected module to not be updated when no files changed")
	}
}

func TestIsModuleUpdated_DirectChange(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")
	moduleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev")

	// Setup: Changed file in the module directory
	changedFiles := map[string]struct{}{
		filepath.Join(moduleDir, "main.tf"): struct{}{},
	}
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	updated, err := analyzer.IsModuleUpdated(moduleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !updated {
		t.Error("Expected module to be updated when its file changed")
	}
}

func TestIsModuleUpdated_ChildModuleChange(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	// Parent module that references child modules
	parentModuleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev")

	// Child module path
	childModulePath := filepath.Join(mockTerraformDir, "modules", "common", "common-1", "main.tf")

	// Setup: Changed file in child module
	changedFiles := map[string]struct{}{
		childModulePath: struct{}{},
	}
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	updated, err := analyzer.IsModuleUpdated(parentModuleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !updated {
		t.Error("Expected parent module to be updated when child module changed")
	}
}

func TestIsModuleUpdated_GrandchildModuleChange(t *testing.T) {
	// This test would require a 3-level module hierarchy
	// For now, we'll use the existing mock structure
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	// Use a root module
	rootModuleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "service-1", "dev")

	// Change a file in a leaf module
	changedFiles := map[string]struct{}{
		filepath.Join(mockTerraformDir, "modules", "service", "service-1", "main.tf"): struct{}{},
	}
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}
	updated, err := analyzer.IsModuleUpdated(rootModuleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !updated {
		t.Error("Expected root module to be updated when descendant module changed")
	}
}

func TestIsModuleUpdated_Cache(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	moduleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev")

	changedFiles := map[string]struct{}{
		filepath.Join(moduleDir, "main.tf"): struct{}{},
	}
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	// First call
	updated1, err := analyzer.IsModuleUpdated(moduleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Second call should use cache
	updated2, err := analyzer.IsModuleUpdated(moduleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if updated1 != updated2 {
		t.Error("Expected same result from cached analysis")
	}

	// Check that the cache contains the entry
	absModuleDir, err := filepath.Abs(moduleDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for module directory: %v", err)
	}
	if _, found := analyzer.analysisCache[absModuleDir]; !found {
		t.Error("Expected module to be in cache")
	}
}

func TestIsModuleUpdated_NonExistentModule(t *testing.T) {
	changedFiles := make(map[string]struct{})
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	moduleDir := "/non/existent/path"

	updated, err := analyzer.IsModuleUpdated(moduleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if updated {
		t.Error("Expected non-existent module to not be updated")
	}
}

func TestConvertToRelativePath(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		path        string
		expected    string
		shouldError bool
	}{
		{
			name:        "Simple relative path",
			basePath:    "/root/project",
			path:        "/root/project/modules/vpc",
			expected:    "modules/vpc",
			shouldError: false,
		},
		{
			name:        "Same path",
			basePath:    "/root/project",
			path:        "/root/project",
			expected:    "",
			shouldError: false,
		},
		{
			name:        "Nested path",
			basePath:    "/root/project",
			path:        "/root/project/a/b/c",
			expected:    "a/b/c",
			shouldError: false,
		},
		{
			name:        "Path outside base",
			basePath:    "/root/project",
			path:        "/root/other",
			expected:    "../other",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertToRelativePath(tt.basePath, tt.path)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Clean both paths for comparison
			result = filepath.Clean(result)
			expected := filepath.Clean(tt.expected)

			// Handle empty path case
			if expected == "." {
				expected = ""
			}
			if result == "." {
				result = ""
			}

			if result != expected {
				t.Errorf("ConvertToRelativePath(%q, %q) = %q, want %q", tt.basePath, tt.path, result, expected)
			}
		})
	}
}

func TestAnalyzeRootModules(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")
	absBasePath, err := filepath.Abs(mockTerraformDir)
	if err != nil {
		t.Fatalf("Failed to get absolute base path: %v", err)
	}

	rootModuleDirs := []string{
		filepath.Join(absBasePath, "environments", "organization-1", "common", "dev"),
		filepath.Join(absBasePath, "environments", "organization-1", "service-1", "dev"),
	}

	// Change a file in common-1 module
	changedFiles := map[string]struct{}{
		filepath.Join(absBasePath, "modules", "common", "common-1", "main.tf"): struct{}{},
	}

	logger := getTestLogger()

	updatedModules, err := AnalyzeRootModules(rootModuleDirs, changedFiles, absBasePath, logger)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// The common/dev module references common-1, so it should be updated
	if len(updatedModules) != 1 {
		t.Errorf("Expected 1 updated module, got %d: %v", len(updatedModules), updatedModules)
	}

	// Check that the path is relative
	for _, module := range updatedModules {
		if filepath.IsAbs(module) {
			t.Errorf("Expected relative path, got absolute: %s", module)
		}
	}
}

func TestHasDirectFileChanges(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")
	moduleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev")

	absModuleDir, err := filepath.Abs(moduleDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	tests := []struct {
		name         string
		changedFiles map[string]struct{}
		expectedHas  bool
	}{
		{
			name: "Has direct changes",
			changedFiles: map[string]struct{}{
				filepath.Join(absModuleDir, "main.tf"): struct{}{},
			},
			expectedHas: true,
		},
		{
			name:         "No direct changes",
			changedFiles: map[string]struct{}{},
			expectedHas:  false,
		},
		{
			name: "Change in different module",
			changedFiles: map[string]struct{}{
				filepath.Join(absModuleDir, "..", "prod", "main.tf"): struct{}{},
			},
			expectedHas: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := getTestLogger()
			analyzer, err := NewAnalyzer(tt.changedFiles, logger)
			if err != nil {
				t.Fatalf("Failed to create analyzer: %v", err)
			}

			has, err := analyzer.hasDirectFileChanges(absModuleDir)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if has != tt.expectedHas {
				t.Errorf("Expected hasDirectFileChanges to be %v, got %v", tt.expectedHas, has)
			}
		})
	}
}

func TestClearCache(t *testing.T) {
	changedFiles := make(map[string]struct{})
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	// Add something to cache
	analyzer.analysisCache["test"] = true

	if len(analyzer.analysisCache) != 1 {
		t.Error("Expected cache to have 1 entry")
	}

	analyzer.ClearCache()

	if len(analyzer.analysisCache) != 0 {
		t.Error("Expected cache to be empty after clear")
	}
}

func TestGetAnalysisCache(t *testing.T) {
	changedFiles := make(map[string]struct{})
	logger := getTestLogger()
	analyzer, err := NewAnalyzer(changedFiles, logger)
	if err != nil {
		t.Fatalf("Failed to create analyzer: %v", err)
	}

	analyzer.analysisCache["test"] = true

	cache := analyzer.GetAnalysisCache()

	if len(cache) != 1 {
		t.Error("Expected cache to have 1 entry")
	}

	if !cache["test"] {
		t.Error("Expected 'test' to be in cache")
	}
}
