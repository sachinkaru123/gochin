package cli

import "github.com/spf13/cobra"

// Command represents a CLI command interface that all commands must implement
type Command interface {
	// GetCobraCommand returns the cobra command instance
	GetCobraCommand() *cobra.Command

	// Execute runs the command with the given arguments
	Execute(args []string) error

	// Validate validates the command arguments and flags
	Validate() error
}

// BaseCommand provides common functionality for all commands
type BaseCommand struct {
	cmd *cobra.Command
}

// NewBaseCommand creates a new base command
func NewBaseCommand(use, short, long string) *BaseCommand {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
	}

	return &BaseCommand{
		cmd: cmd,
	}
}

// GetCobraCommand returns the cobra command instance
func (b *BaseCommand) GetCobraCommand() *cobra.Command {
	return b.cmd
}

// Validate provides default validation (can be overridden)
func (b *BaseCommand) Validate() error {
	return nil
}
