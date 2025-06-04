package cli

import (
	"fmt"
	"os"

	cmdpkg "github.com/berrythewa/clipman-daemon/internal/cli/cmd"
	"github.com/berrythewa/clipman-daemon/internal/config"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	// Flags that apply to all commands
	logLevel  string
	cfgFile   string
	detach    bool

	// The loaded configuration
	cfg *config.Config

	// Logger instance
	zapLogger *zap.Logger
	
	// Version information - set by main
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "none"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "clipman",
	Short: "Clipman is a clipboard manager",
	Long: `Clipman is a clipboard manager that monitors your clipboard
and provides history, searching, and synchronization capabilities.

Use 'clipman daemon start' to start the daemon process.
Use other commands to interact with a running daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show help when no subcommand is provided
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		// Load config first
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}

		// Initialize basic logger
		zapConfig := zap.NewProductionConfig()
		zapLogger, err = zapConfig.Build()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %v", err)
		}
		
		// Share cfg and logger with cmd package
		cmdpkg.SetConfig(cfg)
		cmdpkg.SetZapLogger(zapLogger)

		return nil
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func SetVersionInfo(version, buildTime, commit string) {
	Version = version
	BuildTime = buildTime
	Commit = commit
}

func AddCommand(cmd *cobra.Command) {
	RootCmd.AddCommand(cmd)
}

func init() {
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level (debug, info, warn, error)")
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
} 