package dependencies

import (
	"fmt"

	"github.com/dokulabs/doku-cli/internal/catalog"
	"github.com/dokulabs/doku-cli/internal/config"
)

// Resolver handles dependency resolution and installation order calculation
type Resolver struct {
	catalogMgr *catalog.Manager
	configMgr  *config.Manager
}

// NewResolver creates a new dependency resolver
func NewResolver(catalogMgr *catalog.Manager, configMgr *config.Manager) *Resolver {
	return &Resolver{
		catalogMgr: catalogMgr,
		configMgr:  configMgr,
	}
}

// ResolutionResult contains the resolved installation order
type ResolutionResult struct {
	InstallOrder []DependencyNode    // Topologically sorted installation order
	Graph        map[string][]string // Dependency graph for visualization
	AllNodes     map[string]*DependencyNode // All nodes in the graph
}

// DependencyNode represents a service to be installed
type DependencyNode struct {
	ServiceName  string            // Service name
	Version      string            // Version to install
	Required     bool              // Is this dependency required
	Environment  map[string]string // Environment variable overrides
	IsInstalled  bool              // Whether already installed
	Depth        int               // Depth in dependency tree (0 = root)
}

// Resolve resolves dependencies for a service and returns installation order
// This performs a depth-first search to build the dependency graph and then
// topologically sorts it to determine the correct installation order
func (r *Resolver) Resolve(serviceName, version string) (*ResolutionResult, error) {
	if version == "" {
		version = "latest"
	}

	// Get service spec to check if it exists
	_, err := r.catalogMgr.GetServiceVersion(serviceName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get service spec for %s: %w", serviceName, err)
	}

	// Build dependency graph
	graph := make(map[string][]string)
	nodes := make(map[string]*DependencyNode)

	// Start DFS from the target service
	visiting := make(map[string]bool)
	if err := r.buildDependencyGraph(serviceName, version, graph, nodes, visiting, 0); err != nil {
		return nil, err
	}

	// Perform topological sort to get installation order
	sorted, err := r.topologicalSort(graph, nodes)
	if err != nil {
		return nil, err
	}

	return &ResolutionResult{
		InstallOrder: sorted,
		Graph:        graph,
		AllNodes:     nodes,
	}, nil
}

// buildDependencyGraph recursively builds the dependency graph using DFS
func (r *Resolver) buildDependencyGraph(
	serviceName, version string,
	graph map[string][]string,
	nodes map[string]*DependencyNode,
	visiting map[string]bool,
	depth int,
) error {
	// Check for circular dependency
	if visiting[serviceName] {
		return &CircularDependencyError{
			Service: serviceName,
			Chain:   r.buildChain(visiting),
		}
	}

	// If already processed, skip (but update depth if shallower path found)
	if existing, exists := nodes[serviceName]; exists {
		if depth < existing.Depth {
			existing.Depth = depth
		}
		return nil
	}

	// Mark as visiting (for circular dependency detection)
	visiting[serviceName] = true
	defer delete(visiting, serviceName)

	// Get service spec
	spec, err := r.catalogMgr.GetServiceVersion(serviceName, version)
	if err != nil {
		return fmt.Errorf("failed to get spec for %s@%s: %w", serviceName, version, err)
	}

	// Check if already installed
	isInstalled := r.configMgr.HasInstance(serviceName)

	// Create node
	node := &DependencyNode{
		ServiceName: serviceName,
		Version:     version,
		Required:    true,
		IsInstalled: isInstalled,
		Depth:       depth,
	}
	nodes[serviceName] = node

	// Process dependencies
	if spec.HasDependencies() {
		dependencies := make([]string, 0, len(spec.Dependencies))

		for _, dep := range spec.Dependencies {
			// Add to graph
			dependencies = append(dependencies, dep.Name)

			// Determine version to use
			depVersion := dep.Version
			if depVersion == "" {
				depVersion = "latest"
			}

			// Recurse into dependency
			if err := r.buildDependencyGraph(dep.Name, depVersion, graph, nodes, visiting, depth+1); err != nil {
				return err
			}

			// Update node with dependency-specific configuration
			if depNode := nodes[dep.Name]; depNode != nil {
				depNode.Required = dep.Required
				if len(dep.Environment) > 0 {
					// Merge environment, not replace
					if depNode.Environment == nil {
						depNode.Environment = make(map[string]string)
					}
					for k, v := range dep.Environment {
						depNode.Environment[k] = v
					}
				}
			}
		}

		// Store dependencies in graph
		graph[serviceName] = dependencies
	}

	return nil
}

// topologicalSort performs topological sort using DFS
// Returns nodes in the order they should be installed (dependencies first)
func (r *Resolver) topologicalSort(
	graph map[string][]string,
	nodes map[string]*DependencyNode,
) ([]DependencyNode, error) {
	var result []DependencyNode
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(string) error
	visit = func(node string) error {
		// Check for circular dependency
		if visiting[node] {
			return &CircularDependencyError{
				Service: node,
				Chain:   r.buildChain(visiting),
			}
		}

		// Already visited
		if visited[node] {
			return nil
		}

		// Mark as visiting
		visiting[node] = true

		// Visit all dependencies first (DFS)
		for _, dep := range graph[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		// Mark as visited
		visiting[node] = false
		visited[node] = true

		// Add to result (post-order: dependencies before dependents)
		if nodeData, exists := nodes[node]; exists {
			result = append(result, *nodeData)
		}

		return nil
	}

	// Visit all nodes
	for node := range nodes {
		if !visited[node] {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// buildChain builds a dependency chain from the visiting map for error messages
func (r *Resolver) buildChain(visiting map[string]bool) []string {
	chain := make([]string, 0, len(visiting))
	for service := range visiting {
		chain = append(chain, service)
	}
	return chain
}

// GetMissingDependencies returns dependencies that need to be installed
func (r *Resolver) GetMissingDependencies(result *ResolutionResult) []DependencyNode {
	var missing []DependencyNode
	for _, node := range result.InstallOrder {
		if !node.IsInstalled && node.Required {
			missing = append(missing, node)
		}
	}
	return missing
}

// GetInstalledDependencies returns dependencies that are already installed
func (r *Resolver) GetInstalledDependencies(result *ResolutionResult) []DependencyNode {
	var installed []DependencyNode
	for _, node := range result.InstallOrder {
		if node.IsInstalled {
			installed = append(installed, node)
		}
	}
	return installed
}

// GetDependencyTree returns a human-readable dependency tree
func (r *Resolver) GetDependencyTree(result *ResolutionResult, rootService string) string {
	// Build tree structure
	var buildTree func(service string, prefix string, isLast bool) string
	buildTree = func(service string, prefix string, isLast bool) string {
		node := result.AllNodes[service]
		if node == nil {
			return ""
		}

		// Current node
		marker := "├── "
		if isLast {
			marker = "└── "
		}
		if prefix == "" {
			marker = ""
		}

		status := "✓"
		if !node.IsInstalled {
			status = "○"
		}

		tree := fmt.Sprintf("%s%s%s %s (%s)\n", prefix, marker, status, service, node.Version)

		// Children
		deps := result.Graph[service]
		for i, dep := range deps {
			childPrefix := prefix
			if prefix != "" {
				if isLast {
					childPrefix += "    "
				} else {
					childPrefix += "│   "
				}
			}
			tree += buildTree(dep, childPrefix, i == len(deps)-1)
		}

		return tree
	}

	return buildTree(rootService, "", true)
}

// ValidateDependencies checks if all dependencies can be resolved
func (r *Resolver) ValidateDependencies(serviceName, version string) error {
	_, err := r.Resolve(serviceName, version)
	return err
}

// CircularDependencyError represents a circular dependency error
type CircularDependencyError struct {
	Service string
	Chain   []string
}

func (e *CircularDependencyError) Error() string {
	return fmt.Sprintf("circular dependency detected: %s is part of cycle: %v", e.Service, e.Chain)
}

// IsCircularDependency checks if an error is a circular dependency error
func IsCircularDependency(err error) bool {
	_, ok := err.(*CircularDependencyError)
	return ok
}
