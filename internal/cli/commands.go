package cli

import (
	cmdpkg "github.com/berrythewa/clipman-daemon/internal/cli/cmd"
)

func init() {
	// Register all commands with the root command
	for _, command := range cmdpkg.GetCommands() {
		AddCommand(command)
	}
	
	// Set up version info for the version command
	cmdpkg.SetVersionInfo(Version, BuildTime, Commit)
} 