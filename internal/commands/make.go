package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// NewMakeCommand creates the make command with its subcommands
func NewMakeCommand() *cobra.Command {
	makeCmd := &cobra.Command{
		Use:   "make",
		Short: "Generate application components",
		Long:  "Generate various application components like controllers, models, middleware, etc.",
	}

	// Add subcommands
	makeCmd.AddCommand(newMakeControllerCommand())

	return makeCmd
}

// newMakeControllerCommand creates the "make:controller" subcommand
func newMakeControllerCommand() *cobra.Command {
	controllerCmd := &cobra.Command{
		Use:   "controller [name]",
		Short: "Generate a new controller",
		Long:  "Generate a new controller file with the specified name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateController(args[0])
		},
	}

	return controllerCmd
}

// generateController handles the logic for generating a controller
func generateController(name string) error {
	// Ensure the name ends with "Controller" if not already
	if !strings.HasSuffix(name, "Controller") {
		name += "Controller"
	}

	// Create the file path
	controllerPath := filepath.Join("app", "Controllers", name+".go")

	fmt.Printf("Generating controller: %s\n", name)
	fmt.Printf("File will be created at: %s\n", controllerPath)
	fmt.Println("(Controller template generation will be implemented in later phases)")

	return nil
}