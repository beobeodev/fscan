package rules

import (
	"fmt"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
	"github.com/beobeodev/fscan/internal/router"
)

// UnusedRouteRule flags routes declared in router configuration (auto_route,
// go_router, Navigator named routes) that are never navigated to from anywhere
// in the project. Routes marked as initial/home are excluded.
//
// Severity is warning because dynamic/computed navigation targets cannot be
// matched by the literal scanner — a flagged route may still be reachable.
type UnusedRouteRule struct{}

func (r *UnusedRouteRule) ID() string { return "unused-route" }

func (r *UnusedRouteRule) Run(_ *graph.Graph, _ *asset.ScanResult, _ []*dart.Symbol, cfg *config.ScanConfig) []*report.Issue {
	defs, err := router.Parse(cfg.ProjectRoot)
	if err != nil || len(defs) == 0 {
		return nil
	}

	// Collect all auto_route class names (for efficient union regex in scanner).
	var autoRouteClasses []string
	for _, d := range defs {
		if d.Kind == "auto_route" {
			for _, id := range d.Identifiers {
				autoRouteClasses = append(autoRouteClasses, id)
			}
		}
	}

	refs := router.CollectNavReferences(cfg.ProjectRoot, autoRouteClasses)

	var issues []*report.Issue
	for _, d := range defs {
		if d.Initial {
			continue
		}
		if anyReferenced(d.Identifiers, refs) {
			continue
		}
		issues = append(issues, &report.Issue{
			Rule:     r.ID(),
			File:     d.File,
			Line:     d.Line,
			Message:  fmt.Sprintf("Route %q (%s) is declared but never navigated to", d.Label, d.Kind),
			Severity: report.SeverityWarning,
		})
	}
	return issues
}

func anyReferenced(ids []string, refs map[string]bool) bool {
	for _, id := range ids {
		if refs[id] {
			return true
		}
	}
	return false
}
