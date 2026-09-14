package orm

import (
	"database/sql"
	"sync"
)

// Migration describes a single reversible schema change, identified by a
// sortable version string (e.g. "000001").
type Migration struct {
	Version string
	Name    string
	Up      func(tx *sql.Tx) error
	Down    func(tx *sql.Tx) error
}

var (
	registryMu sync.Mutex
	registry   []Migration
)

// Register adds a migration to the global registry. Migration files call
// this from their own init() function.
func Register(m Migration) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, m)
}

// registered returns a copy of every migration registered so far.
func registered() []Migration {
	registryMu.Lock()
	defer registryMu.Unlock()
	out := make([]Migration, len(registry))
	copy(out, registry)
	return out
}
