package hugo

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// GitCommit holds the git commit hash, injected at build time
	GitCommit = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version (git commit hash)",
	Long:  `Print the version information of the Hugo Reader CLI tool.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version (git commit): %s\n", GitCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
