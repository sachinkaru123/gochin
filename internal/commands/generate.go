package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeGenerated creates dir if needed and writes content to filename inside
// it, refusing to clobber an existing file unless force is set.
func writeGenerated(dir, filename, content string, force bool) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", dir, err)
	}

	path := filepath.Join(dir, filename)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", path, err)
	}
	return path, nil
}

// pascalCase upper-cases the first letter of name, leaving the rest as-is.
func pascalCase(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// camelCase lower-cases the first letter, for local variable names.
func camelCase(name string) string {
	if name == "" {
		return name
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// snakeCase converts a Go identifier like "UserProfile" to "user_profile".
func snakeCase(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
				if prev != '_' && (prev < 'A' || prev > 'Z' || nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// kebabCase converts "UserProfile" to "user-profiles"-style URL segments.
func kebabCase(name string) string {
	return strings.ReplaceAll(snakeCase(name), "_", "-")
}

// pluralize is a deliberately naive English pluralizer: the generated table
// and route names are meant to be reviewed and edited, not trusted blindly.
func pluralize(name string) string {
	switch {
	case name == "":
		return name
	case strings.HasSuffix(name, "y") && !hasVowelBefore(name, 'y'):
		return name[:len(name)-1] + "ies"
	case strings.HasSuffix(name, "s"), strings.HasSuffix(name, "x"),
		strings.HasSuffix(name, "z"), strings.HasSuffix(name, "ch"),
		strings.HasSuffix(name, "sh"):
		return name + "es"
	default:
		return name + "s"
	}
}

func hasVowelBefore(s string, c byte) bool {
	i := strings.LastIndexByte(s, c)
	if i <= 0 {
		return false
	}
	return strings.ContainsRune("aeiouAEIOU", rune(s[i-1]))
}
