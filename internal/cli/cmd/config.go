package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
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
  • Reset configuration to defaults
  • Validate configuration syntax
  • Export configuration to YAML file
  • Load configuration from YAML file`,
	}

	// Add subcommands
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigResetCmd())
	cmd.AddCommand(newConfigValidateCmd())
	cmd.AddCommand(newConfigExportCmd())
	cmd.AddCommand(newConfigLoadCmd())

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get active config path
			configPath, err := config.GetActiveConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get active config path: %w", err)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			switch format {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			case "yaml":
				data, err := yaml.Marshal(cfg)
				if err != nil {
					return fmt.Errorf("failed to marshal config: %w", err)
				}
				fmt.Println(string(data))
				return nil
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "yaml", "output format (yaml or json)")
	return cmd
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration in your preferred editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get active config path
			configPath, err := config.GetActiveConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get active config path: %w", err)
			}

			// If config doesn't exist, create with defaults
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				cfg := config.DefaultConfig()
				if err := cfg.Save(configPath); err != nil {
					return fmt.Errorf("failed to create default config: %w", err)
				}
				fmt.Println("Created new configuration file with defaults")
			}

			// Get editor from environment or use default
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim" // Fallback editor
			}

			// Open config in editor
			editorCmd := exec.Command(editor, configPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("failed to open editor: %w", err)
			}

			// Validate edited config
			if err := validateConfig(configPath); err != nil {
				fmt.Printf("Warning: Configuration validation failed: %v\n", err)
				fmt.Println("The file has been saved, but may contain errors.")
				return nil
			}

			fmt.Println("Configuration updated and validated successfully")
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
			// Get active config path
			configPath, err := config.GetActiveConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get active config path: %w", err)
			}

			// Check if config exists
			if _, err := os.Stat(configPath); err == nil && !force {
				return fmt.Errorf("config file exists, use --force to overwrite")
			}

			// Create default config
			cfg := config.DefaultConfig()
			if err := cfg.Save(configPath); err != nil {
				return fmt.Errorf("failed to write default config: %w", err)
			}

			fmt.Println("Configuration reset to defaults")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force overwrite existing config")
	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration syntax",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get active config path
			configPath, err := config.GetActiveConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get active config path: %w", err)
			}

			if err := validateConfig(configPath); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			fmt.Println("Configuration is valid")
			return nil
		},
	}
}

func newConfigExportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export configuration to YAML file",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get active config path
			configPath, err := config.GetActiveConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get active config path: %w", err)
			}

			// Load current config
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Export to specified path
			if err := cfg.Export(outputPath); err != nil {
				return fmt.Errorf("failed to export config: %w", err)
			}

			fmt.Printf("Configuration exported to: %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "clipman-config.yaml", "output file path")
	return cmd
}

func newConfigLoadCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "load <config-file>",
		Short: "Load configuration from YAML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := args[0]

			// Load config from file
			cfg, err := config.Load(inputPath)
			if err != nil {
				return fmt.Errorf("failed to load config from file: %w", err)
			}

			// Get active config path
			configPath, err := config.GetActiveConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get active config path: %w", err)
			}

			// Check if config exists
			if _, err := os.Stat(configPath); err == nil && !force {
				return fmt.Errorf("config file exists, use --force to overwrite")
			}

			// Save to active config
			if err := cfg.Save(configPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Configuration loaded from: %s\n", inputPath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force overwrite existing config")
	return cmd
}

func validateConfig(configPath string) error {
	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Try to unmarshal as YAML
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}

	return nil
} 