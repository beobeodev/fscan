package asset

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/graph"
)

// ScanResult holds the outcome of asset scanning.
type ScanResult struct {
	// DeclaredAssets are all assets from pubspec.yaml (expanded).
	DeclaredAssets []string

	// FontAssets are font files from pubspec.yaml.
	FontAssets []string

	// ReferencedAssets maps asset path → list of .dart files that reference it.
	ReferencedAssets map[string][]string

	// GenAssets maps asset path → gen file path that covers it.
	GenAssets map[string][]string

	// FieldReferencedAssets maps asset path → dart files using gen field accessor.
	// Populated only in strict-assets mode.
	FieldReferencedAssets map[string][]string
}

// Scan orchestrates the full asset analysis for a project.
func Scan(cfg *config.ScanConfig, g *graph.Graph) (*ScanResult, error) {
	pubspec, err := ParsePubspec(cfg.ProjectRoot)
	if err != nil {
		// Non-fatal — project might not have Flutter assets
		pubspec = &PubspecAssets{}
	}

	result := &ScanResult{
		DeclaredAssets:   pubspec.AssetFiles,
		FontAssets:       pubspec.FontFiles,
		ReferencedAssets: make(map[string][]string),
		GenAssets:        make(map[string][]string),
	}

	// Add asset nodes to graph
	for _, assetPath := range pubspec.AssetFiles {
		node := &graph.Node{
			ID:   graph.AssetID(assetPath),
			Kind: graph.KindAsset,
			Name: assetPath,
			File: assetPath,
		}
		g.Add(node)
	}

	// Scan .dart files for string references to asset paths.
	// Skip generated files — those are handled by the gen mapper below.
	dartFiles := collectLibDartFiles(cfg.ProjectRoot)
	for _, dartFile := range dartFiles {
		if isGeneratedDartFile(dartFile) {
			continue
		}
		refs := findAssetStringRefs(dartFile, pubspec.AssetFiles)
		for _, assetPath := range refs {
			relDart, _ := filepath.Rel(cfg.ProjectRoot, dartFile)
			relDart = filepath.ToSlash(relDart)
			result.ReferencedAssets[assetPath] = append(result.ReferencedAssets[assetPath], relDart)

			// Connect dart file → asset in graph
			fileID := graph.FileID(relDart)
			assetID := graph.AssetID(assetPath)
			g.Connect(fileID, assetID)
		}
	}

	// Scan generated files for asset path coverage.
	// Do NOT add graph edges here — gen coverage is evaluated by the rule
	// by checking if the gen file is itself imported (gen-liveness check).
	genMapper := NewAssetGenMapper(cfg.ProjectRoot)
	result.GenAssets = genMapper.Build()

	// In strict-assets mode: scan user code for gen field accessor usage (e.g., .arrowLeftBorder)
	if cfg.StrictAssets {
		result.FieldReferencedAssets = scanFieldReferences(genMapper, dartFiles, cfg.ProjectRoot)
	}

	return result, nil
}

// findAssetStringRefs scans a .dart file for occurrences of any asset path string.
func findAssetStringRefs(dartFilePath string, assets []string) []string {
	if len(assets) == 0 {
		return nil
	}

	f, err := os.Open(dartFilePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	content := readAll(f)
	var found []string
	for _, asset := range assets {
		if strings.Contains(content, asset) {
			found = append(found, asset)
		}
	}
	return found
}

func readAll(f *os.File) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}
	return sb.String()
}

// collectLibDartFiles returns absolute paths of all .dart files under lib/.
func collectLibDartFiles(projectRoot string) []string {
	libDir := filepath.Join(projectRoot, "lib")
	var files []string
	_ = filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".dart") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// isGeneratedDartFile returns true for build_runner/codegen outputs.
// These are handled by the AssetGenMapper (with liveness check), not the string scanner.
// Detects by known filenames/suffixes and by "GENERATED CODE" header marker —
// the latter covers flutter_asset_generator with custom output names (e.g. resource.dart).
func isGeneratedDartFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".g.dart") ||
		strings.HasSuffix(base, ".gen.dart") ||
		strings.HasSuffix(base, ".freezed.dart") ||
		strings.HasSuffix(base, ".gr.dart") ||
		strings.HasSuffix(base, ".mocks.dart") ||
		base == "r.dart" || base == "R.dart" ||
		base == "assets.dart" || base == "Assets.dart" ||
		base == "resource.dart" {
		return true
	}
	return hasGeneratedHeader(path)
}

// hasGeneratedHeader checks the first ~10 lines for a "GENERATED CODE" marker.
func hasGeneratedHeader(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		if strings.Contains(scanner.Text(), "GENERATED CODE") {
			return true
		}
	}
	return false
}

// scanFieldReferences checks user .dart files for gen field accessor patterns.
// Uses category-qualified patterns (e.g., "icons.arrowLeftBorder") to avoid
// false positives from common Dart method names like .add, .close, .active.
func scanFieldReferences(genMapper *AssetGenMapper, dartFiles []string, projectRoot string) map[string][]string {
	mappings := genMapper.ExtractAllFieldMappings()
	if len(mappings) == 0 {
		return nil
	}

	// Build search patterns: prefer "category.fieldName", fall back to ".fieldName" with word boundary
	type fieldInfo struct {
		pattern   string // "icons.arrowLeftBorder" or ".arrowLeftBorder"
		assetPath string
	}
	var fields []fieldInfo
	for _, m := range mappings {
		if m.Category != "" {
			fields = append(fields, fieldInfo{
				pattern:   m.Category + "." + m.FieldName,
				assetPath: m.AssetPath,
			})
		} else {
			fields = append(fields, fieldInfo{
				pattern:   "." + m.FieldName,
				assetPath: m.AssetPath,
			})
		}
	}

	result := make(map[string][]string)
	for _, dartFile := range dartFiles {
		if isGeneratedDartFile(dartFile) {
			continue
		}

		f, err := os.Open(dartFile)
		if err != nil {
			continue
		}
		content := readAll(f)
		f.Close()

		relDart, _ := filepath.Rel(projectRoot, dartFile)
		relDart = filepath.ToSlash(relDart)

		for _, fi := range fields {
			if containsFieldAccessor(content, fi.pattern) {
				result[fi.assetPath] = append(result[fi.assetPath], relDart)
			}
		}
	}
	return result
}

// containsFieldAccessor checks if content contains the pattern followed by a non-word character.
// This prevents ".arrow" from matching ".arrowLeftBorder".
func containsFieldAccessor(content, pattern string) bool {
	idx := 0
	for {
		pos := strings.Index(content[idx:], pattern)
		if pos < 0 {
			return false
		}
		endPos := idx + pos + len(pattern)
		// Accept if at end of content or next char is not a word character
		if endPos >= len(content) || !isWordChar(content[endPos]) {
			return true
		}
		idx = endPos
	}
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
