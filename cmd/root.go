package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fscan",
	Short: "Flutter dead code scanner",
	Long: `fscan detects unused files, classes, functions, and assets in Flutter projects.

Rules:
  unused-file             Dart files not reachable from any entry point
  unused-private-class    Private classes (_Foo) with no references
  unused-private-function Private functions/methods (_foo) with no references
  unused-asset            Assets declared in pubspec.yaml but never referenced
  maybe-unused-public-api Public symbols with no references within the project
  maybe-unused-widget     Widget subclasses not instantiated or registered
  maybe-unused-method     Class methods with no references within the project`,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
