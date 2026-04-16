package asset

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PubspecAssets holds asset information extracted from pubspec.yaml.
type PubspecAssets struct {
	// PackageName is the Dart package name.
	PackageName string

	// AssetFiles are the expanded individual asset file paths (relative to project root).
	AssetFiles []string

	// FontFiles are font asset paths declared under flutter.fonts.
	FontFiles []string
}

type pubspecDoc struct {
	Name    string `yaml:"name"`
	Flutter struct {
		Assets []string `yaml:"assets"`
		Fonts  []struct {
			Fonts []struct {
				Asset string `yaml:"asset"`
			} `yaml:"fonts"`
		} `yaml:"fonts"`
	} `yaml:"flutter"`
}

// ParsePubspec reads pubspec.yaml from projectRoot and returns declared assets.
func ParsePubspec(projectRoot string) (*PubspecAssets, error) {
	path := filepath.Join(projectRoot, "pubspec.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pubspec.yaml: %w", err)
	}

	var doc pubspecDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing pubspec.yaml: %w", err)
	}

	result := &PubspecAssets{PackageName: doc.Name}

	seen := make(map[string]bool)
	for _, decl := range doc.Flutter.Assets {
		expanded, err := expandAssetDecl(projectRoot, decl)
		if err != nil {
			continue // best-effort
		}
		for _, f := range expanded {
			if !seen[f] {
				seen[f] = true
				result.AssetFiles = append(result.AssetFiles, f)
			}
		}
	}

	for _, fontFamily := range doc.Flutter.Fonts {
		for _, fontAsset := range fontFamily.Fonts {
			if fontAsset.Asset != "" {
				result.FontFiles = append(result.FontFiles, filepath.ToSlash(fontAsset.Asset))
			}
		}
	}

	return result, nil
}

// expandAssetDecl expands a single asset declaration from pubspec.yaml.
// If the declaration ends with '/' it's treated as a directory.
// Otherwise it's a single file.
func expandAssetDecl(projectRoot, decl string) ([]string, error) {
	decl = strings.TrimSpace(decl)

	if strings.HasSuffix(decl, "/") {
		// Directory declaration — list immediate files (not recursive)
		dir := filepath.Join(projectRoot, filepath.FromSlash(decl))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, e := range entries {
			if !e.IsDir() {
				rel := filepath.ToSlash(filepath.Join(decl, e.Name()))
				files = append(files, rel)
			}
		}
		return files, nil
	}

	// Single file declaration
	return []string{filepath.ToSlash(decl)}, nil
}

// WalkAssetDirectory enumerates all asset files under the assets/ dir for completeness.
// This catches assets that exist on disk but aren't declared in pubspec.yaml.
func WalkAssetDirectory(projectRoot string) ([]string, error) {
	assetsDir := filepath.Join(projectRoot, "assets")
	if _, err := os.Stat(assetsDir); err != nil {
		return nil, nil // no assets dir
	}

	var files []string
	err := filepath.WalkDir(assetsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, path)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}
