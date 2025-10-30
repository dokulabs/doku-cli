package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version string
	commit  string
	date    string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "doku",
	Short: "Doku - Local development environment manager",
	Long: `Doku is a CLI tool that simplifies running and managing Docker-based services locally.

It provides:
  • One-command setup for popular services (PostgreSQL, Redis, RabbitMQ, etc.)
  • HTTPS by default with local SSL certificates
  • Clean URLs via subdomain routing (service.doku.local)
  • Automatic service discovery and connection strings
  • Multi-version support for services
  • Resource management (CPU/Memory limits)

Get started with: doku init`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.doku/config.toml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "quiet mode (minimal output)")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
}

// initConfig reads in config file and ENV variables
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".doku" (without extension)
		configPath := home + "/.doku"
		viper.AddConfigPath(configPath)
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("DOKU")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// SetVersionInfo sets version information for the CLI
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}
