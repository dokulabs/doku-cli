package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var envEditCmd = &cobra.Command{
	Use:   "edit <service>",
	Short: "Edit environment variables for a service",
	Long: `Interactively edit environment variables for an installed service.

This command provides an interactive interface to:
  • View all environment variables
  • Add new environment variables
  • Edit existing environment variables
  • Delete environment variables
  • Restart the service to apply changes

Examples:
  doku env edit myapp       # Edit environment variables for myapp
  doku env edit postgres    # Edit environment variables for postgres`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvEdit,
}

func init() {
	envCmd.AddCommand(envEditCmd)
}

func runEnvEdit(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Check if initialized
	if !cfgMgr.IsInitialized() {
		color.Yellow("⚠️  Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance
	instance, err := serviceMgr.Get(serviceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", serviceName)
	}

	// Check if it's a custom project (only custom projects support env editing for now)
	if instance.ServiceType != "custom-project" {
		return fmt.Errorf("environment editing is currently only supported for custom projects")
	}

	// Get project
	projectMgr, err := project.NewManager(dockerClient, cfgMgr)
	if err != nil {
		return fmt.Errorf("failed to create project manager: %w", err)
	}

	proj, err := projectMgr.Get(serviceName)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Clone environment variables for editing
	if proj.Environment == nil {
		proj.Environment = make(map[string]string)
	}

	fmt.Println()
	color.New(color.Bold, color.FgCyan).Printf("Editing Environment Variables for %s\n", serviceName)
	fmt.Println(strings.Repeat("=", len(serviceName)+35))
	fmt.Println()

	// Interactive editing loop
	modified := false
	for {
		action, err := promptEditAction(proj.Environment)
		if err != nil {
			return err
		}

		switch action {
		case "add":
			if err := addEnvVariable(proj.Environment); err != nil {
				color.Red("Error: %v", err)
				fmt.Println()
				continue
			}
			modified = true

		case "edit":
			if len(proj.Environment) == 0 {
				color.Yellow("No environment variables to edit")
				fmt.Println()
				continue
			}
			if err := editEnvVariable(proj.Environment); err != nil {
				color.Red("Error: %v", err)
				fmt.Println()
				continue
			}
			modified = true

		case "delete":
			if len(proj.Environment) == 0 {
				color.Yellow("No environment variables to delete")
				fmt.Println()
				continue
			}
			if err := deleteEnvVariable(proj.Environment); err != nil {
				color.Red("Error: %v", err)
				fmt.Println()
				continue
			}
			modified = true

		case "save":
			if !modified {
				color.Yellow("No changes to save")
				fmt.Println()
				return nil
			}

			// Save changes
			if err := cfgMgr.Update(func(c *types.Config) error {
				if p, exists := c.Projects[serviceName]; exists {
					p.Environment = proj.Environment
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to save changes: %w", err)
			}

			color.Green("✓ Environment variables saved")
			fmt.Println()

			// Ask if user wants to recreate the service to apply changes
			color.Yellow("⚠️  Environment variables require container recreation to take effect")
			fmt.Println()
			recreate := false
			prompt := &survey.Confirm{
				Message: "Recreate the container to apply changes? (stop, remove, rebuild, start)",
				Default: true,
			}
			if err := survey.AskOne(prompt, &recreate); err != nil {
				return err
			}

			if recreate {
				fmt.Println()
				color.Cyan("Recreating container to apply environment changes...")
				fmt.Println()

				// Simply run the project again (which will remove old container and create new one)
				runOpts := project.RunOptions{
					Name:   serviceName,
					Build:  false, // Don't rebuild image
					Detach: true,
				}
				if err := projectMgr.Run(runOpts); err != nil {
					return fmt.Errorf("failed to recreate container: %w", err)
				}

				fmt.Println()
				color.Green("✓ Container recreated successfully with new environment variables")
				fmt.Println()
			} else {
				fmt.Println()
				color.Yellow("⚠️  Changes saved but not applied.")
				color.Yellow("    To apply: doku stop %s && doku start %s won't work", serviceName, serviceName)
				color.Yellow("    You need to: doku remove %s && doku install %s --path=...", serviceName, serviceName)
				fmt.Println()
			}

			return nil

		case "cancel":
			if modified {
				discard := false
				prompt := &survey.Confirm{
					Message: "You have unsaved changes. Discard them?",
					Default: false,
				}
				if err := survey.AskOne(prompt, &discard); err != nil {
					return err
				}
				if !discard {
					continue
				}
			}
			color.Yellow("Cancelled")
			return nil
		}
	}
}

func promptEditAction(env map[string]string) (string, error) {
	// Display current variables
	fmt.Println()
	displayCurrentEnv(env)
	fmt.Println()

	options := []string{
		"Add new variable",
		"Edit existing variable",
		"Delete variable",
		"Save and exit",
		"Cancel (discard changes)",
	}

	var action string
	prompt := &survey.Select{
		Message: "What would you like to do?",
		Options: options,
	}

	if err := survey.AskOne(prompt, &action); err != nil {
		return "", err
	}

	switch action {
	case "Add new variable":
		return "add", nil
	case "Edit existing variable":
		return "edit", nil
	case "Delete variable":
		return "delete", nil
	case "Save and exit":
		return "save", nil
	case "Cancel (discard changes)":
		return "cancel", nil
	}

	return "", fmt.Errorf("unknown action")
}

func displayCurrentEnv(env map[string]string) {
	if len(env) == 0 {
		color.New(color.Faint).Println("  (no environment variables)")
		return
	}

	color.New(color.Bold).Println("Current Variables:")
	fmt.Println()

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := env[key]
		if isSensitiveKey(key) {
			fmt.Printf("  %s = %s %s\n",
				color.YellowString(key),
				maskValue(value),
				color.New(color.Faint).Sprint("🔐"))
		} else {
			fmt.Printf("  %s = %s\n", color.CyanString(key), value)
		}
	}
}

func addEnvVariable(env map[string]string) error {
	fmt.Println()

	var key string
	keyPrompt := &survey.Input{
		Message: "Variable name:",
	}
	if err := survey.AskOne(keyPrompt, &key, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	key = strings.TrimSpace(key)

	// Check if already exists
	if _, exists := env[key]; exists {
		return fmt.Errorf("variable '%s' already exists. Use 'Edit' to modify it", key)
	}

	var value string
	valuePrompt := &survey.Input{
		Message: "Variable value:",
	}
	if err := survey.AskOne(valuePrompt, &value); err != nil {
		return err
	}

	env[key] = value
	color.Green("✓ Added %s", key)
	fmt.Println()

	return nil
}

func editEnvVariable(env map[string]string) error {
	fmt.Println()

	// Get list of keys
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var selectedKey string
	selectPrompt := &survey.Select{
		Message: "Select variable to edit:",
		Options: keys,
	}
	if err := survey.AskOne(selectPrompt, &selectedKey); err != nil {
		return err
	}

	currentValue := env[selectedKey]
	var newValue string
	valuePrompt := &survey.Input{
		Message: fmt.Sprintf("New value for %s:", selectedKey),
		Default: currentValue,
	}
	if err := survey.AskOne(valuePrompt, &newValue); err != nil {
		return err
	}

	if newValue != currentValue {
		env[selectedKey] = newValue
		color.Green("✓ Updated %s", selectedKey)
	} else {
		color.Yellow("No changes made")
	}
	fmt.Println()

	return nil
}

func deleteEnvVariable(env map[string]string) error {
	fmt.Println()

	// Get list of keys
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var selectedKey string
	selectPrompt := &survey.Select{
		Message: "Select variable to delete:",
		Options: keys,
	}
	if err := survey.AskOne(selectPrompt, &selectedKey); err != nil {
		return err
	}

	// Confirm deletion
	confirm := false
	confirmPrompt := &survey.Confirm{
		Message: fmt.Sprintf("Delete %s?", selectedKey),
		Default: false,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}

	if confirm {
		delete(env, selectedKey)
		color.Green("✓ Deleted %s", selectedKey)
	} else {
		color.Yellow("Cancelled")
	}
	fmt.Println()

	return nil
}
