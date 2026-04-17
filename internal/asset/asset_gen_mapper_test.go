package asset

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTmp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestIsAssetMappingFile_ByNameSuffix(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]bool{
		"assets.gen.dart":    true,
		"foo.assets.dart":    true,
		"r.dart":             true,
		"R.dart":             true,
		"assets.dart":        true,
		"Assets.dart":        true,
		"random_widget.dart": false,
	}
	for name, want := range cases {
		p := writeTmp(t, dir, name, "class Foo {}")
		got := isAssetMappingFile(p, name)
		if got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestIsAssetMappingFile_ByGeneratedHeader(t *testing.T) {
	dir := t.TempDir()
	// Name doesn't match suffix rules, but header says GENERATED CODE.
	content := `// GENERATED CODE - DO NOT MODIFY BY HAND
class Res {
  static const String icon = 'assets/icon.png';
}
`
	p := writeTmp(t, dir, "resource.dart", content)
	if !isAssetMappingFile(p, "resource.dart") {
		t.Error("expected GENERATED CODE header to be recognised")
	}
}

func TestIsAssetMappingFile_ByAssetStringLiteral(t *testing.T) {
	dir := t.TempDir()
	content := `class MyAssets {
  static const logo = 'assets/images/logo.png';
}
`
	p := writeTmp(t, dir, "my_assets.dart", content)
	if !isAssetMappingFile(p, "my_assets.dart") {
		t.Error("expected asset string literal detection")
	}
}

func TestIsAssetMappingFile_RejectsOrdinaryFile(t *testing.T) {
	dir := t.TempDir()
	content := `class Service {
  void doWork() { print('hello world'); }
}
`
	p := writeTmp(t, dir, "service.dart", content)
	if isAssetMappingFile(p, "service.dart") {
		t.Error("ordinary file should not be flagged as asset mapping")
	}
}
