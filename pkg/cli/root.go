package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"

	"github.com/sachinkaru123/gochin/pkg/bootstrap"
	"github.com/sachinkaru123/gochin/pkg/commands"
	"github.com/sachinkaru123/gochin/pkg/config"
	"github.com/sachinkaru123/gochin/pkg/logs"
)

// Build-time variables (can be set with -ldflags)
var (
	Version   = "1.0.0"   // Default version
	BuildDate = "unknown" // Set at build time
	GitCommit = "unknown" // Set at build time
)

var rootCmd = &cobra.Command{
	Use:   "gochin",
	Short: "Gochin is a modern Go API framework",
	Long: `Gochin is a fast and flexible Go-based API framework that provides
	a powerful CLI for rapid application development, including scaffolding,
	server management, and database operations.`,
	Version: Version,
}

// delegatedEnv marks a process as already running as a generated project's
// own binary, so the delegation below does not recurse into itself.
const delegatedEnv = "GOCHIN_DELEGATED"

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	if delegated, err := maybeDelegateToProject(os.Args[1:]); delegated {
		return err
	}

	myFigure := figure.NewFigure("GOCHIN", "", true)
	myFigure.Print()
	fmt.Println("------------------------------------------------------")
	fmt.Println("------------------------------------------------------")
	fmt.Println("")
	fmt.Println("🚀 Welcome to the GoChin Framework")

	// CLI commands log to the same files as the server, so a seeder or
	// migration run leaves a record. A logging failure must not stop the
	// command itself.
	if cfg, err := config.Load(); err == nil {
		if err := bootstrap.ConfigureLogging(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		defer logs.Close()
	}

	return rootCmd.Execute()
}

func init() {
	// Initialize and register all commands
	registerCommands()
}

// maybeDelegateToProject re-execs run/db/route via `go run .` when invoked
// from inside a generated project.
//
// The globally-installed gochin binary is compiled with an empty app/: it
// has no routes, migrations or seeders of its own. A generated project's
// binary does, but only once compiled from that project's own main.go. Since
// Go cannot load another module's code into a running process, the only way
// for "gochin run start" to run *that* project is to build and run it.
func maybeDelegateToProject(args []string) (bool, error) {
	if os.Getenv(delegatedEnv) != "" {
		return false, nil // already running as the project's own binary
	}
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "run", "db", "route":
	default:
		return false, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return false, nil
	}
	if !looksLikeGochinProject(dir) {
		return false, nil
	}

	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	cmd.Env = append(os.Environ(), delegatedEnv+"=1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	return true, runErr
}

// looksLikeGochinProject reports whether dir is a generated project: a
// go.mod requiring the framework, with its own main.go — as opposed to the
// framework's own repository, which has no root-level main.go.
func looksLikeGochinProject(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	if !strings.Contains(string(data), "github.com/sachinkaru123/gochin") {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "main.go"))
	return err == nil
}

// registerCommands registers all available commands with the CLI
func registerCommands() {
	// Register the project scaffolding command
	rootCmd.AddCommand(commands.NewNewCommand())

	// Register run command and its subcommands
	rootCmd.AddCommand(commands.NewRunCommand())

	// Register make command and its subcommands
	rootCmd.AddCommand(commands.NewMakeCommand())

	// Register db command and its subcommands
	rootCmd.AddCommand(commands.NewDbCommand())
	rootCmd.AddCommand(commands.NewRouteCommand())
}
