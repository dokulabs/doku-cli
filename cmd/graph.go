package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	graphFormat   string
	graphDetailed bool
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Display service dependency graph",
	Long: `Display the dependency graph of installed services.

Shows how services are connected and their dependencies on each other.

Output formats:
  - text:  ASCII art tree view (default)
  - dot:   Graphviz DOT format (for visualization tools)
  - mermaid: Mermaid diagram format

Examples:
  doku graph                    # Show dependency tree
  doku graph --detailed         # Show with container details
  doku graph --format dot       # Output Graphviz DOT format
  doku graph --format mermaid   # Output Mermaid diagram`,
	Aliases: []string{"deps", "dependencies"},
	RunE:    runGraph,
}

func init() {
	rootCmd.AddCommand(graphCmd)

	graphCmd.Flags().StringVarP(&graphFormat, "format", "f", "text", "Output format (text, dot, mermaid)")
	graphCmd.Flags().BoolVarP(&graphDetailed, "detailed", "d", false, "Show detailed container information")
}

func runGraph(cmd *cobra.Command, args []string) error {
	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Get config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if len(cfg.Instances) == 0 && len(cfg.Projects) == 0 {
		color.Yellow("No services installed")
		return nil
	}

	// Create Docker client for status checks
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Build dependency graph
	graph := buildDependencyGraph(cfg, serviceMgr)

	// Output based on format
	switch graphFormat {
	case "text":
		outputTextGraph(graph, cfg, graphDetailed)
	case "dot":
		outputDotGraph(graph, cfg)
	case "mermaid":
		outputMermaidGraph(graph, cfg)
	default:
		return fmt.Errorf("unsupported format: %s (use text, dot, or mermaid)", graphFormat)
	}

	return nil
}

// DependencyGraph represents the service dependency graph
type DependencyGraph struct {
	Nodes map[string]*GraphNode
	Edges []GraphEdge
}

// GraphNode represents a node in the dependency graph
type GraphNode struct {
	Name         string
	Type         string // "instance" or "project"
	ServiceType  string
	Status       string
	Dependencies []string
	DependedBy   []string
}

// GraphEdge represents an edge in the dependency graph
type GraphEdge struct {
	From string
	To   string
}

func buildDependencyGraph(cfg *types.Config, serviceMgr *service.Manager) *DependencyGraph {
	graph := &DependencyGraph{
		Nodes: make(map[string]*GraphNode),
		Edges: make([]GraphEdge, 0),
	}

	// Add instance nodes
	for name, instance := range cfg.Instances {
		status := string(instance.Status)
		if st, err := serviceMgr.GetStatus(name); err == nil {
			status = string(st)
		}

		node := &GraphNode{
			Name:         name,
			Type:         "instance",
			ServiceType:  instance.ServiceType,
			Status:       status,
			Dependencies: instance.Dependencies,
			DependedBy:   make([]string, 0),
		}
		graph.Nodes[name] = node
	}

	// Add project nodes
	for name, project := range cfg.Projects {
		status := string(project.Status)

		node := &GraphNode{
			Name:         name,
			Type:         "project",
			ServiceType:  "custom-project",
			Status:       status,
			Dependencies: project.Dependencies,
			DependedBy:   make([]string, 0),
		}
		graph.Nodes[name] = node
	}

	// Build edges and reverse dependencies
	for nodeName, node := range graph.Nodes {
		for _, dep := range node.Dependencies {
			graph.Edges = append(graph.Edges, GraphEdge{From: nodeName, To: dep})

			// Update DependedBy
			if depNode, exists := graph.Nodes[dep]; exists {
				depNode.DependedBy = append(depNode.DependedBy, nodeName)
			}
		}
	}

	return graph
}

func outputTextGraph(graph *DependencyGraph, cfg *types.Config, detailed bool) {
	fmt.Println()
	color.Cyan("Service Dependency Graph")
	fmt.Println()

	// Find root nodes (nodes with no dependencies)
	roots := make([]string, 0)
	dependents := make([]string, 0)

	for name, node := range graph.Nodes {
		if len(node.Dependencies) == 0 {
			roots = append(roots, name)
		} else {
			dependents = append(dependents, name)
		}
	}

	sort.Strings(roots)
	sort.Strings(dependents)

	// Print root services (no dependencies)
	if len(roots) > 0 {
		color.New(color.Bold).Println("Root Services (no dependencies):")
		for _, name := range roots {
			node := graph.Nodes[name]
			printServiceNode(node, detailed, "")
		}
		fmt.Println()
	}

	// Print services with dependencies
	if len(dependents) > 0 {
		color.New(color.Bold).Println("Services with Dependencies:")
		for _, name := range dependents {
			node := graph.Nodes[name]
			printServiceNode(node, detailed, "")

			// Print dependencies
			for i, dep := range node.Dependencies {
				prefix := "├── "
				if i == len(node.Dependencies)-1 {
					prefix = "└── "
				}

				depStatus := "unknown"
				if depNode, exists := graph.Nodes[dep]; exists {
					depStatus = depNode.Status
				}

				statusColor := getGraphStatusColor(depStatus)
				fmt.Printf("    %s%s (%s)\n", prefix, dep, statusColor(depStatus))
			}
			fmt.Println()
		}
	}

	// Print reverse dependencies summary
	fmt.Println()
	color.New(color.Bold).Println("Dependency Summary:")
	fmt.Println()

	for _, name := range append(roots, dependents...) {
		node := graph.Nodes[name]
		if len(node.DependedBy) > 0 {
			fmt.Printf("  %s is required by: %s\n",
				color.CyanString(name),
				strings.Join(node.DependedBy, ", "))
		}
	}

	fmt.Println()
}

func printServiceNode(node *GraphNode, detailed bool, prefix string) {
	statusColor := getGraphStatusColor(node.Status)
	typeIcon := "□"
	if node.Type == "project" {
		typeIcon = "◇"
	}

	fmt.Printf("%s%s %s (%s) - %s\n",
		prefix,
		typeIcon,
		color.CyanString(node.Name),
		node.ServiceType,
		statusColor(node.Status))

	if detailed {
		fmt.Printf("%s    Type: %s\n", prefix, node.Type)
		if len(node.Dependencies) > 0 {
			fmt.Printf("%s    Dependencies: %s\n", prefix, strings.Join(node.Dependencies, ", "))
		}
		if len(node.DependedBy) > 0 {
			fmt.Printf("%s    Required by: %s\n", prefix, strings.Join(node.DependedBy, ", "))
		}
	}
}

func outputDotGraph(graph *DependencyGraph, cfg *types.Config) {
	fmt.Println("digraph dependencies {")
	fmt.Println("  rankdir=TB;")
	fmt.Println("  node [shape=box, style=rounded];")
	fmt.Println()

	// Define nodes
	for name, node := range graph.Nodes {
		color := "lightblue"
		if node.Type == "project" {
			color = "lightgreen"
		}
		if node.Status == "running" {
			color = "palegreen"
		} else if node.Status == "stopped" {
			color = "lightgray"
		} else if node.Status == "failed" {
			color = "lightcoral"
		}

		label := fmt.Sprintf("%s\\n(%s)", name, node.ServiceType)
		fmt.Printf("  \"%s\" [label=\"%s\", fillcolor=%s, style=filled];\n",
			name, label, color)
	}
	fmt.Println()

	// Define edges
	for _, edge := range graph.Edges {
		fmt.Printf("  \"%s\" -> \"%s\";\n", edge.From, edge.To)
	}

	fmt.Println("}")
}

func outputMermaidGraph(graph *DependencyGraph, cfg *types.Config) {
	fmt.Println("graph TD")

	// Define nodes with status-based styling
	for name, node := range graph.Nodes {
		shape := fmt.Sprintf("%s[%s]", name, name)
		if node.Type == "project" {
			shape = fmt.Sprintf("%s{{%s}}", name, name)
		}

		fmt.Printf("  %s\n", shape)
	}
	fmt.Println()

	// Define edges
	for _, edge := range graph.Edges {
		fmt.Printf("  %s --> %s\n", edge.From, edge.To)
	}

	// Add styling
	fmt.Println()
	fmt.Println("  classDef running fill:#90EE90,stroke:#228B22")
	fmt.Println("  classDef stopped fill:#D3D3D3,stroke:#696969")
	fmt.Println("  classDef failed fill:#F08080,stroke:#DC143C")

	// Apply classes based on status
	running := make([]string, 0)
	stopped := make([]string, 0)
	failed := make([]string, 0)

	for name, node := range graph.Nodes {
		switch node.Status {
		case "running":
			running = append(running, name)
		case "stopped":
			stopped = append(stopped, name)
		case "failed":
			failed = append(failed, name)
		}
	}

	if len(running) > 0 {
		fmt.Printf("  class %s running\n", strings.Join(running, ","))
	}
	if len(stopped) > 0 {
		fmt.Printf("  class %s stopped\n", strings.Join(stopped, ","))
	}
	if len(failed) > 0 {
		fmt.Printf("  class %s failed\n", strings.Join(failed, ","))
	}
}

func getGraphStatusColor(status string) func(a ...interface{}) string {
	switch status {
	case "running":
		return color.New(color.FgGreen).SprintFunc()
	case "stopped":
		return color.New(color.FgYellow).SprintFunc()
	case "failed":
		return color.New(color.FgRed).SprintFunc()
	default:
		return color.New(color.Faint).SprintFunc()
	}
}

// Ensure we don't have unused imports
var _ = os.Stdout
