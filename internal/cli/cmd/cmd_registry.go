package cmd

import (
	"github.com/spf13/cobra"
)

// GetCommands returns all commands for registration
func GetCommands() []*cobra.Command {
	return []*cobra.Command{
		newDaemonCmd(),
		newServiceCmd(),
		newClipCmd(),
		historyCmd(),
		newConfigCmd(),
	}
} 