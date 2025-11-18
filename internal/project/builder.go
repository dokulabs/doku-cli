package project

import (
	"archive/tar"
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

	// Create build context
	buildContext, err := b.createBuildContext(opts.ContextPath, opts.DockerfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create build context: %w", err)
	}
	defer buildContext.Close()

	// Get relative Dockerfile path for Docker build
	relDockerfile, err := filepath.Rel(opts.ContextPath, opts.DockerfilePath)
	if err != nil {
		relDockerfile = "Dockerfile"
	}

	// Prepare build options
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

	// Walk through project directory
	err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip node_modules directory
		if info.IsDir() && info.Name() == "node_modules" {
			return filepath.SkipDir
		}

		// Skip target directory (Java/Maven)
		if info.IsDir() && info.Name() == "target" {
			return filepath.SkipDir
		}

		// Skip build directories
		if info.IsDir() && (info.Name() == "build" || info.Name() == "dist") {
			return filepath.SkipDir
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

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
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

	// Add Dockerfile if it's outside the context
	if !strings.HasPrefix(dockerfilePath, contextPath) {
		dockerfileContent, err := os.ReadFile(dockerfilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read Dockerfile: %w", err)
		}

		header := &tar.Header{
			Name: "Dockerfile",
			Mode: 0644,
			Size: int64(len(dockerfileContent)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(dockerfileContent); err != nil {
			return nil, err
		}
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
