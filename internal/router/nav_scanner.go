package router

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CollectNavReferences returns the set of literal tokens that appear at any
// navigation call site in the project. Tokens are compared against
// RouteDef.Identifiers to decide whether a route is live.
//
// Recognised call sites:
//
//	auto_route:   FooRoute(           (constructor anywhere)
//	go_router:    .go('X'), .push('X'), .pushReplacement('X'),
//	              .goNamed('Y'), .pushNamed('Y'), .pushReplacementNamed('Y')
//	navigator:    Navigator.pushNamed(ctx, 'X'), Navigator.pushReplacementNamed(...),
//	              .popAndPushNamed(ctx, 'X')
func CollectNavReferences(projectRoot string, autoRouteClassNames []string) map[string]bool {
	refs := make(map[string]bool)
	libDir := filepath.Join(projectRoot, "lib")

	// Pre-compile regex that matches FooRoute( for every known auto_route class.
	var autoRouteRe *regexp.Regexp
	if len(autoRouteClassNames) > 0 {
		alts := make([]string, len(autoRouteClassNames))
		for i, n := range autoRouteClassNames {
			alts[i] = regexp.QuoteMeta(n)
		}
		autoRouteRe = regexp.MustCompile(`\b(` + strings.Join(alts, "|") + `)\s*\(`)
	}

	_ = filepath.WalkDir(libDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".dart") {
			return nil
		}
		collectFromFile(path, refs, autoRouteRe)
		return nil
	})
	return refs
}

var (
	// .go('/x'), .push('/x'), .pushReplacement('/x'), .replace('/x')
	reNavPath = regexp.MustCompile(`\.(?:go|push|pushReplacement|replace)\s*\(\s*['"]([^'"]+)['"]`)
	// .goNamed('x'), .pushNamed('x'), .pushReplacementNamed('x'), .popAndPushNamed(ctx, 'x')
	reNavNamed = regexp.MustCompile(`\.(?:goNamed|pushNamed|pushReplacementNamed|popAndPushNamed)\s*\([^)]*['"]([^'"]+)['"]`)
)

func collectFromFile(absPath string, refs map[string]bool, autoRouteRe *regexp.Regexp) {
	f, err := os.Open(absPath)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		for _, m := range reNavPath.FindAllStringSubmatch(line, -1) {
			refs[m[1]] = true
		}
		for _, m := range reNavNamed.FindAllStringSubmatch(line, -1) {
			refs[m[1]] = true
		}
		if autoRouteRe != nil {
			for _, m := range autoRouteRe.FindAllStringSubmatch(line, -1) {
				refs[m[1]] = true
			}
		}
	}
}
