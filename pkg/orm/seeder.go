package orm

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Seeder populates the database with data for development or testing.
//
// Unlike migrations there is no ledger recording what has run, so a seeder
// must be safe to run repeatedly: use ON CONFLICT DO NOTHING or check for
// existing rows before inserting.
type Seeder struct {
	Name string
	// Priority orders seeders that depend on each other; lower runs first.
	Priority int
	Run      func(ctx context.Context) error
}

var (
	seederMu sync.Mutex
	seeders  []Seeder
)

// RegisterSeeder adds a seeder to the registry. Seeder files call this from
// their own init().
func RegisterSeeder(s Seeder) {
	seederMu.Lock()
	defer seederMu.Unlock()
	seeders = append(seeders, s)
}

// RegisteredSeeders returns every seeder, ordered by priority then name.
func RegisteredSeeders() []Seeder {
	seederMu.Lock()
	defer seederMu.Unlock()

	out := make([]Seeder, len(seeders))
	copy(out, seeders)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// RunSeeders executes the registered seeders, or only those named.
func RunSeeders(ctx context.Context, only ...string) ([]string, error) {
	wanted := map[string]bool{}
	for _, name := range only {
		wanted[name] = true
	}

	if len(wanted) > 0 {
		known := map[string]bool{}
		for _, s := range RegisteredSeeders() {
			known[s.Name] = true
		}
		for name := range wanted {
			if !known[name] {
				return nil, fmt.Errorf("orm: no seeder named %q", name)
			}
		}
	}

	var ran []string
	for _, s := range RegisteredSeeders() {
		if len(wanted) > 0 && !wanted[s.Name] {
			continue
		}
		if s.Run == nil {
			continue
		}
		if err := s.Run(ctx); err != nil {
			return ran, fmt.Errorf("orm: seeder %q failed: %w", s.Name, err)
		}
		ran = append(ran, s.Name)
	}
	return ran, nil
}
