package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ScanConfig holds all configuration for a scan run.
type ScanConfig struct {
	// ProjectRoot is the absolute path to the Flutter project root.
	ProjectRoot string

	// EntryPoints are the Dart files that serve as reachability roots.
	// Defaults to ["lib/main.dart"].
	EntryPoints []string

	// OutputPath is where the report is written. Empty means stdout.
	OutputPath string

	// Format is the output format: "text", "json", or "sarif".
	Format string

	// Rules is the list of enabled rule IDs. Empty means all rules enabled.
	Rules []string

	// Verbose enables additional diagnostic output.
	Verbose bool

	// DartWorkerPath is the path to the Dart worker script.
	// Defaults to "<ProjectRoot>/dart_worker/bin/worker.dart" relative to the binary.
	DartWorkerPath string

	// SkipDartWorker disables the Dart semantic analysis worker.
	// Limits analysis to file-level and string-based detection only.
	SkipDartWorker bool

	// StrictAssets requires specific field-level references for generated assets
	// (rather than just import of the generated file).
	StrictAssets bool

	// LibraryMode treats all lib/ files as entry points (for library packages).
	// Auto-detected when no main.dart exists.
	LibraryMode bool

	// ExcludePatterns are glob patterns for files/dirs to skip.
	ExcludePatterns []string
}

// DefaultEntryPoints returns the default Flutter app entry point.
func DefaultEntryPoints() []string {
	return []string{"lib/main.dart"}
}

// DefaultExcludePatterns returns directories and file patterns to always skip.
func DefaultExcludePatterns() []string {
	return []string{
		"test/**",
		"integration_test/**",
		"build/**",
		".dart_tool/**",
		".pub-cache/**",
	}
}

// DetectEntryPoints resolves entry points for the project.
// If the configured entry points exist, returns them as-is.
// Otherwise, auto-detects: tries lib/{packageName}.dart, then all root lib/*.dart files.
// Returns (entryPoints, isLibraryPackage).
// isLibraryPackage is true when no main.dart exists, indicating unused-file
// rule should use all lib/ files as entry points.
func DetectEntryPoints(projectRoot string, configured []string) ([]string, bool) {
	// Check if any configured entry point actually exists
	for _, ep := range configured {
		absPath := filepath.Join(projectRoot, ep)
		if _, err := os.Stat(absPath); err == nil {
			return configured, false
		}
	}

	// No configured entry points exist — this is likely a library package.
	// Auto-detect: try lib/{packageName}.dart
	packageName := readPackageName(projectRoot)
	if packageName != "" {
		candidate := filepath.ToSlash(filepath.Join("lib", packageName+".dart"))
		absPath := filepath.Join(projectRoot, candidate)
		if _, err := os.Stat(absPath); err == nil {
			return []string{candidate}, true
		}
	}

	// Fall back to all root-level lib/*.dart files
	libDir := filepath.Join(projectRoot, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return configured, true
	}

	var rootFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".dart") {
			rootFiles = append(rootFiles, filepath.ToSlash(filepath.Join("lib", e.Name())))
		}
	}
	if len(rootFiles) > 0 {
		return rootFiles, true
	}

	return configured, true
}

// readPackageName reads the package name from pubspec.yaml.
func readPackageName(projectRoot string) string {
	pubspecPath := filepath.Join(projectRoot, "pubspec.yaml")
	data, err := os.ReadFile(pubspecPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			return strings.Trim(name, `"'`)
		}
	}
	return ""
}
