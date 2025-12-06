package cli

import (
	"log/slog"
	"os"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "Debug level",
			level:    "debug",
			expected: "DEBUG",
		},
		{
			name:     "Info level",
			level:    "info",
			expected: "INFO",
		},
		{
			name:     "Warn level",
			level:    "warn",
			expected: "WARN",
		},
		{
			name:     "Error level",
			level:    "error",
			expected: "ERROR",
		},
		{
			name:     "Invalid level defaults to info",
			level:    "invalid",
			expected: "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLogLevel(tt.level)
			if result.String() != tt.expected {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp()

	if app == nil {
		t.Fatal("NewApp() returned nil")
	}

	if app.Name != "tf-module-analyzer" {
		t.Errorf("Expected name 'tf-module-analyzer', got '%s'", app.Name)
	}

	// Check that required flags are present
	flagNames := map[string]bool{
		"before-commit":   false,
		"after-commit":    false,
		"root-module-dir": false,
		"base-path":       false,
		"log-level":       false,
	}

	for _, flag := range app.Flags {
		var flagName string
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagName = f.Name
		case *cli.StringSliceFlag:
			flagName = f.Name
		}

		if _, exists := flagNames[flagName]; exists {
			flagNames[flagName] = true
		}
	}

	for name, found := range flagNames {
		if !found {
			t.Errorf("Expected flag '%s' to be defined", name)
		}
	}

	// Check that Action is set
	if app.Action == nil {
		t.Error("Expected Action to be set")
	}
}

func TestNewApp_DefaultValues(t *testing.T) {
	app := NewApp()

	// Check default values
	for _, flag := range app.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			if f.Name == "before-commit" && f.Value != "HEAD^" {
				t.Errorf("Expected default before-commit to be 'HEAD^', got '%s'", f.Value)
			}
			if f.Name == "after-commit" && f.Value != "HEAD" {
				t.Errorf("Expected default after-commit to be 'HEAD', got '%s'", f.Value)
			}
			if f.Name == "log-level" && f.Value != "info" {
				t.Errorf("Expected default log-level to be 'info', got '%s'", f.Value)
			}
		}
	}
}

func TestNewApp_RequiredFlags(t *testing.T) {
	app := NewApp()

	requiredFlags := []string{"root-module-dir"}

	for _, requiredFlagName := range requiredFlags {
		found := false
		for _, flag := range app.Flags {
			var flagName string
			var isRequired bool

			switch f := flag.(type) {
			case *cli.StringFlag:
				flagName = f.Name
				isRequired = f.Required
			case *cli.StringSliceFlag:
				flagName = f.Name
				isRequired = f.Required
			}

			if flagName == requiredFlagName {
				found = true
				if !isRequired {
					t.Errorf("Expected flag '%s' to be required", requiredFlagName)
				}
				break
			}
		}

		if !found {
			t.Errorf("Required flag '%s' not found", requiredFlagName)
		}
	}
}

func TestFindGitRepositoryRoot(t *testing.T) {
	// This test should run successfully if we're in a git repository
	root, err := findGitRepositoryRoot()
	if err != nil {
		// If we're not in a git repo, this is expected
		t.Skip("Not in a git repository, skipping test")
		return
	}

	if root == "" {
		t.Error("Expected non-empty root path")
	}

	// Check that .git directory exists in the root
	gitDir := root + "/.git"
	if _, err := os.Stat(gitDir); err != nil {
		t.Errorf("Expected .git directory to exist at %s", gitDir)
	}
}

func TestContainsTerraformFiles(t *testing.T) {
	// Test with mock-terraform directory
	tests := []struct {
		name     string
		dir      string
		expected bool
	}{
		{
			name:     "Directory with .tf files",
			dir:      "../../mock-terraform/environments/organization-1/common/dev",
			expected: true,
		},
		{
			name:     "Directory without .tf files",
			dir:      "../../mock-terraform/modules",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := containsTerraformFiles(tt.dir)
			if err != nil && tt.expected {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("containsTerraformFiles(%q) = %v, want %v", tt.dir, result, tt.expected)
			}
		})
	}
}

func TestFindRootModules(t *testing.T) {
	// Create a test logger
	logger := getTestLogger()

	tests := []struct {
		name        string
		searchDir   string
		minExpected int // Minimum number of modules expected
	}{
		{
			name:        "Find modules in organization-1",
			searchDir:   "../../mock-terraform/environments/organization-1",
			minExpected: 4, // At least common/dev, common/prod, service-1/dev, service-1/prod
		},
		{
			name:        "Find modules in entire environments directory",
			searchDir:   "../../mock-terraform/environments",
			minExpected: 8, // Multiple organizations and services
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modules, err := findRootModules(tt.searchDir, logger)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(modules) < tt.minExpected {
				t.Errorf("findRootModules(%q) found %d modules, want at least %d", tt.searchDir, len(modules), tt.minExpected)
			}

			// Verify all returned paths exist
			for _, module := range modules {
				if _, err := os.Stat(module); err != nil {
					t.Errorf("Module path does not exist: %s", module)
				}
			}
		})
	}
}

func getTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors during tests
	}))
}
