package scanner

import "testing"

// TestScanSameFileUsage_PublicMethodSelfCall verifies the fix for the
// `updateFavoriteBook` false positive: a public method that is only called
// once in the same file where it is declared must be recognised as used.
// Previously the threshold was 2 for all non-private symbols, which wrongly
// required two usages beyond the declaration.
func TestScanSameFileUsage_PublicMethodSelfCall(t *testing.T) {
	content := `class Service {
  void onEvent(Event e) {
    updateFavoriteBook(e.id, e.fav);
  }

  Future<void> updateFavoriteBook(String id, bool fav) async {
    // ...
  }
}
`
	files := []FileContent{{RelPath: "lib/svc.dart", Content: content}}
	symbols := []RawSymbol{{
		Name: "updateFavoriteBook", Kind: "method", File: "lib/svc.dart",
		Line: 6, OwnerClass: "Service",
	}}

	used := ScanSameFileUsage(files, symbols)
	id := "method:lib/svc.dart::updateFavoriteBook"
	if !used[id] {
		t.Fatalf("expected public method with single same-file call to be marked used, got %v", used)
	}
}

// TestScanSameFileUsage_PublicMethodNoSelfCall ensures a public method declared
// but never called in the same file is NOT falsely marked used.
func TestScanSameFileUsage_PublicMethodNoSelfCall(t *testing.T) {
	content := `class Service {
  Future<void> updateFavoriteBook(String id, bool fav) async {}
}
`
	files := []FileContent{{RelPath: "lib/svc.dart", Content: content}}
	symbols := []RawSymbol{{
		Name: "updateFavoriteBook", Kind: "method", File: "lib/svc.dart",
		Line: 2, OwnerClass: "Service",
	}}

	used := ScanSameFileUsage(files, symbols)
	if used["method:lib/svc.dart::updateFavoriteBook"] {
		t.Fatal("method with no same-file call should not be marked used")
	}
}

// TestScanSameFileUsage_PublicWidgetNeedsTwoMatches verifies widgets still
// require threshold=2 (constructor-def line plus a real usage) to avoid
// counting `const Foo()` definitions as usage.
func TestScanSameFileUsage_PublicWidgetNeedsTwoMatches(t *testing.T) {
	// Only constructor occurs outside declaration line (once) — not used.
	single := `class FooWidget extends StatelessWidget {
  const FooWidget({super.key});
  Widget build(BuildContext ctx) => const Text('hi');
}
`
	files := []FileContent{{RelPath: "lib/w.dart", Content: single}}
	symbols := []RawSymbol{{
		Name: "FooWidget", Kind: "class", File: "lib/w.dart",
		Line: 1, IsWidget: true,
	}}
	used := ScanSameFileUsage(files, symbols)
	if used["class:lib/w.dart::FooWidget"] {
		t.Fatal("widget with only one same-file match should not be marked used")
	}

	// Two matches below the declaration — marked used.
	dbl := `class FooWidget extends StatelessWidget {
  const FooWidget({super.key});
  static final a = FooWidget();
  static final b = FooWidget();
}
`
	files = []FileContent{{RelPath: "lib/w.dart", Content: dbl}}
	used = ScanSameFileUsage(files, symbols)
	if !used["class:lib/w.dart::FooWidget"] {
		t.Fatal("widget with two same-file matches should be marked used")
	}
}

// TestScanSameFileUsage_PrivateSymbolSingleUse ensures private symbols still
// use threshold=1.
func TestScanSameFileUsage_PrivateSymbolSingleUse(t *testing.T) {
	content := `void _helper() {}
void main() { _helper(); }
`
	files := []FileContent{{RelPath: "lib/a.dart", Content: content}}
	symbols := []RawSymbol{{
		Name: "_helper", Kind: "function", File: "lib/a.dart",
		Line: 1, IsPrivate: true,
	}}
	used := ScanSameFileUsage(files, symbols)
	if !used["function:lib/a.dart::_helper"] {
		t.Fatal("private function with one same-file call should be marked used")
	}
}
