package rules

import (
	"fmt"
	"path/filepath"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
)

// UnusedFileRule flags Dart files not reachable from any entry point.
// In library mode, flags files that no other file imports (orphan files).
type UnusedFileRule struct{}

func (r *UnusedFileRule) ID() string { return "unused-file" }

func (r *UnusedFileRule) Run(g *graph.Graph, _ *asset.ScanResult, _ []*dart.Symbol, cfg *config.ScanConfig) []*report.Issue {
	if cfg.LibraryMode {
		return r.runLibraryMode(g, cfg)
	}
	return r.runAppMode(g, cfg)
}

// runAppMode uses BFS reachability from entry points.
func (r *UnusedFileRule) runAppMode(g *graph.Graph, cfg *config.ScanConfig) []*report.Issue {
	var entryIDs []string
	for _, ep := range cfg.EntryPoints {
		ep = filepath.ToSlash(ep)
		entryIDs = append(entryIDs, graph.FileID(ep))
	}

	unreachable := g.UnreachableFiles(entryIDs)

	var issues []*report.Issue
	for _, node := range unreachable {
		issues = append(issues, &report.Issue{
			Rule:     r.ID(),
			File:     node.File,
			Line:     0,
			Message:  fmt.Sprintf("%s is not reachable from any entry point", node.File),
			Severity: report.SeverityError,
		})
	}
	return issues
}

// runLibraryMode flags files that no other file imports (orphan files).
func (r *UnusedFileRule) runLibraryMode(g *graph.Graph, cfg *config.ScanConfig) []*report.Issue {
	// Build entry point set for exclusion
	entrySet := make(map[string]bool, len(cfg.EntryPoints))
	for _, ep := range cfg.EntryPoints {
		entrySet[graph.FileID(filepath.ToSlash(ep))] = true
	}

	var issues []*report.Issue
	for _, node := range g.NodesByKind(graph.KindFile) {
		if node.IsGenerated || node.IsPartOf {
			continue
		}
		// Skip entry points — they're roots by definition
		if entrySet[node.ID] {
			continue
		}
		// A file with no incoming edges is orphaned (not imported by anything)
		if len(node.RefBy) == 0 {
			issues = append(issues, &report.Issue{
				Rule:     r.ID(),
				File:     node.File,
				Line:     0,
				Message:  fmt.Sprintf("%s is not imported by any file in the project (may be external API)", node.File),
				Severity: report.SeverityWarning,
			})
		}
	}
	return issues
}
