package walker

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// ImportInfo holds a parsed import or part directive from a Dart file.
type ImportInfo struct {
	// Path is the raw import path string (e.g., "../utils/helper.dart" or "package:flutter/material.dart").
	Path string

	// IsPart is true for `part '...'` directives (this file includes a part).
	IsPart bool

	// IsPartOf is true for `part of '...'` directives (this file is a part of another).
	IsPartOf bool

	// IsExport is true for `export '...'` directives.
	IsExport bool
}

var (
	// Matches: import 'path'; or import "path";
	reImport = regexp.MustCompile(`^\s*import\s+['"]([^'"]+)['"]\s*`)

	// Matches: export 'path'; or export "path";
	reExport = regexp.MustCompile(`^\s*export\s+['"]([^'"]+)['"]\s*`)

	// Matches: part 'path';
	rePart = regexp.MustCompile(`^\s*part\s+['"]([^'"]+)['"]\s*`)

	// Matches: part of 'path'; or part of package:...;
	rePartOf = regexp.MustCompile(`^\s*part\s+of\s+['"]([^'"]+)['"]`)

	// Matches inline suppression: // go-scan:ignore
	reIgnore = regexp.MustCompile(`//\s*go-scan:ignore`)

	// Matches file-level suppression: // go-scan:ignore-file
	reIgnoreFile = regexp.MustCompile(`//\s*go-scan:ignore-file`)
)

// ParseImports reads a Dart file and returns all import/part/export directives.
func ParseImports(path string) ([]ImportInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var imports []ImportInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if m := rePartOf.FindStringSubmatch(line); m != nil {
			imports = append(imports, ImportInfo{Path: m[1], IsPartOf: true})
			continue
		}
		if m := rePart.FindStringSubmatch(line); m != nil {
			imports = append(imports, ImportInfo{Path: m[1], IsPart: true})
			continue
		}
		if m := reExport.FindStringSubmatch(line); m != nil {
			imports = append(imports, ImportInfo{Path: m[1], IsExport: true})
			continue
		}
		if m := reImport.FindStringSubmatch(line); m != nil {
			imports = append(imports, ImportInfo{Path: m[1]})
			continue
		}

		// Stop scanning once we hit the first non-directive line (after initial blank/comments)
		// This prevents scanning large files unnecessarily.
		trimmed := strings.TrimSpace(line)
		if trimmed != "" &&
			!strings.HasPrefix(trimmed, "//") &&
			!strings.HasPrefix(trimmed, "/*") &&
			!strings.HasPrefix(trimmed, "*") &&
			!strings.HasPrefix(trimmed, "@") &&
			!strings.HasPrefix(trimmed, "library") &&
			!strings.HasPrefix(trimmed, "import") &&
			!strings.HasPrefix(trimmed, "export") &&
			!strings.HasPrefix(trimmed, "part") &&
			!strings.HasPrefix(trimmed, "as ") &&
			!strings.HasPrefix(trimmed, "show ") &&
			!strings.HasPrefix(trimmed, "hide ") &&
			!strings.HasPrefix(trimmed, "deferred ") &&
			!strings.HasPrefix(trimmed, "ignore_for_file") {
			break
		}
	}
	return imports, scanner.Err()
}

// HasIgnoreFile checks if a Dart file has a // go-scan:ignore-file directive.
func HasIgnoreFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() && lineCount < 10 {
		if reIgnoreFile.MatchString(scanner.Text()) {
			return true
		}
		lineCount++
	}
	return false
}

// HasInlineIgnore checks if a source line contains // go-scan:ignore.
func HasInlineIgnore(line string) bool {
	return reIgnore.MatchString(line)
}
