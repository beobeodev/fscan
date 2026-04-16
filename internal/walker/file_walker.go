package walker

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/graph"
)

// BuildGraph walks the project's lib/ directory, adds file nodes to the graph,
// parses imports/parts, and connects edges.
func BuildGraph(cfg *config.ScanConfig, g *graph.Graph) error {
	libDir := filepath.Join(cfg.ProjectRoot, "lib")
	if _, err := os.Stat(libDir); err != nil {
		return nil // no lib/ directory — not a Dart project (non-fatal)
	}

	// Collect all .dart files concurrently
	dartFiles, err := collectDartFiles(libDir)
	if err != nil {
		return err
	}

	// Add all file nodes first (so edges can reference them)
	for _, absPath := range dartFiles {
		relPath, _ := filepath.Rel(cfg.ProjectRoot, absPath)
		relPath = filepath.ToSlash(relPath)

		node := &graph.Node{
			ID:          graph.FileID(relPath),
			Kind:        graph.KindFile,
			Name:        relPath,
			File:        relPath,
			IsGenerated: isGeneratedFile(relPath) || IsGeneratedFileByContent(absPath),
		}
		g.Add(node)
	}

	// Parse imports and connect edges concurrently
	var wg sync.WaitGroup
	errs := make(chan error, len(dartFiles))
	packageName := readPackageName(cfg.ProjectRoot)

	for _, absPath := range dartFiles {
		absPath := absPath
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := processFileImports(absPath, cfg.ProjectRoot, packageName, g); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// collectDartFiles returns absolute paths of all .dart files under root.
func collectDartFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			name := d.Name()
			// Skip hidden dirs and build artifacts
			if name == ".dart_tool" || name == "build" || name == ".pub-cache" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".dart") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// processFileImports parses a Dart file and adds import/part edges to the graph.
func processFileImports(absPath, projectRoot, packageName string, g *graph.Graph) error {
	relPath, _ := filepath.Rel(projectRoot, absPath)
	relPath = filepath.ToSlash(relPath)
	srcID := graph.FileID(relPath)

	imports, err := ParseImports(absPath)
	if err != nil {
		return nil // skip parse errors
	}

	for _, imp := range imports {
		if imp.IsPartOf {
			// This file is a part — mark it accordingly
			if src := g.Get(srcID); src != nil {
				src.IsPartOf = true
				src.PartOfFile = resolveImportPath(imp.Path, relPath, projectRoot, packageName)
			}
			continue
		}

		dstRel := resolveImportPath(imp.Path, relPath, projectRoot, packageName)
		if dstRel == "" {
			continue // external package or dart: — skip
		}

		dstID := graph.FileID(dstRel)
		if imp.IsPart {
			// Mark the part file as part-of
			if dst := g.Get(dstID); dst != nil {
				dst.IsPartOf = true
				dst.PartOfFile = relPath
			}
		}
		g.Connect(srcID, dstID)
	}
	return nil
}

// resolveImportPath converts an import path to a relative file path from projectRoot.
// Returns "" for external packages and dart: imports.
func resolveImportPath(importPath, fromRelPath, projectRoot, packageName string) string {
	// dart: built-ins — skip
	if strings.HasPrefix(importPath, "dart:") {
		return ""
	}

	// package:own_package/path.dart → lib/path.dart
	if strings.HasPrefix(importPath, "package:"+packageName+"/") {
		sub := strings.TrimPrefix(importPath, "package:"+packageName+"/")
		return filepath.ToSlash(filepath.Join("lib", sub))
	}

	// Other packages — skip
	if strings.HasPrefix(importPath, "package:") {
		return ""
	}

	// Relative import
	dir := filepath.Dir(fromRelPath)
	resolved := filepath.ToSlash(filepath.Clean(filepath.Join(dir, importPath)))
	return resolved
}

// isGeneratedFile returns true for files created by build_runner.
// Checks suffix patterns first (fast), then falls back to content-based
// detection for non-standard generated files like injectable .module.dart.
func isGeneratedFile(relPath string) bool {
	base := filepath.Base(relPath)
	return strings.HasSuffix(base, ".g.dart") ||
		strings.HasSuffix(base, ".gen.dart") ||
		strings.HasSuffix(base, ".freezed.dart") ||
		strings.HasSuffix(base, ".gr.dart") ||
		strings.HasSuffix(base, ".config.dart") ||
		strings.HasSuffix(base, ".chopper.dart") ||
		strings.HasSuffix(base, ".mocks.dart") ||
		strings.HasSuffix(base, ".module.dart")
}

// IsGeneratedFileByContent checks if a file's content indicates it's generated.
// Looks for "GENERATED CODE" in the first 5 lines.
func IsGeneratedFileByContent(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 5 && scanner.Scan(); i++ {
		if strings.Contains(scanner.Text(), "GENERATED CODE") {
			return true
		}
	}
	return false
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
