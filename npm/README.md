# @beobeodev/fscan

A blazing-fast dead code scanner for Flutter & Dart projects.

Finds unused files, assets, classes, functions, widgets, routes — in seconds. No Dart SDK required.

## Install

```bash
npm install -g @beobeodev/fscan
```

On first install, the correct binary for your platform is automatically downloaded from [GitHub Releases](https://github.com/beobeodev/fscan/releases).

**Supported platforms:** macOS (arm64/x64), Linux (arm64/x64), Windows (x64)

## Usage

```bash
# Scan your Flutter project
fscan scan ./my_flutter_app

# With verbose output
fscan scan ./my_flutter_app -v

# JSON output for CI
fscan scan ./my_flutter_app -f json -o report.json

# SARIF for GitHub Code Scanning
fscan scan ./my_flutter_app -f sarif -o results.sarif

# Only specific rules
fscan scan ./my_flutter_app -r unused-file,unused-asset

# Print version
fscan version
```

## What It Detects

| Rule | Severity | Description |
|------|----------|-------------|
| `unused-file` | Error | Dart files not reachable from any entry point |
| `unused-asset` | Error | Assets in pubspec.yaml never referenced in code |
| `unused-private-class` | Error | Private classes with zero references |
| `unused-private-function` | Error | Private functions with zero references |
| `maybe-unused-public-api` | Warning | Public classes/functions with no project references |
| `maybe-unused-widget` | Warning | Widgets never instantiated |
| `maybe-unused-method` | Warning | Public methods with no callers |
| `unused-route` | Warning | Routes defined but never navigated to |

Supports **auto_route**, **go_router**, and **Navigator** named routes.

## Sample Output

```
fscan results for: ./my_flutter_app
────────────────────────────────────────────────────────────

UNUSED FILE (2)
  lib/legacy_screen.dart
  lib/old_util.dart

UNUSED ASSET (1)
  assets/images/old_logo.png

MAYBE UNUSED WIDGET (1)
  lib/widgets/debug_overlay.dart:12  Widget "DebugOverlay" has no direct instantiation

────────────────────────────────────────────────────────────
Total: 4 issues (3 errors, 1 warning)
```

Exit code `1` when issues are found — use as a CI quality gate.

## CI/CD (GitHub Actions)

```yaml
- name: Install fscan
  run: npm install -g @beobeodev/fscan

- name: Scan for dead code
  run: fscan scan . -f sarif -o results.sarif

- name: Upload SARIF
  if: always()
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
```

## Other Install Methods

```bash
# Homebrew (macOS/Linux)
brew install beobeodev/tap/fscan

# Go
go install github.com/beobeodev/fscan@latest
```

## Links

- [GitHub](https://github.com/beobeodev/fscan)
- [Issues](https://github.com/beobeodev/fscan/issues)
- [Releases](https://github.com/beobeodev/fscan/releases)

## License

MIT
