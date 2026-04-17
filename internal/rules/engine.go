package rules

import (
	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
)

// Rule is the interface for a single scan rule.
type Rule interface {
	// ID returns the rule identifier (e.g., "unused-file").
	ID() string

	// Run applies the rule and returns any issues found.
	Run(g *graph.Graph, assets *asset.ScanResult, symbols []*dart.Symbol, cfg *config.ScanConfig) []*report.Issue
}

// Engine runs all enabled rules and collects issues.
type Engine struct {
	cfg   *config.ScanConfig
	rules []Rule
}

// NewEngine creates an engine with all rules registered.
func NewEngine(cfg *config.ScanConfig) *Engine {
	all := []Rule{
		&UnusedFileRule{},
		&UnusedAssetRule{},
		&UnusedPrivateClassRule{},
		&UnusedPrivateFunctionRule{},
		&MaybeUnusedPublicAPIRule{},
		&MaybeUnusedWidgetRule{},
		&MaybeUnusedMethodRule{},
		&UnusedRouteRule{},
	}

	// Filter to enabled rules if specified
	if len(cfg.Rules) == 0 {
		return &Engine{cfg: cfg, rules: all}
	}

	enabled := make(map[string]bool, len(cfg.Rules))
	for _, r := range cfg.Rules {
		enabled[r] = true
	}

	var filtered []Rule
	for _, r := range all {
		if enabled[r.ID()] {
			filtered = append(filtered, r)
		}
	}
	return &Engine{cfg: cfg, rules: filtered}
}

// Run executes all enabled rules and returns collected issues.
func (e *Engine) Run(g *graph.Graph, assets *asset.ScanResult, symbols []*dart.Symbol) []*report.Issue {
	var all []*report.Issue
	for _, r := range e.rules {
		issues := r.Run(g, assets, symbols, e.cfg)
		all = append(all, issues...)
	}
	return all
}
