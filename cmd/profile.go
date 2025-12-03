package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/profile"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage service profiles (development/production configurations)",
	Long: `Manage service configuration profiles.

Profiles allow you to define different configurations for development and production environments.
Each profile can specify environment variables, resource limits, and feature flags.

Examples:
  doku profile list                     # List all profiles
  doku profile show postgres            # Show profiles for postgres
  doku profile create postgres          # Create default profiles for postgres
  doku profile apply postgres --production  # Apply production profile`,
	Aliases: []string{"profiles"},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services with profiles",
	Long: `List all services that have profiles defined.

Example:
  doku profile list`,
	Args: cobra.NoArgs,
	RunE: runProfileList,
}

var profileShowCmd = &cobra.Command{
	Use:   "show <service>",
	Short: "Show profiles for a service",
	Long: `Show all profiles defined for a service.

Example:
  doku profile show postgres
  doku profile show redis`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileShow,
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <service>",
	Short: "Create default profiles for a service",
	Long: `Create default development and production profiles for a service.

This creates a profiles file with sensible defaults that you can customize.

Example:
  doku profile create postgres
  doku profile create redis`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileCreate,
}

var profileApplyCmd = &cobra.Command{
	Use:   "apply <service>",
	Short: "Apply a profile to a running service",
	Long: `Apply a profile configuration to a running service.

This will update the service's environment variables and settings based on the profile.

Example:
  doku profile apply postgres --profile production
  doku profile apply postgres --development
  doku profile apply postgres --production`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileApply,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <service>",
	Short: "Delete profiles for a service",
	Long: `Delete all profiles for a service.

Example:
  doku profile delete postgres`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileDelete,
}

var (
	profileName       string
	profileDev        bool
	profileProd       bool
	profileForce      bool
)

func init() {
	rootCmd.AddCommand(profileCmd)

	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileApplyCmd)
	profileCmd.AddCommand(profileDeleteCmd)

	profileApplyCmd.Flags().StringVarP(&profileName, "profile", "p", "", "Profile name to apply")
	profileApplyCmd.Flags().BoolVar(&profileDev, "development", false, "Apply development profile")
	profileApplyCmd.Flags().BoolVar(&profileProd, "production", false, "Apply production profile")

	profileCreateCmd.Flags().BoolVarP(&profileForce, "force", "f", false, "Overwrite existing profiles")
}

func runProfileList(cmd *cobra.Command, args []string) error {
	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create profile manager
	profileMgr := profile.NewManager(cfgMgr.GetDokuDir())

	// List all services with profiles
	services, err := profileMgr.ListAllServices()
	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(services) == 0 {
		fmt.Println()
		color.Yellow("No profiles defined yet.")
		fmt.Println()
		color.Cyan("Create profiles for a service with:")
		fmt.Println("  doku profile create <service>")
		fmt.Println()
		return nil
	}

	fmt.Println()
	color.Cyan("Services with Profiles")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tPROFILES\tDEFAULT\n")
	fmt.Fprintf(w, "-------\t--------\t-------\n")

	for _, svc := range services {
		profiles, err := profileMgr.GetServiceProfiles(svc)
		if err != nil {
			fmt.Fprintf(w, "%s\t%s\t%s\n", svc, color.YellowString("error"), "-")
			continue
		}

		profileNames := ""
		for name := range profiles.Profiles {
			if profileNames != "" {
				profileNames += ", "
			}
			profileNames += name
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", svc, profileNames, profiles.Default)
	}

	w.Flush()
	fmt.Println()

	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create profile manager
	profileMgr := profile.NewManager(cfgMgr.GetDokuDir())

	// Get profiles for service
	profiles, err := profileMgr.GetServiceProfiles(serviceName)
	if err != nil {
		return fmt.Errorf("no profiles found for '%s'. Create with: doku profile create %s", serviceName, serviceName)
	}

	fmt.Println()
	color.Cyan("Profiles for '%s'", serviceName)
	fmt.Println()
	color.New(color.Faint).Printf("Default profile: %s\n", profiles.Default)
	fmt.Println()

	for name, p := range profiles.Profiles {
		// Profile header
		typeColor := color.New(color.FgCyan)
		if p.Type == profile.ProfileProduction {
			typeColor = color.New(color.FgGreen)
		} else if p.Type == profile.ProfileDevelopment {
			typeColor = color.New(color.FgYellow)
		}

		fmt.Printf("[%s] %s\n", typeColor.Sprint(name), p.Description)
		fmt.Println()

		// Resources
		if p.Resources.MemoryLimit != "" || p.Resources.CPULimit != "" {
			color.New(color.Faint).Println("  Resources:")
			if p.Resources.MemoryLimit != "" {
				fmt.Printf("    Memory: %s (min: %s)\n", p.Resources.MemoryLimit, p.Resources.MemoryMin)
			}
			if p.Resources.CPULimit != "" {
				fmt.Printf("    CPU:    %s (min: %s)\n", p.Resources.CPULimit, p.Resources.CPUMin)
			}
		}

		// Environment
		if len(p.Environment) > 0 {
			color.New(color.Faint).Println("  Environment:")
			for k, v := range p.Environment {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}

		// Features
		color.New(color.Faint).Println("  Features:")
		fmt.Printf("    Debug: %s  SSL: %s  Logging: %s  Metrics: %s\n",
			formatBool(p.Features.Debug),
			formatBool(p.Features.SSL),
			formatBool(p.Features.Logging),
			formatBool(p.Features.Metrics))
		fmt.Printf("    Health Check: %s  Auto-Restart: %s  Resource Limits: %s\n",
			formatBool(p.Features.HealthCheck),
			formatBool(p.Features.AutoRestart),
			formatBool(p.Features.ResourceLimits))

		fmt.Println()
	}

	// Show usage
	color.New(color.Faint).Println("Apply a profile with:")
	color.New(color.Faint).Printf("  doku profile apply %s --profile <name>\n", serviceName)
	fmt.Println()

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create profile manager
	profileMgr := profile.NewManager(cfgMgr.GetDokuDir())

	// Check if profiles already exist
	if profileMgr.HasProfiles(serviceName) && !profileForce {
		color.Yellow("Profiles already exist for '%s'", serviceName)
		fmt.Println()
		color.Cyan("Use --force to overwrite, or edit the file directly:")
		fmt.Printf("  %s/%s.toml\n", profileMgr.GetProfilesDir(), serviceName)
		fmt.Println()
		return nil
	}

	// Create default profiles
	profiles, err := profileMgr.CreateDefaultProfiles(serviceName)
	if err != nil {
		return fmt.Errorf("failed to create profiles: %w", err)
	}

	color.Green("Created profiles for '%s'", serviceName)
	fmt.Println()

	fmt.Println("Profiles created:")
	for name, p := range profiles.Profiles {
		fmt.Printf("  • %s - %s\n", name, p.Description)
	}
	fmt.Println()

	color.Cyan("Edit profiles at:")
	fmt.Printf("  %s/%s.toml\n", profileMgr.GetProfilesDir(), serviceName)
	fmt.Println()

	color.Cyan("View profiles with:")
	fmt.Printf("  doku profile show %s\n", serviceName)
	fmt.Println()

	return nil
}

func runProfileApply(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Determine which profile to apply
	targetProfile := profileName
	if profileDev {
		targetProfile = "development"
	} else if profileProd {
		targetProfile = "production"
	}

	if targetProfile == "" {
		color.Yellow("Please specify a profile to apply:")
		fmt.Println("  --profile <name>   Apply a specific profile")
		fmt.Println("  --development      Apply development profile")
		fmt.Println("  --production       Apply production profile")
		return nil
	}

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create profile manager
	profileMgr := profile.NewManager(cfgMgr.GetDokuDir())

	// Get the profile
	p, err := profileMgr.GetProfile(serviceName, targetProfile)
	if err != nil {
		return fmt.Errorf("profile '%s' not found for service '%s'", targetProfile, serviceName)
	}

	// Get the service instance
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	instance, exists := cfg.Instances[serviceName]
	if !exists {
		return fmt.Errorf("service '%s' is not installed", serviceName)
	}

	// Apply profile to instance
	fmt.Println()
	color.Cyan("Applying '%s' profile to '%s'", targetProfile, serviceName)
	fmt.Println()

	// Merge environment variables
	if instance.Environment == nil {
		instance.Environment = make(map[string]string)
	}
	instance.Environment = p.MergeEnvironment(instance.Environment)

	// Update resource config based on profile
	if p.Features.ResourceLimits {
		instance.Resources.MemoryLimit = p.Resources.MemoryLimit
		instance.Resources.CPULimit = p.Resources.CPULimit
	}

	// Save updated instance
	if err := cfgMgr.UpdateInstance(serviceName, instance); err != nil {
		return fmt.Errorf("failed to update instance configuration: %w", err)
	}

	color.Green("Profile applied successfully!")
	fmt.Println()

	// Show what was applied
	fmt.Println("Applied settings:")
	if len(p.Environment) > 0 {
		fmt.Println("  Environment:")
		for k, v := range p.Environment {
			fmt.Printf("    %s=%s\n", k, v)
		}
	}

	if p.Features.ResourceLimits {
		fmt.Println("  Resources:")
		fmt.Printf("    Memory: %s, CPU: %s\n", p.Resources.MemoryLimit, p.Resources.CPULimit)
	}
	fmt.Println()

	color.Yellow("Note: Restart the service for changes to take effect:")
	fmt.Printf("  doku restart %s\n", serviceName)
	fmt.Println()

	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create profile manager
	profileMgr := profile.NewManager(cfgMgr.GetDokuDir())

	if !profileMgr.HasProfiles(serviceName) {
		color.Yellow("No profiles found for '%s'", serviceName)
		return nil
	}

	if err := profileMgr.DeleteProfiles(serviceName); err != nil {
		return fmt.Errorf("failed to delete profiles: %w", err)
	}

	color.Green("Deleted profiles for '%s'", serviceName)
	fmt.Println()

	return nil
}

func formatBool(b bool) string {
	if b {
		return color.GreenString("yes")
	}
	return color.New(color.Faint).Sprint("no")
}
