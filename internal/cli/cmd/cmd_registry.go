package cmd

import (
	"github.com/spf13/cobra"
)

// GetCommands returns all commands for registration
func GetCommands() []*cobra.Command {
	return []*cobra.Command{
		versionCmd,
		runCmd,
		historyCmd,
		flushCmd,
		serviceCmd,
		pairCmd,
	}
} 