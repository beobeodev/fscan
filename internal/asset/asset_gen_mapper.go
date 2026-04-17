package asset

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AssetGenMapper extracts asset path → generated file mappings.
// Supports flutter_gen (doc comment format) and generic string literal scan.
type AssetGenMapper struct {
	projectRoot string
}

// NewAssetGenMapper creates a mapper for the given project.
func NewAssetGenMapper(projectRoot string) *AssetGenMapper {
	return &AssetGenMapper{projectRoot: projectRoot}
}

// AssetFieldMapping maps a gen field name to its asset path.
// E.g., category="icons", fieldName="arrowLeftBorder" → assetPath="assets/icons/arrow_left_border.svg"
type AssetFieldMapping struct {
	Category  string // e.g., "icons", "images"
	FieldName string // e.g., "arrowLeftBorder"
	AssetPath string // e.g., "assets/icons/arrow_left_border.svg"
}

var (
	// flutter_gen: /// File path: assets/images/logo.png
	reFilePathComment = regexp.MustCompile(`///\s*File path:\s*(.+)`)

	// Generic string literal containing an asset path
	reAssetStringLiteral = regexp.MustCompile(`['"]((assets|fonts)/[^'"]+)['"]`)

	// flutter_gen field accessor: get fieldName => const SvgGenImage('assets/...')
	reFieldAccessor = regexp.MustCompile(`get\s+(\w+)\s*=>\s*const\s+\w+\('([^']+)'\)`)

	// flutter_gen class declaration: class $AssetsIconsGen {
	reGenClass = regexp.MustCompile(`^class\s+\$(\w+Gen)\s*\{`)

	// flutter_gen category mapping (two forms):
	//   static const $AssetsIconsGen icons = $AssetsIconsGen();
	//   $AssetsIconsVocabularyGen get vocabulary => const $AssetsIconsVocabularyGen();
	reCategoryMapping = regexp.MustCompile(`\$(\w+Gen)\s+(?:get\s+)?(\w+)\s*(?:=|=>)`)
)

// Build returns a map of asset path → list of gen files that reference it.
func (m *AssetGenMapper) Build() map[string][]string {
	genFiles := m.findGenFiles()
	result := make(map[string][]string)

	for _, genFile := range genFiles {
		assets := m.extractAssetsFromGenFile(genFile)
		relGen, _ := filepath.Rel(m.projectRoot, genFile)
		relGen = filepath.ToSlash(relGen)

		for _, assetPath := range assets {
			result[assetPath] = append(result[assetPath], relGen)
		}
	}
	return result
}

// findGenFiles returns paths of Dart files that likely contain asset mappings.
// Recognises common generator outputs by filename (flutter_gen, flutter_asset_generator,
// spider) AND any file whose first lines contain a "GENERATED CODE" marker — covers
// custom output filenames. Also includes any .dart file containing an "assets/" or
// "fonts/" string literal, so hand-written asset constant files are picked up too.
func (m *AssetGenMapper) findGenFiles() []string {
	libDir := filepath.Join(m.projectRoot, "lib")
	var genFiles []string

	_ = filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".dart") {
			return nil
		}
		if isAssetMappingFile(path, d.Name()) {
			genFiles = append(genFiles, path)
		}
		return nil
	})
	return genFiles
}

// isAssetMappingFile returns true if the file is a known generated asset mapping file,
// has a "GENERATED CODE" marker in its header, or contains asset path string literals.
func isAssetMappingFile(absPath, base string) bool {
	if strings.HasSuffix(base, ".gen.dart") ||
		strings.HasSuffix(base, ".assets.dart") ||
		base == "r.dart" || base == "R.dart" ||
		base == "assets.dart" || base == "Assets.dart" {
		return true
	}
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for i := 0; scanner.Scan() && i < 200; i++ {
		line := scanner.Text()
		if i < 10 && strings.Contains(line, "GENERATED CODE") {
			return true
		}
		if reAssetStringLiteral.MatchString(line) {
			return true
		}
	}
	return false
}

// extractAssetsFromGenFile extracts asset paths from a generated Dart file.
// Tries flutter_gen doc comment format first, falls back to string literal scan.
func (m *AssetGenMapper) extractAssetsFromGenFile(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var assets []string
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// flutter_gen: /// File path: assets/images/logo.png
		if m := reFilePathComment.FindStringSubmatch(line); m != nil {
			p := strings.TrimSpace(m[1])
			if !seen[p] {
				seen[p] = true
				assets = append(assets, p)
			}
			continue
		}

		// Generic: any 'assets/...' or 'fonts/...' string literal
		for _, match := range reAssetStringLiteral.FindAllStringSubmatch(line, -1) {
			p := match[1]
			if !seen[p] {
				seen[p] = true
				assets = append(assets, p)
			}
		}
	}
	return assets
}

// ExtractAllFieldMappings scans all gen files and returns field→asset mappings with categories.
// Parses class structure to determine category (e.g., $AssetsIconsGen → "icons").
func (m *AssetGenMapper) ExtractAllFieldMappings() []AssetFieldMapping {
	genFiles := m.findGenFiles()
	var mappings []AssetFieldMapping

	for _, genFile := range genFiles {
		mappings = append(mappings, m.extractFieldMappings(genFile)...)
	}
	return mappings
}

// extractFieldMappings parses a single gen file for class→category and field→asset mappings.
func (m *AssetGenMapper) extractFieldMappings(path string) []AssetFieldMapping {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	// Pass 1: collect class→category mappings (e.g., "AssetsIconsGen" → "icons")
	// and class→fields (fields within each class)
	type fieldEntry struct {
		fieldName string
		assetPath string
	}

	classToCategory := make(map[string]string) // "AssetsIconsGen" → "icons"
	classToFields := make(map[string][]fieldEntry)
	currentClass := ""

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Track current class: class $AssetsIconsGen {
		if match := reGenClass.FindStringSubmatch(line); match != nil {
			currentClass = match[1]
			continue
		}

		// Category mapping: static const $AssetsIconsGen icons = ...
		if match := reCategoryMapping.FindStringSubmatch(line); match != nil {
			className := match[1]
			category := match[2]
			classToCategory[className] = category
			continue
		}

		// Field accessor within current class
		if currentClass != "" {
			if match := reFieldAccessor.FindStringSubmatch(line); match != nil {
				fieldName := match[1]
				assetPath := match[2]
				if fieldName != "values" {
					classToFields[currentClass] = append(classToFields[currentClass], fieldEntry{
						fieldName: fieldName,
						assetPath: assetPath,
					})
				}
			}
		}
	}

	// Build mappings with resolved categories
	var mappings []AssetFieldMapping
	for className, fields := range classToFields {
		category := classToCategory[className]
		for _, fe := range fields {
			mappings = append(mappings, AssetFieldMapping{
				Category:  category,
				FieldName: fe.fieldName,
				AssetPath: fe.assetPath,
			})
		}
	}
	return mappings
}
