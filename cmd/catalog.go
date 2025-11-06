package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	catalogCategory string
	catalogSearch   string
	catalogVerbose  bool
	catalogSource   string // URL, branch, or tag for catalog update
)

var catalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Manage service catalog",
	Long:  `Browse, search, and update the Doku service catalog`,
	RunE:  runCatalogList, // Default to listing services
}

var catalogListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available services",
	Long:  `List all available services in the catalog, optionally filtered by category`,
	RunE:  runCatalogList,
}

var catalogSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for services",
	Long:  `Search for services by name, description, or tags`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCatalogSearch,
}

var catalogUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update service catalog",
	Long: `Download the service catalog from GitHub.

By default, downloads from the main branch. You can specify a different source:

  # Update from a specific branch
  doku catalog update --source develop

  # Update from a specific tag
  doku catalog update --source v1.2.0

  # Update from a custom URL
  doku catalog update --source https://example.com/catalog.tar.gz

You can also use the DOKU_CATALOG_SOURCE environment variable:
  export DOKU_CATALOG_SOURCE=develop
  doku catalog update`,
	RunE: runCatalogUpdate,
}

var catalogShowCmd = &cobra.Command{
	Use:   "show <service>",
	Short: "Show service details",
	Long:  `Display detailed information about a specific service`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCatalogShow,
}

func init() {
	rootCmd.AddCommand(catalogCmd)

	// Add subcommands
	catalogCmd.AddCommand(catalogListCmd)
	catalogCmd.AddCommand(catalogSearchCmd)
	catalogCmd.AddCommand(catalogUpdateCmd)
	catalogCmd.AddCommand(catalogShowCmd)

	// Flags for list command
	catalogListCmd.Flags().StringVarP(&catalogCategory, "category", "c", "", "Filter by category")
	catalogListCmd.Flags().BoolVarP(&catalogVerbose, "verbose", "v", false, "Show detailed information")

	// Flags for show command
	catalogShowCmd.Flags().BoolVarP(&catalogVerbose, "verbose", "v", false, "Show all versions")

	// Flags for update command
	catalogUpdateCmd.Flags().StringVarP(&catalogSource, "source", "s", "", "Catalog source (branch name, tag name, or full URL)")
}

func runCatalogList(cmd *cobra.Command, args []string) error {
	// Get config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create catalog manager
	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Check if catalog exists
	if !catalogMgr.CatalogExists() {
		color.Yellow("⚠️  Catalog not found. Please run 'doku catalog update' first.")
		return nil
	}

	// Get services
	var services []*types.CatalogService
	if catalogCategory != "" {
		services, err = catalogMgr.ListServicesByCategory(catalogCategory)
		if err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}
		color.Cyan("Services in category '%s':\n", catalogCategory)
	} else {
		services, err = catalogMgr.ListServices()
		if err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}
		color.Cyan("Available services:\n")
	}

	if len(services) == 0 {
		fmt.Println("No services found.")
		return nil
	}

	// Sort services by name
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	// Display services
	for _, service := range services {
		displayService(service, catalogVerbose)
	}

	fmt.Printf("\nTotal: %d service(s)\n", len(services))
	return nil
}

func runCatalogSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")

	// Get config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create catalog manager
	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Check if catalog exists
	if !catalogMgr.CatalogExists() {
		color.Yellow("⚠️  Catalog not found. Please run 'doku catalog update' first.")
		return nil
	}

	// Search services
	services, err := catalogMgr.SearchServices(query)
	if err != nil {
		return fmt.Errorf("failed to search services: %w", err)
	}

	color.Cyan("Search results for '%s':\n", query)

	if len(services) == 0 {
		fmt.Println("No services found.")
		return nil
	}

	// Display services
	for _, service := range services {
		displayService(service, false)
	}

	fmt.Printf("\nFound: %d service(s)\n", len(services))
	return nil
}

func runCatalogUpdate(cmd *cobra.Command, args []string) error {
	// Get config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create catalog manager
	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Determine catalog source
	// Priority: command flag > environment variable > default
	source := catalogSource
	if source == "" {
		source = os.Getenv("DOKU_CATALOG_SOURCE")
	}

	// If custom source is specified, set it
	if source != "" {
		catalogURL := buildCatalogURL(source)
		catalogMgr.SetCatalogURL(catalogURL)
		color.Cyan("Using catalog source: %s", source)
	}

	// Check if local catalog exists
	hasLocalCatalog := catalogMgr.CatalogExists()

	fmt.Println("Updating service catalog...")

	// Fetch catalog
	if err := catalogMgr.FetchCatalog(); err != nil {
		// If download fails but we have a local catalog, keep using it
		if hasLocalCatalog {
			color.Yellow("⚠️  Could not download latest catalog from GitHub")
			fmt.Println()
			color.New(color.Faint).Println("Reason: The catalog repository has no published releases yet.")
			color.New(color.Faint).Println("This is expected during development.")
			fmt.Println()
			color.Cyan("✓ Using existing local catalog")
			fmt.Println()

			// Show current catalog info
			if version, err := catalogMgr.GetCatalogVersion(); err == nil && version != "" {
				fmt.Printf("  Current version: %s\n", version)
			}

			services, err := catalogMgr.ListServices()
			if err == nil {
				fmt.Printf("  Services available: %d\n", len(services))
			}

			fmt.Println()
			color.New(color.Faint).Println("💡 Your local catalog is fully functional. You can:")
			color.New(color.Faint).Println("   • Browse services: doku catalog list")
			color.New(color.Faint).Println("   • Install services: doku install <service>")
			color.New(color.Faint).Println("   • The catalog will auto-update once GitHub releases are published")

			return nil
		}

		// No local catalog and download failed
		return fmt.Errorf("failed to update catalog: %w\n\nThe catalog repository has no published releases yet.\nFor development, you can copy the catalog manually:\n  cp -r /path/to/doku-catalog/* ~/.doku/catalog/", err)
	}

	// Validate catalog
	if err := catalogMgr.ValidateCatalog(); err != nil {
		color.Red("⚠️  Catalog validation failed: %v", err)
		return nil
	}

	// Get catalog version
	version, err := catalogMgr.GetCatalogVersion()
	if err != nil {
		color.Yellow("⚠️  Could not determine catalog version")
	} else {
		// Update config with catalog version
		if err := cfgMgr.UpdateCatalogVersion(version); err != nil {
			color.Yellow("⚠️  Could not save catalog version: %v", err)
		}
	}

	color.Green("✓ Catalog updated successfully")
	if version != "" {
		fmt.Printf("  Version: %s\n", version)
	}

	// Show statistics
	services, _ := catalogMgr.ListServices()
	fmt.Printf("  Services: %d\n", len(services))

	return nil
}

func runCatalogShow(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Get config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create catalog manager
	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Check if catalog exists
	if !catalogMgr.CatalogExists() {
		color.Yellow("⚠️  Catalog not found. Please run 'doku catalog update' first.")
		return nil
	}

	// Get service
	service, err := catalogMgr.GetService(serviceName)
	if err != nil {
		return fmt.Errorf("service not found: %w", err)
	}

	// Display detailed service information
	displayServiceDetails(service, catalogVerbose)

	return nil
}

// Helper functions for displaying service information

func displayService(service *types.CatalogService, verbose bool) {
	icon := service.Icon
	if icon == "" {
		icon = "📦"
	}

	fmt.Printf("\n%s %s", icon, color.CyanString(service.Name))
	if service.Category != "" {
		fmt.Printf(" [%s]", color.YellowString(service.Category))
	}
	fmt.Println()

	if service.Description != "" {
		fmt.Printf("  %s\n", service.Description)
	}

	if verbose {
		// Show versions
		versions := make([]string, 0, len(service.Versions))
		for version := range service.Versions {
			versions = append(versions, version)
		}
		sort.Strings(versions)
		fmt.Printf("  Versions: %s\n", strings.Join(versions, ", "))

		// Show tags
		if len(service.Tags) > 0 {
			fmt.Printf("  Tags: %s\n", strings.Join(service.Tags, ", "))
		}
	}
}

func displayServiceDetails(service *types.CatalogService, showAllVersions bool) {
	icon := service.Icon
	if icon == "" {
		icon = "📦"
	}

	// Header
	fmt.Printf("\n%s %s\n", icon, color.New(color.Bold, color.FgCyan).Sprint(service.Name))
	fmt.Println(strings.Repeat("=", len(service.Name)+4))

	// Description
	if service.Description != "" {
		fmt.Printf("\n%s\n", service.Description)
	}

	// Metadata
	fmt.Println()
	if service.Category != "" {
		fmt.Printf("Category: %s\n", color.YellowString(service.Category))
	}

	if len(service.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(service.Tags, ", "))
	}

	// Links
	if service.Links != nil {
		fmt.Println()
		if service.Links.Homepage != "" {
			fmt.Printf("Homepage: %s\n", service.Links.Homepage)
		}
		if service.Links.Documentation != "" {
			fmt.Printf("Documentation: %s\n", service.Links.Documentation)
		}
		if service.Links.Repository != "" {
			fmt.Printf("Repository: %s\n", service.Links.Repository)
		}
	}

	// Versions
	fmt.Printf("\n%s\n", color.New(color.Bold).Sprint("Available Versions:"))

	versions := make([]string, 0, len(service.Versions))
	for version := range service.Versions {
		versions = append(versions, version)
	}
	sort.Strings(versions)

	if showAllVersions {
		// Show detailed version info
		for _, version := range versions {
			spec := service.Versions[version]
			fmt.Printf("\n  %s\n", color.CyanString(version))
			fmt.Printf("    Image: %s\n", spec.Image)
			if spec.Description != "" {
				fmt.Printf("    Description: %s\n", spec.Description)
			}
			fmt.Printf("    Port: %d\n", spec.Port)
			if spec.Protocol != "" {
				fmt.Printf("    Protocol: %s\n", spec.Protocol)
			}
			if spec.Resources != nil {
				fmt.Printf("    Memory: %s - %s\n", spec.Resources.MemoryMin, spec.Resources.MemoryMax)
				fmt.Printf("    CPU: %s - %s\n", spec.Resources.CPUMin, spec.Resources.CPUMax)
			}
		}
	} else {
		// Show compact version list
		fmt.Printf("  %s\n", strings.Join(versions, ", "))
		fmt.Println("\nRun with --verbose to see detailed version information")
	}

	fmt.Println()
	color.Cyan("To install: doku install %s [version]", service.Name)
	fmt.Println()
}

// buildCatalogURL constructs the catalog URL from a source specification
// Supports:
// - Full URLs: https://github.com/user/repo/archive/refs/heads/branch.tar.gz
// - Branch names: "main", "develop", "feature-branch"
// - Tag names: "v1.0.0", "1.0.0"
func buildCatalogURL(source string) string {
	// If it's already a full URL, use it as-is
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return source
	}

	// Check if it looks like a tag (starts with 'v' and has a number, or just numbers and dots)
	if strings.HasPrefix(source, "v") || strings.Contains(source, ".") {
		// Treat as a tag
		return fmt.Sprintf("https://github.com/dokulabs/doku-catalog/archive/refs/tags/%s.tar.gz", source)
	}

	// Otherwise, treat as a branch name
	return fmt.Sprintf("https://github.com/dokulabs/doku-catalog/archive/refs/heads/%s.tar.gz", source)
}
