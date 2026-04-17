package router

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Parse walks the project's lib/ directory and extracts all route definitions
// it can recognise across auto_route, go_router and Navigator named-route styles.
// For auto_route, the generated .gr.dart files are consulted to recover the true
// route class names (auto_route strips "Page"/"Screen"/"View" suffixes).
func Parse(projectRoot string) ([]*RouteDef, error) {
	libDir := filepath.Join(projectRoot, "lib")

	// First pass: map widget class name → generated route class name from .gr.dart.
	widgetToRoute := collectAutoRouteGeneratedNames(libDir)

	var defs []*RouteDef
	err := filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".dart") {
			return nil
		}
		// Skip generated files — they restate routes from source.
		base := d.Name()
		if strings.HasSuffix(base, ".gr.dart") ||
			strings.HasSuffix(base, ".g.dart") ||
			strings.HasSuffix(base, ".freezed.dart") {
			return nil
		}
		rel, _ := filepath.Rel(projectRoot, path)
		rel = filepath.ToSlash(rel)
		fileDefs, perr := parseFile(path, rel, widgetToRoute)
		if perr == nil {
			defs = append(defs, fileDefs...)
		}
		return nil
	})
	return defs, err
}

// collectAutoRouteGeneratedNames scans .gr.dart files and extracts widget-to-route
// mappings. auto_route emits a doc comment `/// [_iN.WidgetName]` on the line
// immediately above each generated `FooRoute` class — we use that as ground truth.
func collectAutoRouteGeneratedNames(libDir string) map[string]string {
	out := make(map[string]string)
	reRouteClass := regexp.MustCompile(`^class\s+(\w+Route)\b`)
	reDocWidget := regexp.MustCompile(`^///\s*\[_?\w*\.?(\w+)\]`)

	_ = filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".gr.dart") {
			return nil
		}
		f, ferr := os.Open(path)
		if ferr != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

		var pendingWidget string
		for scanner.Scan() {
			line := scanner.Text()
			if m := reDocWidget.FindStringSubmatch(line); m != nil {
				pendingWidget = m[1]
				continue
			}
			if m := reRouteClass.FindStringSubmatch(line); m != nil && pendingWidget != "" {
				if _, seen := out[pendingWidget]; !seen {
					out[pendingWidget] = m[1]
				}
				pendingWidget = ""
			}
		}
		return nil
	})
	return out
}

var (
	// @RoutePage() annotation directly above a widget class declaration.
	reAutoRouteAnnotation = regexp.MustCompile(`@RoutePage\s*\(`)
	reAutoRouteClass      = regexp.MustCompile(`^\s*class\s+(\w+)\s+extends\s+\w+`)

	// GoRoute/ShellRoute/StatefulShellRoute constructor (name/path may follow on later lines)
	reGoRouteStart = regexp.MustCompile(`\b(?:GoRoute|ShellRoute|StatefulShellRoute)\s*\(`)
	reGoPath       = regexp.MustCompile(`path\s*:\s*['"]([^'"]+)['"]`)
	reGoName       = regexp.MustCompile(`name\s*:\s*['"]([^'"]+)['"]`)
	reGoInitial    = regexp.MustCompile(`initialLocation\s*:\s*['"]([^'"]+)['"]`)

	// Navigator named-route map entry: '/foo': (ctx) => FooScreen()
	reNavRouteEntry = regexp.MustCompile(`['"]([^'"]+)['"]\s*:\s*\([^)]*\)\s*=>`)

	// auto_route AutoRoute(page: FooRoute.page, initial: true)
	reAutoInitial = regexp.MustCompile(`AutoRoute\s*\([^)]*page\s*:\s*(\w+)Route\.page[^)]*initial\s*:\s*true`)
)

// parseFile extracts route defs from a single .dart file.
// widgetToRoute gives the generated auto_route class name for a widget class name
// (recovered from .gr.dart). If a widget isn't in the map, fall back to
// <ClassName>Route which matches auto_route's documented default.
func parseFile(absPath, relPath string, widgetToRoute map[string]string) ([]*RouteDef, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var defs []*RouteDef
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	lines := make([]string, 0, 512)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Collect go_router initial locations across the whole file so we can
	// back-label matching routes as initial.
	initialPaths := make(map[string]bool)
	initialAutoRoutes := make(map[string]bool)
	for _, l := range lines {
		if m := reGoInitial.FindStringSubmatch(l); m != nil {
			initialPaths[m[1]] = true
		}
		if m := reAutoInitial.FindStringSubmatch(l); m != nil {
			initialAutoRoutes[m[1]+"Route"] = true
		}
	}

	for i, line := range lines {
		// auto_route: @RoutePage() on a previous line, class on a later one.
		if reAutoRouteAnnotation.MatchString(line) {
			// Search up to 5 following lines for the class decl.
			for j := i + 1; j < len(lines) && j < i+6; j++ {
				if m := reAutoRouteClass.FindStringSubmatch(lines[j]); m != nil {
					widget := m[1]
					routeClass, ok := widgetToRoute[widget]
					if !ok {
						routeClass = widget + "Route"
					}
					defs = append(defs, &RouteDef{
						File:        relPath,
						Line:        j + 1,
						Kind:        "auto_route",
						Label:       routeClass,
						Identifiers: []string{routeClass},
						Initial:     initialAutoRoutes[routeClass],
					})
					break
				}
			}
			continue
		}

		// go_router: GoRoute( ... path: ... name: ... )
		if reGoRouteStart.MatchString(line) {
			// Gather the next few lines as a window; constructor args span lines.
			end := i + 15
			if end > len(lines) {
				end = len(lines)
			}
			window := strings.Join(lines[i:end], "\n")
			pathMatch := reGoPath.FindStringSubmatch(window)
			nameMatch := reGoName.FindStringSubmatch(window)
			var ids []string
			label := ""
			if pathMatch != nil {
				ids = append(ids, pathMatch[1])
				label = pathMatch[1]
			}
			if nameMatch != nil {
				ids = append(ids, nameMatch[1])
				if label == "" {
					label = nameMatch[1]
				}
			}
			if len(ids) == 0 {
				continue
			}
			initial := false
			for _, id := range ids {
				if initialPaths[id] || id == "/" {
					initial = true
					break
				}
			}
			defs = append(defs, &RouteDef{
				File:        relPath,
				Line:        i + 1,
				Kind:        "go_router",
				Label:       label,
				Identifiers: ids,
				Initial:     initial,
			})
			continue
		}

		// Navigator named route map entry — only count lines that look like
		// '/path': (ctx) => Screen(). Avoid plain string-keyed maps.
		if m := reNavRouteEntry.FindStringSubmatch(line); m != nil {
			path := m[1]
			if !strings.HasPrefix(path, "/") {
				continue
			}
			defs = append(defs, &RouteDef{
				File:        relPath,
				Line:        i + 1,
				Kind:        "navigator",
				Label:       path,
				Identifiers: []string{path},
				Initial:     path == "/",
			})
		}
	}
	return defs, nil
}
