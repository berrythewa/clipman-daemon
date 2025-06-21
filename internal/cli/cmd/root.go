package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	// Global flags
	configFile string
	verbose    bool
	quiet      bool
	useJSON    bool

	// Shared resources
	logger *zap.Logger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "clipman",
	Short: "A modern clipboard manager with history and sync capabilities",
	Long: `Clipman is a modern clipboard manager that provides:
  • Clipboard history with content type detection
  • Secure clipboard sync between devices
  • Efficient storage and retrieval of clipboard content
  • Cross-platform support (Linux, macOS, Windows)`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.config/clipman/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "minimize output")
	rootCmd.PersistentFlags().BoolVar(&useJSON, "json", false, "output in JSON format")

	// Add commands
	rootCmd.AddCommand(
		newDaemonCmd(),
		newServiceCmd(),
		newClipCmd(),
		historyCmd(),
		newConfigCmd(),
	)
} 