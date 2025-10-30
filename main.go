package main

import (
	"os"

	"github.com/dokulabs/doku-cli/cmd"
)

// Version information (set during build)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Set version info for commands to access
	cmd.SetVersionInfo(Version, Commit, BuildDate)

	// Execute root command
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
