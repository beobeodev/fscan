package rules

import (
	"fmt"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
)

// MaybeUnusedPublicAPIRule flags public symbols with no references within the project.
// Emits warnings only — these could be part of a library's public API.
type MaybeUnusedPublicAPIRule struct{}

func (r *MaybeUnusedPublicAPIRule) ID() string { return "maybe-unused-public-api" }

func (r *MaybeUnusedPublicAPIRule) Run(_ *graph.Graph, _ *asset.ScanResult, symbols []*dart.Symbol, cfg *config.ScanConfig) []*report.Issue {
	var issues []*report.Issue
	for _, sym := range symbols {
		if sym.IsPrivate {
			continue
		}
		if sym.IsOverride || sym.IsEntryPoint || sym.IsWidget || sym.IsFrameworkState {
			continue
		}
		// Only report classes and top-level functions
		if sym.Kind != "class" && sym.Kind != "function" {
			continue
		}
		if len(sym.Refs) == 0 {
			issues = append(issues, &report.Issue{
				Rule:     r.ID(),
				File:     sym.File,
				Line:     sym.Line,
				Message:  fmt.Sprintf("Public %s %q has no references within the project (may be external API)", sym.Kind, sym.Name),
				Severity: report.SeverityWarning,
			})
		}
	}
	return issues
}

// MaybeUnusedWidgetRule flags Widget subclasses not instantiated or registered in routes.
// Emits warnings only — widgets may be used via routing or design system.
type MaybeUnusedWidgetRule struct{}

func (r *MaybeUnusedWidgetRule) ID() string { return "maybe-unused-widget" }

func (r *MaybeUnusedWidgetRule) Run(_ *graph.Graph, _ *asset.ScanResult, symbols []*dart.Symbol, _ *config.ScanConfig) []*report.Issue {
	var issues []*report.Issue
	for _, sym := range symbols {
		if !sym.IsWidget {
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
				Message:  fmt.Sprintf("Widget %q has no direct instantiation or route registration (may be used via dynamic routing)", sym.Name),
				Severity: report.SeverityWarning,
			})
		}
	}
	return issues
}
