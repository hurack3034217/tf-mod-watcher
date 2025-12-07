package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"slices"
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
	app := NewApp(io.Discard)

	if app == nil {
		t.Fatal("NewApp() returned nil")
	}

	if app.Name != "tf-module-analyzer" {
		t.Errorf("Expected name 'tf-module-analyzer', got '%s'", app.Name)
	}

	// Check that required flags are present
	flagNames := map[string]bool{
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

	exclusiveFlagGroups := []map[string]bool{
		{
			"before-commit":            false,
			"after-commit":             false,
			"git-repository-root-path": false,
		},
		{
			"changed-file": false,
		},
	}

	for _, group := range app.MutuallyExclusiveFlags {
		for i, flag := range group.Flags {
			for _, f := range flag {
				var flagName string
				switch fl := f.(type) {
				case *cli.StringFlag:
					flagName = fl.Name
				case *cli.StringSliceFlag:
					flagName = fl.Name
				}

				if _, exists := exclusiveFlagGroups[i][flagName]; exists {
					exclusiveFlagGroups[i][flagName] = true
				}
			}
		}
	}

	for i, group := range exclusiveFlagGroups {
		for name, found := range group {
			if !found {
				t.Errorf("Expected mutually exclusive flag '%s' in group %d to be defined", name, i)
			}
		}
	}

	// Check that Action is set
	if app.Action == nil {
		t.Error("Expected Action to be set")
	}
}

func TestNewApp_DefaultValues(t *testing.T) {
	app := NewApp(io.Discard)

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
	app := NewApp(io.Discard)

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

func TestRunAnalysis(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedModules []string
		expectedError   bool
	}{
		{
			name: "Run analysis on mock-terraform environment",
			args: []string{
				"--root-module-dir", "../../mock-terraform/environments",
				"--changed-file", "../../mock-terraform/modules/common/common-1/main.tf",
			},
			expectedModules: []string{
				"../../mock-terraform/environments/organization-1/common/dev",
				"../../mock-terraform/environments/organization-1/common/prod",
				"../../mock-terraform/environments/organization-2/common/dev",
				"../../mock-terraform/environments/organization-2/common/prod",
				"../../mock-terraform/environments/usecases-1/common/dev",
			},
			expectedError: false,
		},
		{
			name: "Specified base-path",
			args: []string{
				"--root-module-dir", "../../mock-terraform/environments",
				"--base-path", "../../mock-terraform",
				"--changed-file", "../../mock-terraform/modules/common/common-1/main.tf",
			},
			expectedModules: []string{
				"environments/organization-1/common/dev",
				"environments/organization-1/common/prod",
				"environments/organization-2/common/dev",
				"environments/organization-2/common/prod",
				"environments/usecases-1/common/dev",
			},
			expectedError: false,
		},
		{
			name: "No changed files",
			args: []string{
				"--root-module-dir", "../../mock-terraform/environments",
			},
			expectedModules: []string{},
			expectedError:   false,
		},
		{
			name: "Specified conflicting flags",
			args: []string{
				"--root-module-dir", "../../mock-terraform/environments",
				"--before-commit", "HEAD^",
				"--after-commit", "HEAD",
				"--changed-file", "../../mock-terraform/modules/common/common-1/main.tf",
			},
			expectedModules: nil,
			expectedError:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := NewApp(&buf).Run(context.Background(), append([]string{os.Args[0]}, tt.args...))
			if tt.expectedError {
				if err == nil {
					t.Fatalf("Expected error: %v, got: %v", tt.expectedError, err)
				} else {
					t.Logf("Expected error occurred: %v", err)
					return
				}
			} else {
				if err != nil {
					t.Fatalf("NewApp().Run() failed: %v", err)
				}
			}

			output := buf.String()
			slices.Sort(tt.expectedModules)
			expectedOutput, err := json.Marshal(tt.expectedModules)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			if output != string(expectedOutput) {
				t.Errorf("Expected output %s, got %s", string(expectedOutput), output)
			}
		})
	}
}
