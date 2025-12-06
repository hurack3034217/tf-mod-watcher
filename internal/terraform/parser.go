package terraform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// FindChildModules finds all child modules referenced in the given module directory.
// It returns a list of paths to the child modules.
func FindChildModules(moduleDir string) ([]string, error) {
	// Find all .tf files in the module directory
	tfFiles, err := findTerraformFiles(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform files in %s: %w", moduleDir, err)
	}

	// Parse all .tf files and extract module sources
	childModules := make([]string, 0)
	for _, tfFile := range tfFiles {
		modules, err := extractModuleSources(tfFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", tfFile, err)
		}

		// Resolve module sources to paths
		for _, source := range modules {
			// Skip remote modules (git::, registry, etc.) if they don't exist locally
			if _, err := os.Stat(filepath.Join(moduleDir, source)); os.IsNotExist(err) {
				continue
			}

			// Skip absolute paths
			if filepath.IsAbs(source) {
				continue
			}

			// Resolve relative path to path
			path := resolveModulePath(moduleDir, source)
			childModules = append(childModules, path)
		}
	}

	return childModules, nil
}

// findTerraformFiles finds all .tf files in the given directory (non-recursive)
func findTerraformFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	tfFiles := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.HasSuffix(entry.Name(), ".tf") {
			tfFiles = append(tfFiles, filepath.Join(dir, entry.Name()))
		}
	}

	return tfFiles, nil
}

// extractModuleSources parses a Terraform file and extracts all module sources
func extractModuleSources(filePath string) ([]string, error) {
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL file: %s", diags.Error())
	}

	sources := make([]string, 0)

	// Extract module blocks
	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "module",
				LabelNames: []string{"name"},
			},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to extract content: %s", diags.Error())
	}

	// Extract source attribute from each module block
	for _, block := range content.Blocks {
		attrs, diags := block.Body.JustAttributes()
		if diags.HasErrors() {
			// Skip blocks that we can't parse
			continue
		}

		if sourceAttr, exists := attrs["source"]; exists {
			val, diags := sourceAttr.Expr.Value(nil)
			if diags.HasErrors() {
				// Skip if we can't evaluate the expression
				continue
			}

			if val.Type().FriendlyName() == "string" {
				sources = append(sources, val.AsString())
			}
		}
	}

	return sources, nil
}

// resolveModulePath resolves a relative module source path
func resolveModulePath(moduleDir, source string) string {
	// Join the module directory with the source path
	joined := filepath.Join(moduleDir, source)

	// Clean the path to resolve .. and . elements
	cleaned := filepath.Clean(joined)

	return cleaned
}

// GetModuleInfo returns basic information about a module
func GetModuleInfo(moduleDir string) (map[string]interface{}, error) {
	tfFiles, err := findTerraformFiles(moduleDir)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"path":      moduleDir,
		"tfFiles":   tfFiles,
		"fileCount": len(tfFiles),
	}, nil
}
