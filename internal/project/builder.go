package project

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/fatih/color"
)

// Builder handles Docker build operations for projects
type Builder struct {
	docker *docker.Client
}

// DockerBuildOptions contains options for building a Docker image
type DockerBuildOptions struct {
	ContextPath    string             // Project directory path
	DockerfilePath string             // Path to Dockerfile
	Tags           []string           // Image tags
	NoCache        bool               // Build without cache
	Pull           bool               // Pull base image
	BuildArgs      map[string]*string // Build arguments
}

// buildMessage represents a single build output line
type buildMessage struct {
	Stream string `json:"stream"`
	Error  string `json:"error"`
}

// NewBuilder creates a new Docker builder
func NewBuilder(dockerClient *docker.Client) *Builder {
	return &Builder{
		docker: dockerClient,
	}
}

// Build builds a Docker image from a Dockerfile
func (b *Builder) Build(opts DockerBuildOptions) (string, error) {
	// Validate Dockerfile
	if err := b.ValidateDockerfile(opts.DockerfilePath); err != nil {
		return "", err
	}

	// Make paths absolute to avoid any relative path issues
	absContextPath, err := filepath.Abs(opts.ContextPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve context path: %w", err)
	}

	absDockerfilePath, err := filepath.Abs(opts.DockerfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve Dockerfile path: %w", err)
	}

	// Create build context
	buildContext, err := b.createBuildContext(absContextPath, absDockerfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create build context: %w", err)
	}
	defer buildContext.Close()

	// Get relative Dockerfile path for Docker build
	relDockerfile, err := filepath.Rel(absContextPath, absDockerfilePath)
	if err != nil {
		// If we can't get a relative path, try using just the basename
		// This happens when paths are on different volumes or other edge cases
		relDockerfile = filepath.Base(absDockerfilePath)
	}

	// Prepare build options
	// Note: BuildKit is controlled by the Docker daemon configuration
	// If you need SSH mounts, ensure BuildKit is enabled in Docker Desktop settings
	// or set DOCKER_BUILDKIT=1 before running doku commands
	buildOpts := types.ImageBuildOptions{
		Tags:       opts.Tags,
		Dockerfile: relDockerfile,
		NoCache:    opts.NoCache,
		Remove:     true,
		PullParent: opts.Pull,
		BuildArgs:  opts.BuildArgs,
	}

	// Execute build
	response, err := b.docker.ImageBuild(buildContext, buildOpts)
	if err != nil {
		errMsg := err.Error()
		// Check for common BuildKit-related errors and provide helpful messages
		if strings.Contains(errMsg, "--mount option requires BuildKit") {
			return "", fmt.Errorf("BuildKit is required for SSH mounts.\n\nTo fix this:\n  1. Enable BuildKit in Docker Desktop (Settings → Features in development → Use Docker Buildx)\n  2. Or set environment variable: export DOCKER_BUILDKIT=1\n  3. Then try again\n\nOriginal error: %w", err)
		}
		return "", fmt.Errorf("failed to start build: %w", err)
	}
	defer response.Body.Close()

	// Parse and display build output
	imageID, err := b.parseBuildOutput(response.Body)
	if err != nil {
		return "", err
	}

	return imageID, nil
}

// ValidateDockerfile checks if a Dockerfile exists and is readable
func (b *Builder) ValidateDockerfile(dockerfilePath string) error {
	info, err := os.Stat(dockerfilePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found: %s", dockerfilePath)
	}
	if err != nil {
		return fmt.Errorf("failed to access Dockerfile: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("Dockerfile path is a directory: %s", dockerfilePath)
	}

	// Check if file is readable
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return fmt.Errorf("Dockerfile is not readable: %w", err)
	}
	file.Close()

	return nil
}

// createBuildContext creates a tar archive of the project directory
func (b *Builder) createBuildContext(contextPath, dockerfilePath string) (io.ReadCloser, error) {
	// Create tar buffer
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	// Load .dockerignore patterns if file exists
	dockerignorePath := filepath.Join(contextPath, ".dockerignore")
	ignorePatterns := []string{}
	if _, err := os.Stat(dockerignorePath); err == nil {
		patterns, err := b.loadDockerignore(dockerignorePath)
		if err != nil {
			// Don't fail the build, just warn
			fmt.Printf("Warning: Failed to load .dockerignore: %v\n", err)
		} else {
			ignorePatterns = patterns
		}
	}

	// Directories to skip during build context creation (fallback if no .dockerignore)
	skipDirs := map[string]bool{
		".git":          true,
		"node_modules":  true,
		"target":        true, // Java/Maven
		"build":         true, // Common build output
		"dist":          true, // Distribution files
		"vendor":        true, // Go/PHP dependencies
		".next":         true, // Next.js build
		".nuxt":         true, // Nuxt.js build
		"venv":          true, // Python virtual env
		".venv":         true, // Python virtual env
		"__pycache__":   true, // Python cache
		".pytest_cache": true, // Pytest cache
		"coverage":      true, // Test coverage
		".tox":          true, // Python tox
		"tmp":           true, // Temporary files
		"temp":          true, // Temporary files
		"logs":          true, // Log files
		".cache":        true, // Cache directories
	}

	// Walk through project directory
	err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(contextPath, path)
		if err != nil {
			return err
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Check against .dockerignore patterns if they exist
		if len(ignorePatterns) > 0 && b.shouldIgnore(relPath, ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Fallback: Skip common build/dependency directories if no .dockerignore
		if len(ignorePatterns) == 0 && info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}

		// Check path length and provide helpful error
		if len(relPath) > 255 {
			return fmt.Errorf("path too long (>255 chars): %s\nConsider using .dockerignore to exclude this file/directory", relPath)
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Use PAX format for better compatibility with long paths
		header.Format = tar.FormatPAX

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			// Provide more context on tar errors
			if strings.Contains(err.Error(), "too long") {
				return fmt.Errorf("tar path too long: %s\nPath: %s\nTip: Add this directory to .dockerignore", err.Error(), relPath)
			}
			return fmt.Errorf("failed to write tar header for %s: %w", relPath, err)
		}

		// Write file content if not a directory
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Always ensure Dockerfile is in the tar, even if:
	// 1. It's outside the context, OR
	// 2. It was excluded by .dockerignore
	//
	// We need to check if the Dockerfile is in the tar and add it if not
	relDockerfile, err := filepath.Rel(contextPath, dockerfilePath)
	if err != nil {
		relDockerfile = filepath.Base(dockerfilePath)
	}

	// Read and add the Dockerfile explicitly
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	// Add Dockerfile to tar with its relative path
	header := &tar.Header{
		Name: relDockerfile,
		Mode: 0644,
		Size: int64(len(dockerfileContent)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("failed to write Dockerfile header: %w", err)
	}
	if _, err := tw.Write(dockerfileContent); err != nil {
		return nil, fmt.Errorf("failed to write Dockerfile content: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// parseBuildOutput parses Docker build output and displays progress
func (b *Builder) parseBuildOutput(reader io.Reader) (string, error) {
	decoder := json.NewDecoder(reader)
	var imageID string

	cyan := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	fmt.Println()
	cyan.Println("→ Building Docker image...")
	fmt.Println()

	for {
		var msg buildMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to parse build output: %w", err)
		}

		// Handle error messages
		if msg.Error != "" {
			red.Printf("✗ Build failed: %s\n", msg.Error)
			return "", fmt.Errorf("build error: %s", msg.Error)
		}

		// Display stream output
		if msg.Stream != "" {
			stream := strings.TrimSpace(msg.Stream)
			if stream != "" {
				// Highlight important messages
				if strings.HasPrefix(stream, "Step ") {
					cyan.Printf("  %s\n", stream)
				} else if strings.Contains(stream, "Successfully built") {
					// Extract image ID
					parts := strings.Fields(stream)
					if len(parts) >= 3 {
						imageID = parts[2]
					}
					green.Printf("  ✓ %s\n", stream)
				} else if strings.Contains(stream, "Successfully tagged") {
					green.Printf("  ✓ %s\n", stream)
				} else {
					// Regular output
					fmt.Printf("  %s\n", stream)
				}
			}
		}
	}

	if imageID == "" {
		// Try to get image ID from tags
		return "built", nil
	}

	fmt.Println()
	green.Printf("✓ Build completed successfully\n")
	fmt.Println()

	return imageID, nil
}

// TagImage adds a tag to an image
func (b *Builder) TagImage(imageID, tag string) error {
	if err := b.docker.ImageTag(imageID, tag); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	return nil
}

// GetImageInfo retrieves information about a Docker image
func (b *Builder) GetImageInfo(imageTag string) (types.ImageInspect, error) {
	inspect, _, err := b.docker.ImageInspectWithRaw(imageTag)
	if err != nil {
		return types.ImageInspect{}, fmt.Errorf("failed to inspect image: %w", err)
	}

	return inspect, nil
}

// loadDockerignore loads patterns from a .dockerignore file
func (b *Builder) loadDockerignore(dockerignorePath string) ([]string, error) {
	file, err := os.Open(dockerignorePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

// shouldIgnore checks if a path should be ignored based on .dockerignore patterns
func (b *Builder) shouldIgnore(relPath string, patterns []string) bool {
	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		// Normalize pattern
		pattern = filepath.ToSlash(pattern)

		// Simple pattern matching (supports * and ** wildcards)
		matched, err := filepath.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}

		// Check if pattern matches a prefix (directory)
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath, pattern) {
				return true
			}
		}

		// Check if it's a directory match
		if strings.HasPrefix(relPath, pattern+"/") {
			return true
		}

		// Exact match
		if relPath == pattern {
			return true
		}
	}

	return false
}
