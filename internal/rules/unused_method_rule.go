package rules

import (
	"fmt"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
)

// MaybeUnusedMethodRule flags class methods with no references within the project.
// Public methods emit warnings (may be called externally via class instances).
// Private methods are handled by UnusedPrivateFunctionRule.
type MaybeUnusedMethodRule struct{}

func (r *MaybeUnusedMethodRule) ID() string { return "maybe-unused-method" }

func (r *MaybeUnusedMethodRule) Run(_ *graph.Graph, _ *asset.ScanResult, symbols []*dart.Symbol, _ *config.ScanConfig) []*report.Issue {
	var issues []*report.Issue
	for _, sym := range symbols {
		if sym.Kind != "method" {
			continue
		}
		// Private methods are handled by unused-private-function rule
		if sym.IsPrivate {
			continue
		}
		if sym.IsOverride || sym.IsEntryPoint {
			continue
		}
		if len(sym.Refs) == 0 {
			issues = append(issues, &report.Issue{
				Rule:     r.ID(),
				File:     sym.File,
				Line:     sym.Line,
				Message:  fmt.Sprintf("Method %q has no references within the project (may be called externally)", sym.Name),
				Severity: report.SeverityWarning,
			})
		}
	}
	return issues
}
