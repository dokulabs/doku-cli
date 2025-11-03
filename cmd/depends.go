package cmd

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/dependencies"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	dependsTree     bool
	dependsReverse  bool
	dependsValidate bool
)

var dependsCmd = &cobra.Command{
	Use:   "depends <service>[:<version>]",
	Short: "Show dependencies for a service",
	Long: `Show the dependency tree for a service from the catalog.

This command displays:
  • Direct dependencies of the service
  • Complete dependency tree (with --tree flag)
  • Reverse dependencies - services that depend on this service (with --reverse flag)
  • Dependency validation (with --validate flag)

Examples:
  doku depends signoz              # Show direct dependencies
  doku depends signoz --tree       # Show full dependency tree
  doku depends clickhouse --reverse  # Show what depends on clickhouse
  doku depends signoz --validate   # Validate dependency graph`,
	Args: cobra.ExactArgs(1),
	RunE: runDepends,
}

func init() {
	rootCmd.AddCommand(dependsCmd)

	dependsCmd.Flags().BoolVarP(&dependsTree, "tree", "t", false, "Show full dependency tree")
	dependsCmd.Flags().BoolVarP(&dependsReverse, "reverse", "r", false, "Show reverse dependencies (what depends on this)")
	dependsCmd.Flags().BoolVar(&dependsValidate, "validate", false, "Validate dependency graph (detect circular dependencies)")
}

func runDepends(cmd *cobra.Command, args []string) error {
	serviceSpec := args[0]

	// Parse service:version
	parts := strings.SplitN(serviceSpec, ":", 2)
	serviceName := parts[0]
	version := ""
	if len(parts) > 1 {
		version = parts[1]
	}

	// Create managers
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	catalogMgr := catalog.NewManager(cfgMgr.GetCatalogDir())

	// Check if catalog exists
	if !catalogMgr.CatalogExists() {
		color.Yellow("⚠️  Catalog not found. Please run 'doku catalog update' first.")
		return nil
	}

	// Get service from catalog
	catalogService, err := catalogMgr.GetService(serviceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found in catalog", serviceName)
	}

	// Get service version
	spec, err := catalogMgr.GetServiceVersion(serviceName, version)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// Determine actual version
	actualVersion := version
	if actualVersion == "" || actualVersion == "latest" {
		for v, s := range catalogService.Versions {
			if s == spec {
				actualVersion = v
				break
			}
		}
	}

	// Show service header
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Printf("📦 %s %s (v%s)\n", catalogService.Icon, catalogService.Name, actualVersion)
	fmt.Println(catalogService.Description)
	fmt.Println()

	// Handle reverse dependencies
	if dependsReverse {
		return showReverseDependencies(catalogMgr, serviceName)
	}

	// Handle validation
	if dependsValidate {
		return validateDependencies(catalogMgr, cfgMgr, serviceName, actualVersion)
	}

	// Create dependency resolver
	resolver := dependencies.NewResolver(catalogMgr, cfgMgr)

	// Resolve dependencies
	result, err := resolver.Resolve(serviceName, actualVersion)
	if err != nil {
		if dependencies.IsCircularDependency(err) {
			color.Red("✗ Circular dependency detected:")
			fmt.Println(err.Error())
			fmt.Println()
			color.Yellow("To fix this, update the service catalog configuration.")
			return nil
		}
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Show dependency tree
	if dependsTree {
		color.Cyan("Dependency Tree:")
		fmt.Println()
		tree := resolver.GetDependencyTree(result, serviceName)
		fmt.Println(tree)
	} else {
		// Show direct dependencies
		if len(spec.Dependencies) == 0 {
			color.Green("✓ No dependencies")
			fmt.Println()
			return nil
		}

		color.Cyan("Direct Dependencies:")
		fmt.Println()
		for _, dep := range spec.Dependencies {
			required := ""
			if dep.Required {
				required = color.RedString(" (required)")
			} else {
				required = color.New(color.Faint).Sprint(" (optional)")
			}
			fmt.Printf("  • %s %s%s\n", dep.Name, dep.Version, required)
		}
		fmt.Println()

		// Show installation order
		if len(result.InstallOrder) > 1 {
			color.Cyan("Installation Order:")
			fmt.Println()
			for i, node := range result.InstallOrder {
				installedMark := ""
				if node.IsInstalled {
					installedMark = color.GreenString(" ✓")
				}
				fmt.Printf("  %d. %s (%s)%s\n", i+1, node.ServiceName, node.Version, installedMark)
			}
			fmt.Println()
		}
	}

	// Show helpful commands
	color.New(color.Faint).Println("Useful commands:")
	color.New(color.Faint).Printf("  doku depends %s --tree       # Show full dependency tree\n", serviceName)
	color.New(color.Faint).Printf("  doku depends %s --validate   # Validate dependencies\n", serviceName)
	color.New(color.Faint).Printf("  doku install %s              # Install with dependencies\n", serviceName)
	fmt.Println()

	return nil
}

// showReverseDependencies shows services that depend on this service
func showReverseDependencies(catalogMgr *catalog.Manager, serviceName string) error {
	// Get all services from catalog
	services, err := catalogMgr.ListServices()
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	// Find services that depend on this one
	dependents := make([]string, 0)
	for _, svc := range services {
		// Check all versions
		for version := range svc.Versions {
			spec, err := catalogMgr.GetServiceVersion(svc.Name, version)
			if err != nil {
				continue
			}

			// Check if this service depends on our target
			for _, dep := range spec.Dependencies {
				if dep.Name == serviceName {
					dependents = append(dependents, fmt.Sprintf("%s (%s)", svc.Name, version))
					break
				}
			}
		}
	}

	if len(dependents) == 0 {
		color.Yellow("No services depend on %s", serviceName)
		fmt.Println()
		return nil
	}

	color.Cyan("Services that depend on %s:", serviceName)
	fmt.Println()
	for _, dependent := range dependents {
		fmt.Printf("  • %s\n", dependent)
	}
	fmt.Println()

	return nil
}

// validateDependencies validates the dependency graph
func validateDependencies(catalogMgr *catalog.Manager, cfgMgr *config.Manager, serviceName, version string) error {
	resolver := dependencies.NewResolver(catalogMgr, cfgMgr)

	fmt.Println("Validating dependency graph...")
	fmt.Println()

	err := resolver.ValidateDependencies(serviceName, version)
	if err != nil {
		if dependencies.IsCircularDependency(err) {
			color.Red("✗ Validation failed: Circular dependency detected")
			fmt.Println()
			fmt.Println(err.Error())
			fmt.Println()
			return nil
		}
		color.Red("✗ Validation failed: %v", err)
		fmt.Println()
		return nil
	}

	color.Green("✓ Dependency graph is valid")
	fmt.Println()

	// Show resolved dependencies
	result, err := resolver.Resolve(serviceName, version)
	if err != nil {
		return err
	}

	if len(result.InstallOrder) > 1 {
		fmt.Printf("Total dependencies: %d\n", len(result.InstallOrder)-1)
		fmt.Println()
	}

	return nil
}
