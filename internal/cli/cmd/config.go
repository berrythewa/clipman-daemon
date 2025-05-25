package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/berrythewa/clipman-daemon/internal/config"
)

// newConfigCmd creates the config command
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Clipman configuration",
		Long: `Manage Clipman configuration:
  • Show current configuration
  • Edit configuration in your preferred editor
  • Reset configuration to defaults`,
	}

	// Add subcommands
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigResetCmd())

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			}

			// Output YAML by default
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration in your preferred editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get config file path
			configPath := configFile
			if configPath == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				configPath = filepath.Join(home, ".config/clipman/config.yaml")
			}

			// Ensure config directory exists
			configDir := filepath.Dir(configPath)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// If config doesn't exist, create with defaults
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				cfg := config.Default()
				data, err := yaml.Marshal(cfg)
				if err != nil {
					return fmt.Errorf("failed to marshal default config: %w", err)
				}
				if err := os.WriteFile(configPath, data, 0644); err != nil {
					return fmt.Errorf("failed to write default config: %w", err)
				}
			}

			// Get editor from environment or use default
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim" // Fallback editor
			}

			// Open config in editor
			cmd := exec.Command(editor, configPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to run editor: %w", err)
			}

			// Validate edited config
			if _, err := config.Load(configPath); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			fmt.Println("Configuration updated successfully")
			return nil
		},
	}
}

func newConfigResetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get config file path
			configPath := configFile
			if configPath == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				configPath = filepath.Join(home, ".config/clipman/config.yaml")
			}

			// Check if config exists
			if _, err := os.Stat(configPath); err == nil && !force {
				return fmt.Errorf("config file exists, use --force to overwrite")
			}

			// Ensure config directory exists
			configDir := filepath.Dir(configPath)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Write default config
			cfg := config.Default()
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal default config: %w", err)
			}

			if err := os.WriteFile(configPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write default config: %w", err)
			}

			fmt.Println("Configuration reset to defaults")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force overwrite existing config")
	return cmd
} 