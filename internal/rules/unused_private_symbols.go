package rules

import (
	"fmt"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
)

// UnusedPrivateClassRule flags private classes (_Foo) with no references.
type UnusedPrivateClassRule struct{}

func (r *UnusedPrivateClassRule) ID() string { return "unused-private-class" }

func (r *UnusedPrivateClassRule) Run(_ *graph.Graph, _ *asset.ScanResult, symbols []*dart.Symbol, _ *config.ScanConfig) []*report.Issue {
	var issues []*report.Issue
	for _, sym := range symbols {
		if sym.Kind != "class" {
			continue
		}
		if !sym.IsPrivate {
			continue
		}
		if sym.IsOverride || sym.IsEntryPoint || sym.IsFrameworkState {
			continue
		}
		if len(sym.Refs) == 0 {
			issues = append(issues, &report.Issue{
				Rule:     r.ID(),
				File:     sym.File,
				Line:     sym.Line,
				Message:  fmt.Sprintf("Private class %q is never referenced", sym.Name),
				Severity: report.SeverityError,
			})
		}
	}
	return issues
}

// UnusedPrivateFunctionRule flags private functions/methods (_foo) with no references.
type UnusedPrivateFunctionRule struct{}

func (r *UnusedPrivateFunctionRule) ID() string { return "unused-private-function" }

func (r *UnusedPrivateFunctionRule) Run(_ *graph.Graph, _ *asset.ScanResult, symbols []*dart.Symbol, _ *config.ScanConfig) []*report.Issue {
	var issues []*report.Issue
	for _, sym := range symbols {
		if sym.Kind != "function" && sym.Kind != "method" {
			continue
		}
		if !sym.IsPrivate {
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
				Message:  fmt.Sprintf("Private %s %q is never called", sym.Kind, sym.Name),
				Severity: report.SeverityError,
			})
		}
	}
	return issues
}
