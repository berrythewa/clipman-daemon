package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information - accessed via cli package
var (
	version   string
	buildTime string
	commit    string
)

// SetVersionInfo allows setting version info from outside 
func SetVersionInfo(v, bt, c string) {
	version = v
	buildTime = bt
	commit = c 
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print detailed version information about the Clipman daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Clipman Daemon\n")
		fmt.Printf("Version:    %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Commit:     %s\n", commit)
	},
}

func init() {
	// No need to register the command here anymore
}