package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/beobeodev/fscan/internal/asset"
	"github.com/beobeodev/fscan/internal/config"
	"github.com/beobeodev/fscan/internal/dart"
	"github.com/beobeodev/fscan/internal/graph"
	"github.com/beobeodev/fscan/internal/report"
	"github.com/beobeodev/fscan/internal/rules"
	"github.com/beobeodev/fscan/internal/scanner"
	"github.com/beobeodev/fscan/internal/walker"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [project-path]",
	Short: "Scan a Flutter project for dead code",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringSliceP("entry", "e", nil, "Entry point files (default: lib/main.dart)")
	scanCmd.Flags().StringP("format", "f", "text", "Output format: text, json, sarif")
	scanCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
	scanCmd.Flags().StringSliceP("rules", "r", nil, "Rules to enable (default: all)")
	scanCmd.Flags().BoolP("verbose", "v", false, "Verbose output")
	scanCmd.Flags().String("dart-worker", "", "Path to dart_worker/bin/worker.dart")
	scanCmd.Flags().Bool("skip-dart", false, "Skip Dart semantic worker (file-level analysis only)")
	scanCmd.Flags().Bool("strict-assets", false, "Require field-level references for generated assets")
}

func runScan(cmd *cobra.Command, args []string) error {
	projectRoot := "."
	if len(args) > 0 {
		projectRoot = args[0]
	}

	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}
	if _, err := os.Stat(absRoot); err != nil {
		return fmt.Errorf("project path not found: %s", absRoot)
	}

	entryPoints, _ := cmd.Flags().GetStringSlice("entry")
	if len(entryPoints) == 0 {
		entryPoints = config.DefaultEntryPoints()
	}

	// Auto-detect entry points if configured ones don't exist
	detectedEntryPoints, isLibrary := config.DetectEntryPoints(absRoot, entryPoints)
	entryPoints = detectedEntryPoints

	format, _ := cmd.Flags().GetString("format")
	outputPath, _ := cmd.Flags().GetString("output")
	rulesFilter, _ := cmd.Flags().GetStringSlice("rules")
	verbose, _ := cmd.Flags().GetBool("verbose")
	dartWorkerPath, _ := cmd.Flags().GetString("dart-worker")
	skipDart, _ := cmd.Flags().GetBool("skip-dart")
	strictAssets, _ := cmd.Flags().GetBool("strict-assets")

	cfg := &config.ScanConfig{
		ProjectRoot:     absRoot,
		EntryPoints:     entryPoints,
		OutputPath:      outputPath,
		Format:          format,
		Rules:           rulesFilter,
		Verbose:         verbose,
		DartWorkerPath:  dartWorkerPath,
		SkipDartWorker:  skipDart,
		StrictAssets:    strictAssets,
		LibraryMode:     isLibrary,
		ExcludePatterns: config.DefaultExcludePatterns(),
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Scanning: %s\n", cfg.ProjectRoot)
		if isLibrary {
			fmt.Fprintf(os.Stderr, "Library mode (no main.dart found): using orphan-file detection\n")
			fmt.Fprintf(os.Stderr, "Entry points: %s\n", strings.Join(cfg.EntryPoints, ", "))
		} else {
			fmt.Fprintf(os.Stderr, "Entry points: %s\n", strings.Join(cfg.EntryPoints, ", "))
		}
	}

	// Build symbol graph
	g := graph.New()
	if err := walker.BuildGraph(cfg, g); err != nil {
		return fmt.Errorf("failed to build import graph: %w", err)
	}

	// Scan assets
	assetResult, err := asset.Scan(cfg, g)
	if err != nil {
		return fmt.Errorf("failed to scan assets: %w", err)
	}

	// Deep scan: Go-based regex symbol extraction + cross-file reference check.
	// Always runs — no Dart SDK needed.
	deepSymbols, deepErr := scanner.DeepScan(cfg)
	if deepErr != nil && cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Warning: deep scan error: %v\n", deepErr)
	}

	// Run Dart semantic worker (if enabled) — overrides deep scan with more accuracy
	var semanticSymbols []*dart.Symbol
	if !cfg.SkipDartWorker {
		workerPath := cfg.DartWorkerPath
		if workerPath == "" {
			workerPath = filepath.Join(absRoot, "dart_worker", "bin", "worker.dart")
		}
		if _, statErr := os.Stat(workerPath); statErr == nil {
			client, workerErr := dart.NewWorkerClient(workerPath, cfg.Verbose)
			if workerErr == nil {
				defer client.Stop()
				semanticSymbols, workerErr = client.AnalyzeProject(cfg.ProjectRoot)
				if workerErr != nil && cfg.Verbose {
					fmt.Fprintf(os.Stderr, "Warning: Dart worker error: %v\n", workerErr)
				}
			} else if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: could not start Dart worker: %v\n", workerErr)
			}
		} else if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Warning: Dart worker not found at %s, skipping semantic analysis\n", workerPath)
		}
	}

	// Merge: Dart worker results override deep scan when available
	allSymbols := scanner.MergeSymbols(deepSymbols, semanticSymbols)

	// Apply rules
	engine := rules.NewEngine(cfg)
	issues := engine.Run(g, assetResult, allSymbols)

	// Output report
	reporter := report.New(cfg.Format)
	out := os.Stdout
	if cfg.OutputPath != "" {
		f, err := os.Create(cfg.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	if err := reporter.Report(issues, cfg.ProjectRoot, out); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	// Exit 1 if any issues found (for CI pipelines)
	if len(issues) > 0 {
		os.Exit(1)
	}
	return nil
}
