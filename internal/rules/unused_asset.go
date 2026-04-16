package rules

import (
	"fmt"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
)

// UnusedAssetRule flags assets declared in pubspec.yaml that are never referenced.
type UnusedAssetRule struct{}

func (r *UnusedAssetRule) ID() string { return "unused-asset" }

func (r *UnusedAssetRule) Run(g *graph.Graph, assets *asset.ScanResult, _ []*dart.Symbol, cfg *config.ScanConfig) []*report.Issue {
	if assets == nil || len(assets.DeclaredAssets) == 0 {
		return nil
	}

	var issues []*report.Issue
	for _, assetPath := range assets.DeclaredAssets {
		assetID := graph.AssetID(assetPath)
		node := g.Get(assetID)
		if node == nil {
			continue
		}

		// Check if asset has any references (string refs OR gen refs)
		if len(node.RefBy) > 0 {
			continue
		}

		// In strict mode: require actual Dart field reference, not just gen file coverage
		if !cfg.StrictAssets {
			// Non-strict: if a gen file covers this asset AND that gen file is imported → asset is live
			if genFiles, ok := assets.GenAssets[assetPath]; ok && len(genFiles) > 0 {
				genLive := false
				for _, genFile := range genFiles {
					genNode := g.Get(graph.FileID(genFile))
					if genNode != nil && len(genNode.RefBy) > 0 {
						genLive = true
						break
					}
				}
				if genLive {
					continue
				}
			}
		} else {
			// Strict mode: asset is live if any user code references its gen field accessor
			if refs, ok := assets.FieldReferencedAssets[assetPath]; ok && len(refs) > 0 {
				continue
			}
		}

		issues = append(issues, &report.Issue{
			Rule:     r.ID(),
			File:     assetPath,
			Line:     0,
			Message:  fmt.Sprintf("Asset %q is declared in pubspec.yaml but never referenced", assetPath),
			Severity: report.SeverityError,
		})
	}
	return issues
}
