package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/sachinkaru123/gochin/pkg/skeleton"
)

// NewNewCommand creates the `gochin new` command.
func NewNewCommand() *cobra.Command {
	var module string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new Gochin project",
		Long: "Scaffold a new Gochin project: a working authentication app " +
			"(users + auth_tokens migrations, a User model, register/login/me/logout " +
			"routes) wired against the framework, ready to `go run . run start`.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(args[0], module)
		},
	}

	cmd.Flags().StringVar(&module, "module", "", "Module path for the new project (default: the project name)")
	return cmd
}

var validProjectName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

func runNew(name, module string) error {
	if !validProjectName.MatchString(name) {
		return fmt.Errorf("invalid project name %q: use letters, digits, - and _, starting with a letter", name)
	}
	if module == "" {
		module = name
	}

	dir, err := filepath.Abs(name)
	if err != nil {
		return err
	}

	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty", dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}

	fmt.Printf("Creating Gochin project %q in %s\n", name, dir)

	if err := skeleton.Generate(dir, module); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
	}

	if replace := os.Getenv("GOCHIN_FRAMEWORK_REPLACE"); replace != "" {
		// Framework contributor workflow: point the generated project at a
		// local checkout instead of a published version.
		absReplace, err := filepath.Abs(replace)
		if err != nil {
			return err
		}
		if err := runIn(dir, "go", "mod", "edit", "-replace",
			"github.com/sachinkaru123/gochin="+absReplace); err != nil {
			return fmt.Errorf("failed to add local replace directive: %w", err)
		}
		fmt.Printf("Using local framework checkout: %s\n", absReplace)
	}

	fmt.Println("Fetching dependencies (go mod tidy)...")
	if err := runIn(dir, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\n(the project was created; you may need to run this yourself)", err)
	}

	fmt.Printf(`
✅ Project created: %s

Next steps:
  cd %s
  cp .env.example .env      # then edit your database credentials
  go run . db migrate
  go run . db seed
  go run . run start
`, dir, name)

	return nil
}

func runIn(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
