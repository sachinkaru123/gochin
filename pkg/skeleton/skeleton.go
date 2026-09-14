// Package skeleton embeds the project template that `gochin new` renders: a
// working authentication app (users + auth_tokens migrations, a User model,
// AuthController/AuthService, and auth routes) wired against the framework.
package skeleton

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed all:templates
var templatesFS embed.FS

const templatesRoot = "templates"

// FrameworkVersion is the framework release a generated project requires by
// default.
const FrameworkVersion = "v0.1.0"

// Data is the substitution set available to every .tmpl file.
type Data struct {
	// Module is the full import path of the generated project, e.g.
	// "github.com/alice/myapp" or just "myapp" for a standalone module.
	Module string
	// Name is Module's last path segment, used for display strings and as
	// the default database name.
	Name string
	// FrameworkVersion is the gochin release the generated go.mod requires.
	FrameworkVersion string
}

// Generate renders the embedded skeleton into dir for the given module path.
// dir must not already contain files.
func Generate(dir, module string) error {
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("skeleton: %s is not empty", dir)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	data := Data{
		Module:           module,
		Name:             lastSegment(module),
		FrameworkVersion: FrameworkVersion,
	}

	return fs.WalkDir(templatesFS, templatesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(templatesRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dir, 0o755)
		}

		destPath := filepath.Join(dir, mapTargetPath(rel))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		content, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}

		if strings.HasSuffix(rel, ".tmpl") {
			content, err = render(rel, content, data)
			if err != nil {
				return err
			}
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(destPath, content, 0o644)
	})
}

func render(name string, content []byte, data Data) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("skeleton: parsing %s: %w", name, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("skeleton: rendering %s: %w", name, err)
	}
	return []byte(buf.String()), nil
}

// mapTargetPath strips the .tmpl suffix and restores the dotfile names that
// can't live as literal filenames inside the template source tree.
func mapTargetPath(rel string) string {
	rel = strings.TrimSuffix(rel, ".tmpl")

	dir, base := filepath.Split(rel)
	switch base {
	case "gitignore":
		base = ".gitignore"
	case "env.example":
		base = ".env.example"
	}
	return filepath.Join(dir, base)
}

func lastSegment(module string) string {
	if i := strings.LastIndex(module, "/"); i >= 0 {
		return module[i+1:]
	}
	return module
}
