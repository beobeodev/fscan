package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
)

// DeepScan performs a Go-based regex scan of all Dart files in the project.
// It extracts symbol declarations and cross-references them across files,
// producing []*dart.Symbol compatible with the rules engine.
// This works without the Dart SDK.
func DeepScan(cfg *config.ScanConfig) ([]*dart.Symbol, error) {
	libDir := filepath.Join(cfg.ProjectRoot, "lib")
	if _, err := os.Stat(libDir); err != nil {
		return nil, nil // no lib/ directory
	}

	// Collect Dart files — separate user files from generated files.
	// User files: extract declarations + scan for references.
	// Generated files: scan for references only (they reference user code).
	var userFiles []string
	var generatedFiles []string
	err := filepath.WalkDir(libDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				name := d.Name()
				if name == ".dart_tool" || name == "build" || name == ".pub-cache" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(path, ".dart") {
			relPath, _ := filepath.Rel(cfg.ProjectRoot, path)
			relPath = filepath.ToSlash(relPath)
			if IsGeneratedFile(relPath) || IsGeneratedFileByContent(path) {
				generatedFiles = append(generatedFiles, path)
			} else {
				userFiles = append(userFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking lib/: %w", err)
	}

	// Extract symbols from user files only
	var allSymbols []RawSymbol
	var fileContents []FileContent

	for _, absPath := range userFiles {
		relPath, _ := filepath.Rel(cfg.ProjectRoot, absPath)
		relPath = filepath.ToSlash(relPath)

		symbols, err := ParseSymbols(absPath, relPath)
		if err != nil {
			continue
		}
		allSymbols = append(allSymbols, symbols...)

		content, err := ReadFileContent(absPath)
		if err != nil {
			continue
		}
		fileContents = append(fileContents, FileContent{
			RelPath: relPath,
			Content: content,
		})
	}

	// Include generated files as reference sources (not declaration sources)
	for _, absPath := range generatedFiles {
		relPath, _ := filepath.Rel(cfg.ProjectRoot, absPath)
		relPath = filepath.ToSlash(relPath)

		content, err := ReadFileContent(absPath)
		if err != nil {
			continue
		}
		fileContents = append(fileContents, FileContent{
			RelPath: relPath,
			Content: content,
		})
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Deep scan: %d user files, %d generated files, %d symbols extracted\n",
			len(userFiles), len(generatedFiles), len(allSymbols))
	}

	// Cross-reference scan (other files referencing this symbol)
	refs := ScanReferences(fileContents, allSymbols)

	// Same-file usage scan (symbol used in its own file beyond declaration)
	sameFileUsed := ScanSameFileUsage(fileContents, allSymbols)

	// Convert to []*dart.Symbol
	var result []*dart.Symbol
	for _, raw := range allSymbols {
		id := raw.Kind + ":" + raw.File + "::" + raw.Name

		refFiles := refs[id]

		// If symbol is used in its own file, add its file to refs
		if sameFileUsed[id] {
			refFiles = append(refFiles, raw.File)
		}

		sym := &dart.Symbol{
			ID:               id,
			Kind:             raw.Kind,
			Name:             raw.Name,
			File:             raw.File,
			Line:             raw.Line,
			IsPrivate:        raw.IsPrivate,
			IsOverride:       raw.IsOverride,
			IsWidget:         raw.IsWidget,
			IsFrameworkState: raw.IsFrameworkState,
			OwnerClass:       raw.OwnerClass,
			Refs:             refFiles,
		}
		result = append(result, sym)
	}

	return result, nil
}

// MergeSymbols merges Dart worker symbols (accurate) with deep scan symbols (baseline).
// Worker symbols take precedence by symbol ID.
func MergeSymbols(deepScanSymbols, workerSymbols []*dart.Symbol) []*dart.Symbol {
	if len(workerSymbols) == 0 {
		return deepScanSymbols
	}
	if len(deepScanSymbols) == 0 {
		return workerSymbols
	}

	// Worker results override deep scan by ID
	merged := make(map[string]*dart.Symbol, len(deepScanSymbols)+len(workerSymbols))
	for _, s := range deepScanSymbols {
		merged[s.ID] = s
	}
	for _, s := range workerSymbols {
		merged[s.ID] = s // override
	}

	result := make([]*dart.Symbol, 0, len(merged))
	for _, s := range merged {
		result = append(result, s)
	}
	return result
}
