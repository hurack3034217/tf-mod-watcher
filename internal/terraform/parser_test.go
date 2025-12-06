package terraform

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestFindChildModules(t *testing.T) {
	// Use the mock-terraform directory for testing
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	tests := []struct {
		name          string
		moduleDir     string
		expectedCount int
		expectedPaths []string
		shouldError   bool
	}{
		{
			name:          "Module with child modules",
			moduleDir:     filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev"),
			expectedCount: 2, // common-1 and common-2
			expectedPaths: []string{
				filepath.Join(mockTerraformDir, "modules", "common", "common-1"),
				filepath.Join(mockTerraformDir, "modules", "common", "common-2"),
			},
			shouldError: false,
		},
		{
			name:          "Module with child modules",
			moduleDir:     filepath.Join(mockTerraformDir, "environments", "organization-1", "service-1", "dev"),
			expectedCount: 2, // service-1 and service-2
			expectedPaths: []string{
				filepath.Join(mockTerraformDir, "modules", "service", "service-1"),
				filepath.Join(mockTerraformDir, "modules", "service", "service-2"),
			},
			shouldError: false,
		},
		{
			name:          "Leaf module (no children)",
			moduleDir:     filepath.Join(mockTerraformDir, "modules", "common", "common-1"),
			expectedCount: 0,
			expectedPaths: []string{},
			shouldError:   false,
		},
		{
			name:          "Non-existent directory",
			moduleDir:     filepath.Join(mockTerraformDir, "non-existent"),
			expectedCount: 0,
			expectedPaths: []string{},
			shouldError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			childModules, err := FindChildModules(tt.moduleDir)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(childModules) != tt.expectedCount {
				t.Errorf("Expected %d child modules, got %d: %v", tt.expectedCount, len(childModules), childModules)
			}

			// Check that expected paths are present
			for _, expectedPath := range tt.expectedPaths {
				if !slices.Contains(childModules, expectedPath) {
					t.Errorf("Expected child module path %s not found in result: %v", expectedPath, childModules)
				}
			}
		})
	}
}

func TestResolveModulePath(t *testing.T) {
	tests := []struct {
		name       string
		moduleDir  string
		source     string
		wantSuffix string // We'll check if the result ends with this suffix
	}{
		{
			name:       "Relative path single level",
			moduleDir:  "/root/modules/parent",
			source:     "./child",
			wantSuffix: "modules/parent/child",
		},
		{
			name:       "Relative path with parent directory",
			moduleDir:  "/root/modules/parent",
			source:     "../sibling",
			wantSuffix: "modules/sibling",
		},
		{
			name:       "Complex relative path",
			moduleDir:  "/root/environments/prod/app",
			source:     "../../../modules/vpc",
			wantSuffix: "modules/vpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveModulePath(tt.moduleDir, tt.source)

			// Clean both paths for comparison
			result = filepath.Clean(result)
			wantSuffix := filepath.Clean(tt.wantSuffix)

			if len(result) < len(wantSuffix) || result[len(result)-len(wantSuffix):] != wantSuffix {
				t.Errorf("resolveModulePath(%q, %q) = %q, want suffix %q", tt.moduleDir, tt.source, result, wantSuffix)
			}
		})
	}
}

func TestFindTerraformFiles(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	tests := []struct {
		name        string
		dir         string
		minFiles    int
		shouldError bool
	}{
		{
			name:        "Directory with .tf files",
			dir:         filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev"),
			minFiles:    1,
			shouldError: false,
		},
		{
			name:        "Non-existent directory",
			dir:         filepath.Join(mockTerraformDir, "non-existent"),
			minFiles:    0,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := findTerraformFiles(tt.dir)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(files) < tt.minFiles {
				t.Errorf("Expected at least %d .tf files, got %d", tt.minFiles, len(files))
			}

			// Verify all files end with .tf
			for _, file := range files {
				if filepath.Ext(file) != ".tf" {
					t.Errorf("Expected .tf file, got: %s", file)
				}
			}
		})
	}
}

func TestExtractModuleSources(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	tests := []struct {
		name          string
		filePath      string
		expectedCount int
		shouldError   bool
	}{
		{
			name:          "File with module blocks",
			filePath:      filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev", "main.tf"),
			expectedCount: 2,
			shouldError:   false,
		},
		{
			name:          "Non-existent file",
			filePath:      filepath.Join(mockTerraformDir, "non-existent.tf"),
			expectedCount: 0,
			shouldError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources, err := extractModuleSources(tt.filePath)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(sources) != tt.expectedCount {
				t.Errorf("Expected %d module sources, got %d: %v", tt.expectedCount, len(sources), sources)
			}
		})
	}
}

func TestGetModuleInfo(t *testing.T) {
	mockTerraformDir := filepath.Join("..", "..", "mock-terraform")

	moduleDir := filepath.Join(mockTerraformDir, "environments", "organization-1", "common", "dev")
	info, err := GetModuleInfo(moduleDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info["path"] != moduleDir {
		t.Errorf("Expected path %s, got %s", moduleDir, info["path"])
	}

	if info["fileCount"].(int) < 1 {
		t.Error("Expected at least 1 .tf file")
	}
}
