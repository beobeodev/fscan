package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var appVersion = "dev"

// SetVersion sets the application version (called from main with ldflags value).
func SetVersion(v string) {
	appVersion = v
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of fscan",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("fscan %s\n", appVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
