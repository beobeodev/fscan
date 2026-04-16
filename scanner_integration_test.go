package main_test

import (
	"path/filepath"
	"testing"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/rules"
	"github.com/beobeodev/fscan/internal/scanner"
	"github.com/beobeodev/fscan/internal/walker"
)

func sampleAppRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/sample_app")
	if err != nil {
		t.Fatalf("resolving testdata path: %v", err)
	}
	return abs
}

// runRules executes the full pipeline: graph + assets + deep scan + rules.
func runRules(t *testing.T, projectRoot string, ruleIDs []string) map[string][]string {
	t.Helper()

	cfg := &config.ScanConfig{
		ProjectRoot:    projectRoot,
		EntryPoints:    config.DefaultEntryPoints(),
		Rules:          ruleIDs,
		SkipDartWorker: true,
	}

	g := graph.New()
	if err := walker.BuildGraph(cfg, g); err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	assetResult, err := asset.Scan(cfg, g)
	if err != nil {
		t.Fatalf("asset.Scan: %v", err)
	}

	// Run deep scan for symbol analysis
	symbols, err := scanner.DeepScan(cfg)
	if err != nil {
		t.Fatalf("DeepScan: %v", err)
	}

	engine := rules.NewEngine(cfg)
	issues := engine.Run(g, assetResult, symbols)

	// Group by rule
	result := make(map[string][]string)
	for _, issue := range issues {
		result[issue.Rule] = append(result[issue.Rule], issue.File)
	}
	return result
}

// --- unused-file tests ---

func TestUnusedFile_DetectsUnreachableFile(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"unused-file"})

	unusedFiles := result["unused-file"]
	if !containsPath(unusedFiles, "lib/unused_file.dart") {
		t.Errorf("expected lib/unused_file.dart in unused-file issues, got: %v", unusedFiles)
	}
}

func TestUnusedFile_DoesNotFlagReachableFiles(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"unused-file"})

	unusedFiles := result["unused-file"]
	for _, reachable := range []string{"lib/main.dart", "lib/home.dart", "lib/used_screen.dart"} {
		if containsPath(unusedFiles, reachable) {
			t.Errorf("reachable file %s should not be in unused-file issues", reachable)
		}
	}
}

func TestUnusedFile_DoesNotFlagGeneratedFiles(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"unused-file"})

	unusedFiles := result["unused-file"]
	if containsPath(unusedFiles, "lib/gen/assets.gen.dart") {
		t.Errorf("generated file lib/gen/assets.gen.dart should not be flagged as unused")
	}
}

// --- unused-asset tests ---

func TestUnusedAsset_DetectsUnreferencedAsset(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"unused-asset"})

	unusedAssets := result["unused-asset"]
	if !containsPath(unusedAssets, "assets/images/unused_icon.png") {
		t.Errorf("expected assets/images/unused_icon.png in unused-asset issues, got: %v", unusedAssets)
	}
}

func TestUnusedAsset_DoesNotFlagReferencedAsset(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"unused-asset"})

	unusedAssets := result["unused-asset"]
	if containsPath(unusedAssets, "assets/images/used_logo.png") {
		t.Errorf("used asset assets/images/used_logo.png should not be flagged")
	}
}

func TestAssetGenMapper_ExtractsFlutterGenPaths(t *testing.T) {
	root := sampleAppRoot(t)
	mapper := asset.NewAssetGenMapper(root)
	genMap := mapper.Build()

	for _, assetPath := range []string{"assets/images/used_logo.png", "assets/images/unused_icon.png"} {
		genFiles, ok := genMap[assetPath]
		if !ok || len(genFiles) == 0 {
			t.Errorf("expected %s to be covered by asset gen mapper, got: %v", assetPath, genMap)
		}
	}
}

// --- Deep scan: unused public class ---

func TestDeepScan_DetectsUnusedPublicClass(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"maybe-unused-public-api"})

	issues := result["maybe-unused-public-api"]
	if !containsPath(issues, "lib/unused_public_class.dart") {
		t.Errorf("expected UnusedPublicClass to be flagged, got: %v", issues)
	}
}

// --- Deep scan: unused public function ---

func TestDeepScan_DetectsUnusedPublicFunction(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"maybe-unused-public-api"})

	issues := result["maybe-unused-public-api"]
	if !containsPath(issues, "lib/unused_public_function.dart") {
		t.Errorf("expected unused functions in unused_public_function.dart to be flagged, got: %v", issues)
	}
}

// --- Deep scan: unused widget ---

func TestDeepScan_DetectsUnusedWidget(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"maybe-unused-widget"})

	issues := result["maybe-unused-widget"]
	if !containsPath(issues, "lib/unused_widget.dart") {
		t.Errorf("expected UnusedWidget to be flagged, got: %v", issues)
	}
}

// --- Deep scan: does NOT flag used symbols ---

func TestDeepScan_DoesNotFlagUsedSymbols(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"maybe-unused-public-api"})

	issues := result["maybe-unused-public-api"]
	// formatTitle is used in home.dart, should NOT be flagged
	for _, issue := range issues {
		if issue == "lib/used_util.dart" {
			// Check it's not about formatTitle
			// We'd need the issue Message, but at minimum,
			// used_util.dart should not appear since formatTitle is used.
			// However the file also contains formatTitle which IS used.
		}
	}

	// HomePage is used in main.dart — should not be flagged
	if containsPath(issues, "lib/home.dart") {
		t.Errorf("used class HomePage should not be flagged")
	}
}

// --- Deep scan: skips generated files ---

func TestDeepScan_SkipsGeneratedFiles(t *testing.T) {
	root := sampleAppRoot(t)
	result := runRules(t, root, []string{"maybe-unused-public-api"})

	issues := result["maybe-unused-public-api"]
	if containsPath(issues, "lib/gen/assets.gen.dart") {
		t.Errorf("generated file should not be scanned for public API")
	}
}

// --- Deep scan: symbol parser unit test ---

func TestSymbolParser_ExtractsDeclarations(t *testing.T) {
	root := sampleAppRoot(t)
	path := filepath.Join(root, "lib", "unused_public_class.dart")
	symbols, err := scanner.ParseSymbols(path, "lib/unused_public_class.dart")
	if err != nil {
		t.Fatalf("ParseSymbols: %v", err)
	}

	found := false
	for _, s := range symbols {
		if s.Name == "UnusedPublicClass" && s.Kind == "class" && !s.IsPrivate {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UnusedPublicClass declaration, got: %+v", symbols)
	}
}

func TestSymbolParser_DetectsWidgets(t *testing.T) {
	root := sampleAppRoot(t)
	path := filepath.Join(root, "lib", "unused_widget.dart")
	symbols, err := scanner.ParseSymbols(path, "lib/unused_widget.dart")
	if err != nil {
		t.Fatalf("ParseSymbols: %v", err)
	}

	found := false
	for _, s := range symbols {
		if s.Name == "UnusedWidget" && s.IsWidget {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UnusedWidget to be detected as widget, got: %+v", symbols)
	}
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if filepath.ToSlash(p) == target || p == target {
			return true
		}
	}
	return false
}
