<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go 1.21+"/>
  <img src="https://img.shields.io/badge/Flutter-Compatible-02569B?style=for-the-badge&logo=flutter&logoColor=white" alt="Flutter"/>
  <img src="https://img.shields.io/badge/SARIF-2.1.0-4B275F?style=for-the-badge" alt="SARIF 2.1.0"/>
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License"/>
  <img src="https://img.shields.io/npm/v/@beobeodev/fscan?style=for-the-badge&logo=npm&logoColor=white" alt="npm"/>
</p>

<h1 align="center">fscan</h1>

<p align="center">
  <strong>A blazing-fast dead code scanner for Flutter & Dart projects</strong>
</p>

<p align="center">
  Find unused files, classes, functions, widgets, and assets in your Flutter project.<br/>
  Zero configuration. No Dart SDK required. CI-ready.
</p>

---

## Why fscan?

Flutter projects accumulate dead code over time — orphaned screens, unused utility classes, forgotten assets eating up your bundle size. Manual cleanup is tedious and error-prone.

**fscan** statically analyzes your entire Flutter project in seconds and reports exactly what's unused, with zero false positives on private symbols and smart heuristics for public APIs.

| | fscan | Manual Review | dart analyze |
|---|---|---|---|
| Unused files | Yes | Painful | No |
| Unused assets | Yes | Very painful | No |
| Unused private symbols | Yes | Impossible at scale | Partial |
| Unused public APIs | Yes (warnings) | Impossible at scale | No |
| Unused widgets | Yes | Manual | No |
| Unused routes (auto_route/go_router/Navigator) | Yes | Manual | No |
| flutter_gen support | Yes | N/A | N/A |
| Speed (500+ files) | ~2 seconds | Hours | Minutes |
| Requires Dart SDK | No | N/A | Yes |

---

## Quick Start

```bash
# Homebrew (macOS/Linux)
brew install beobeodev/tap/fscan

# npm (any platform with Node.js)
npm install -g @beobeodev/fscan

# Go
go install github.com/beobeodev/fscan@latest

# Scan your Flutter project
fscan scan ./my_flutter_app

# That's it. No config files, no setup.
```

---

## Sample Output

```
fscan results for: ./my_flutter_app
────────────────────────────────────────────────────────────

UNUSED FILE (4)
  lib/unused_public_function.dart
  lib/unused_widget.dart
  lib/unused_file.dart
  lib/unused_public_class.dart

UNUSED PRIVATE CLASS (1)
  lib/unused_file.dart:9  Private class "_UnusedPrivateHelper" is never referenced

UNUSED ASSET (1)
  assets/images/unused_icon.png

MAYBE UNUSED PUBLIC API (4)
  lib/unused_public_class.dart:4  Public class "UnusedPublicClass" has no references
  lib/unused_public_function.dart:4  Public function "formatCurrency" has no references

MAYBE UNUSED WIDGET (1)
  lib/unused_widget.dart:6  Widget "UnusedWidget" has no direct instantiation

────────────────────────────────────────────────────────────
Total: 11 issues (6 errors, 5 warnings)
```

---

## What It Detects

### Errors (High Confidence)

| Rule | Description |
|------|-------------|
| `unused-file` | Dart files in `lib/` not reachable from any entry point |
| `unused-asset` | Assets declared in `pubspec.yaml` but never referenced in code |
| `unused-private-class` | Private classes (`_Foo`) with zero references anywhere |
| `unused-private-function` | Private functions (`_foo`) with zero references anywhere |

### Warnings (May Be Intentional)

| Rule | Description |
|------|-------------|
| `maybe-unused-public-api` | Public classes and functions with no references in the project |
| `maybe-unused-widget` | Widget subclasses never instantiated or route-registered |
| `maybe-unused-method` | Public methods with no cross-file or same-file callers (owner-class-aware) |
| `unused-route` | Routes declared in auto_route / go_router / Navigator but never navigated to |

> Warnings are for symbols that *could* be part of a public API or used via dynamic routing. Review them — don't blindly delete.

---

## Features

### Dual Scan Engine

```
                  ┌─────────────────────────────────┐
                  │         fscan CLI              │
                  └──────────┬──────────────────────-┘
                             │
              ┌──────────────┴──────────────┐
              ▼                             ▼
   ┌─────────────────────┐     ┌────────────────────────┐
   │  Go Deep Scanner    │     │  Dart Semantic Worker   │
   │  (always runs)      │     │  (optional, higher      │
   │                     │     │   accuracy)             │
   │  • Regex-based      │     │  • package:analyzer     │
   │  • No dependencies  │     │  • Full AST resolution  │
   │  • ~2s for 500 files│     │  • Requires Dart SDK    │
   └─────────────────────┘     └────────────────────────┘
              │                             │
              └──────────┬──────────────────┘
                         ▼
              ┌─────────────────────┐
              │  MergeSymbols       │
              │  (Dart overrides Go │
              │   when available)   │
              └─────────────────────┘
```

- **Go deep scanner** runs by default with zero external dependencies. It extracts symbol declarations via regex and performs concurrent cross-file reference counting.
- **Dart semantic worker** (opt-in) uses `package:analyzer` for full AST resolution, constructor tracking, and class hierarchy analysis. When available, its results override the Go scanner for higher accuracy.

### Smart Detection

- **App mode vs Library mode** — auto-detected based on `main.dart` presence
  - App mode: BFS reachability from entry points
  - Library mode: orphan detection (files not imported by anything)
- **Framework-aware** — skips lifecycle methods (`build`, `createState`, `dispose`, `initState`, ...), `@override` annotations, and `_FooState` classes paired with `StatefulWidget`
- **Generated file handling** — detects `.g.dart`, `.gen.dart`, `.freezed.dart`, `.gr.dart`, `.chopper.dart` and more. Generated files are excluded from unused-file detection but included as reference sources.
- **flutter_gen support** — understands typed asset accessors (`BookAssets.icons.arrowLeft.svg()`) for accurate asset usage detection
- **Inline suppression** — skip files with `// fscan:ignore-file` or lines with `// fscan:ignore`

### CI/CD Integration

- **Exit code 1** when issues are found — use as a CI quality gate
- **SARIF output** for GitHub Code Scanning integration
- **JSON output** for custom tooling and dashboards

---

## Usage

```bash
fscan scan [project-path] [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--entry` | `-e` | `lib/main.dart` | Entry point files for reachability analysis |
| `--format` | `-f` | `text` | Output format: `text`, `json`, `sarif` |
| `--output` | `-o` | stdout | Write report to file |
| `--rules` | `-r` | all | Comma-separated list of rules to enable |
| `--verbose` | `-v` | false | Print scan statistics to stderr |
| `--skip-dart` | | false | Skip Dart semantic worker |
| `--strict-assets` | | false | Require field-level references for generated assets |
| `--dart-worker` | | auto | Path to `dart_worker/bin/worker.dart` |

### Examples

```bash
# Scan with verbose output
fscan scan ./my_app -v

# Output JSON for CI parsing
fscan scan ./my_app -f json -o report.json

# SARIF for GitHub Code Scanning
fscan scan ./my_app -f sarif -o results.sarif

# Only check for unused files and assets
fscan scan ./my_app -r unused-file,unused-asset

# Scan a library package (auto-detects, no main.dart needed)
fscan scan ./my_library_package

# Strict asset checking (detects flutter_gen accessor usage)
fscan scan ./my_app --strict-assets

# Multiple entry points
fscan scan ./my_app -e lib/main.dart -e lib/admin.dart

# With Dart semantic analysis (requires Dart SDK)
fscan scan ./my_app --dart-worker dart_worker/bin/worker.dart
```

---

## GitHub Actions

```yaml
name: Dead Code Check

on: [pull_request]

jobs:
  fscan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install fscan
        run: go install github.com/beobeodev/fscan@latest

      - name: Scan for dead code
        run: fscan scan . -f sarif -o results.sarif

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

---

## Output Formats

### Text (default)

Human-readable, grouped by rule with issue counts and a summary line. Best for local development.

### JSON

```json
{
  "project": "./my_flutter_app",
  "scanned_at": "2026-04-16T08:00:00Z",
  "total": 11,
  "issues": [
    {
      "rule": "unused-file",
      "file": "lib/unused_file.dart",
      "line": 0,
      "message": "lib/unused_file.dart is not reachable from any entry point",
      "severity": "error"
    }
  ]
}
```

### SARIF 2.1.0

Full [SARIF](https://sarifweb.azurewebsites.net/) compliance for integration with GitHub Code Scanning, VS Code SARIF Viewer, and other static analysis tools.

---

## Strict Assets Mode

By default, fscan considers an asset "used" if its generated file (e.g., `assets.gen.dart`) is imported anywhere in the project. This is fast but permissive.

With `--strict-assets`, fscan performs field-level reference scanning:

```dart
// assets.gen.dart (generated by flutter_gen)
class $AssetsIconsGen {
  SvgGenImage get arrowLeft => const SvgGenImage('assets/icons/arrow_left.svg');
  SvgGenImage get unusedIcon => const SvgGenImage('assets/icons/unused_icon.svg');
}

// user_code.dart
BookAssets.icons.arrowLeft.svg()  // arrowLeft is used
// unusedIcon is never referenced → flagged
```

fscan parses the flutter_gen class hierarchy to build `category.fieldName` patterns (e.g., `icons.arrowLeft`) and searches user code with word-boundary checking to eliminate false positives from common names like `.add`, `.close`, `.active`.

---

## Suppression

Skip entire files or specific lines:

```dart
// fscan:ignore-file
// Place in the first 10 lines to skip the entire file

import 'package:legacy/old_api.dart'; // fscan:ignore
```

---

## Architecture

```
fscan/
├── cmd/                          # CLI commands (Cobra)
│   ├── root.go                   # Root command, rule descriptions
│   └── scan.go                   # Scan pipeline orchestration
├── internal/
│   ├── config/                   # ScanConfig, entry point detection
│   ├── graph/                    # Thread-safe directed graph (file → file, file → asset)
│   ├── walker/                   # Concurrent file walker, import/export/part parser
│   ├── scanner/                  # Symbol extraction, cross-file reference counting
│   ├── asset/                    # pubspec.yaml parsing, asset scanning, flutter_gen mapping
│   ├── dart/                     # Optional Dart semantic worker (subprocess protocol)
│   ├── rules/                    # Rule engine + 8 detection rules
│   └── report/                   # Text, JSON, SARIF reporters
├── dart_worker/                  # Dart subprocess (package:analyzer)
├── testdata/sample_app/          # Integration test fixtures
├── Makefile                      # Build, test, lint, run targets
└── main.go                       # Entry point
```

### Scan Pipeline

```
  Parse CLI flags
       │
       ▼
  Detect entry points (app vs library mode)
       │
       ▼
  Build import graph (concurrent file walk)
       │
       ▼
  Scan assets (pubspec.yaml + string refs + flutter_gen)
       │
       ▼
  Deep scan symbols (Go regex, concurrent)
       │
       ▼
  Dart semantic analysis (optional, overrides deep scan)
       │
       ▼
  Apply rules → issues
       │
       ▼
  Report (text / json / sarif)
       │
       ▼
  Exit code 0 (clean) or 1 (issues found)
```

---

## Building from Source

```bash
git clone https://github.com/beobeodev/fscan.git
cd fscan
make build          # → ./build/fscan
make test           # Run all tests
make install        # Copy to $GOPATH/bin
```

### With Dart Semantic Worker

```bash
make dart-setup     # Install Dart dependencies
make run-sample-full  # Run with both scanners
```

Requires [Dart SDK](https://dart.dev/get-dart) 3.0+.

---

## Tested At Scale

fscan has been validated against real-world Flutter projects:

| Project | Files | Issues Found | Scan Time |
|---------|-------|-------------|-----------|
| App (774 files) | 774 Dart files | 164 issues | ~3s |
| Library (409 files) | 409 Dart files | 69 issues | ~2s |
| Library + strict assets | 409 Dart files | 139 issues | ~3s |

---

## Contributing

Contributions are welcome! Please open an issue or submit a PR.

```bash
# Run tests
make test

# Run linter
make lint

# Test against sample project
make run-sample
```

---

## License

MIT

---

<p align="center">
  <sub>Built with Go. Made for Flutter developers who care about clean codebases.</sub>
</p>
