package orm

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// fieldInfo describes how a single struct field maps to a database column.
type fieldInfo struct {
	Column       string
	Index        []int
	IsPrimaryKey bool
}

// typeSchema is the cached reflection metadata for a model type.
type typeSchema struct {
	Fields        []fieldInfo
	ByColumn      map[string]fieldInfo
	PrimaryKey    fieldInfo
	HasPrimaryKey bool
}

var (
	schemaCache   = map[reflect.Type]*typeSchema{}
	schemaCacheMu sync.RWMutex

	validIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// schemaFor returns the cached typeSchema for T, building it on first use.
func schemaFor[T any]() *typeSchema {
	var zero T
	t := reflect.TypeOf(zero)

	schemaCacheMu.RLock()
	s, ok := schemaCache[t]
	schemaCacheMu.RUnlock()
	if ok {
		return s
	}

	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()

	// Another goroutine may have built it while we waited for the write lock.
	if s, ok := schemaCache[t]; ok {
		return s
	}

	s = buildSchema(t)
	schemaCache[t] = s
	return s
}

func buildSchema(t reflect.Type) *typeSchema {
	s := &typeSchema{
		ByColumn: map[string]fieldInfo{},
	}

	for _, sf := range reflect.VisibleFields(t) {
		if !sf.IsExported() {
			continue
		}

		// Anonymous struct fields (embedded types like orm.Model) show up
		// both as their own entry and as separately visible promoted
		// fields; skip the embedding itself and keep only the leaves.
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct {
			continue
		}

		tag, hasTag := sf.Tag.Lookup("db")
		if hasTag && tag == "-" {
			continue
		}

		column := snakeCase(sf.Name)
		isPK := false

		if hasTag {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				column = parts[0]
			}
			for _, opt := range parts[1:] {
				if opt == "primary_key" {
					isPK = true
				}
			}
		}

		fi := fieldInfo{
			Column:       column,
			Index:        sf.Index,
			IsPrimaryKey: isPK,
		}

		s.Fields = append(s.Fields, fi)
		s.ByColumn[column] = fi

		if isPK {
			s.PrimaryKey = fi
			s.HasPrimaryKey = true
		}
	}

	return s
}

// IsValidColumn reports whether column is a known column for T.
func (s *typeSchema) IsValidColumn(column string) bool {
	_, ok := s.ByColumn[column]
	return ok
}

// columns returns every mapped column name, in struct declaration order.
func (s *typeSchema) columns() []string {
	cols := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		cols[i] = f.Column
	}
	return cols
}

// snakeCase converts a Go identifier like "CreatedAt" to "created_at".
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

// validateIdentifier ensures a column/direction name is safe to place
// directly into a SQL string (it is never taken from unchecked user input,
// but this guards against typos and defends in depth).
func validateIdentifier(name string) error {
	if !validIdentifier.MatchString(name) {
		return fmt.Errorf("orm: invalid identifier %q", name)
	}
	return nil
}
