package scanner

import (
	"os"
	"regexp"
	"strings"
	"sync"
)

// FileContent holds the content of a Dart file for reference scanning.
type FileContent struct {
	RelPath string
	Content string
}

// ScanReferences checks which symbols are referenced in other files.
// Returns symbolID → list of files that reference the symbol.
// For private symbols, same-file references are counted (they can only be used in the same file).
// For public symbols, only cross-file references are counted.
// For methods with an owner class, cross-file references are only counted in files
// that also reference the owner class name (owner-class-aware matching).
func ScanReferences(files []FileContent, symbols []RawSymbol) map[string][]string {
	// Build regex patterns for each unique symbol name
	type symRef struct {
		id         string
		file       string
		ownerClass string // non-empty for methods inside a class
	}
	nameToSyms := make(map[string][]symRef)
	for _, s := range symbols {
		id := s.Kind + ":" + s.File + "::" + s.Name
		nameToSyms[s.Name] = append(nameToSyms[s.Name], symRef{id: id, file: s.File, ownerClass: s.OwnerClass})
	}

	// Pre-compile word-boundary regex for each unique symbol name
	type namePattern struct {
		name string
		re   *regexp.Regexp
		syms []symRef
	}
	patterns := make([]namePattern, 0, len(nameToSyms))
	for name, syms := range nameToSyms {
		// Skip very short names (1-2 chars) — too many false positives
		if len(name) <= 2 {
			continue
		}
		re, err := regexp.Compile(`\b` + regexp.QuoteMeta(name) + `\b`)
		if err != nil {
			continue
		}
		patterns = append(patterns, namePattern{name: name, re: re, syms: syms})
	}

	// Pre-compile owner class regexes for methods
	ownerClassRegexes := make(map[string]*regexp.Regexp)
	for _, s := range symbols {
		if s.OwnerClass != "" {
			if _, exists := ownerClassRegexes[s.OwnerClass]; !exists {
				re, err := regexp.Compile(`\b` + regexp.QuoteMeta(s.OwnerClass) + `\b`)
				if err == nil {
					ownerClassRegexes[s.OwnerClass] = re
				}
			}
		}
	}

	result := make(map[string][]string)
	var mu sync.Mutex

	// Scan files concurrently
	var wg sync.WaitGroup
	for _, fc := range files {
		fc := fc
		wg.Add(1)
		go func() {
			defer wg.Done()
			localRefs := make(map[string]bool)

			// Cache which owner classes appear in this file
			ownerClassPresent := make(map[string]bool)

			for _, pat := range patterns {
				if pat.re.MatchString(fc.Content) {
					for _, s := range pat.syms {
						// Skip self-references for public symbols (cross-file only)
						if s.file == fc.RelPath {
							continue
						}

						// Owner-class-aware: for methods, require the file also
						// references the owner class to avoid false positives
						// from common method names (e.g., "check", "update").
						if s.ownerClass != "" {
							if _, checked := ownerClassPresent[s.ownerClass]; !checked {
								if re, ok := ownerClassRegexes[s.ownerClass]; ok {
									ownerClassPresent[s.ownerClass] = re.MatchString(fc.Content)
								} else {
									ownerClassPresent[s.ownerClass] = false
								}
							}
							if !ownerClassPresent[s.ownerClass] {
								continue
							}
						}

						localRefs[s.id] = true
					}
				}
			}

			if len(localRefs) > 0 {
				mu.Lock()
				for id := range localRefs {
					result[id] = append(result[id], fc.RelPath)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return result
}

// ReadFileContent reads a Dart file and strips single-line comments to reduce
// false positives from commented-out code referencing symbol names.
func ReadFileContent(absPath string) (string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	content := string(data)
	// Strip single-line comments (// ...) to avoid false matches.
	// Replace with empty lines to preserve line numbers for ScanSameFileUsage.
	lines := strings.Split(content, "\n")
	var cleaned strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			cleaned.WriteByte('\n')
			continue
		}
		// Remove inline comments
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		cleaned.WriteString(line)
		cleaned.WriteByte('\n')
	}
	return cleaned.String(), nil
}

// ScanSameFileUsage checks which symbols are used in their own file beyond
// just the declaration. Uses line-level scanning to exclude the declaration
// line itself and count only actual usages.
// - Private symbols: any match (>= 1) counts as used.
// - Public widgets: requires >= 2 matches (constructor def doesn't count as usage).
func ScanSameFileUsage(files []FileContent, symbols []RawSymbol) map[string]bool {
	// Build a map of file → symbols to check in that file
	// Include: private symbols (all) + public widgets + methods (all)
	fileSymbols := make(map[string][]RawSymbol)
	for _, s := range symbols {
		if s.IsPrivate || s.IsWidget || s.Kind == "method" {
			fileSymbols[s.File] = append(fileSymbols[s.File], s)
		}
	}

	result := make(map[string]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, fc := range files {
		syms, ok := fileSymbols[fc.RelPath]
		if !ok {
			continue
		}
		fc := fc
		localSyms := syms
		wg.Add(1)
		go func() {
			defer wg.Done()
			lines := strings.Split(fc.Content, "\n")
			localUsed := make(map[string]bool)

			for _, s := range localSyms {
				if len(s.Name) <= 2 {
					continue
				}
				re, err := regexp.Compile(`\b` + regexp.QuoteMeta(s.Name) + `\b`)
				if err != nil {
					continue
				}
				id := s.Kind + ":" + s.File + "::" + s.Name

				// Count matches on non-declaration lines
				// Private symbols: 1 match = used
				// Public widgets: 2 matches = used (skip constructor def)
				threshold := 1
				if !s.IsPrivate {
					threshold = 2
				}

				matchCount := 0
				for lineIdx, line := range lines {
					lineNum := lineIdx + 1
					if lineNum == s.Line {
						continue // skip declaration line
					}
					if re.MatchString(line) {
						matchCount++
						if matchCount >= threshold {
							localUsed[id] = true
							break
						}
					}
				}
			}

			if len(localUsed) > 0 {
				mu.Lock()
				for id := range localUsed {
					result[id] = true
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return result
}
