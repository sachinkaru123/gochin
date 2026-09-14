package orm

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/gochin/framework/pkg/database"
)

type relParent struct {
	Model
	Name string      `db:"name"`
	Kids []*relChild `db:"-"`
}

func (relParent) TableName() string { return "orm_test_parents" }

type relChild struct {
	Model
	ParentID int64  `db:"parent_id"`
	Label    string `db:"label"`
}

func (relChild) TableName() string { return "orm_test_children" }

// countingExecutor wraps a real Executor to count round trips, which is how
// the batching guarantee is actually verified rather than assumed.
type countingExecutor struct {
	inner   Executor
	queries int64
}

func (c *countingExecutor) count() int64 { return atomic.LoadInt64(&c.queries) }

func (c *countingExecutor) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	atomic.AddInt64(&c.queries, 1)
	return c.inner.ExecContext(ctx, q, args...)
}

func (c *countingExecutor) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	atomic.AddInt64(&c.queries, 1)
	return c.inner.QueryContext(ctx, q, args...)
}

func (c *countingExecutor) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	atomic.AddInt64(&c.queries, 1)
	return c.inner.QueryRowContext(ctx, q, args...)
}

func setupRelationFixtures(t *testing.T, parentCount, kidsEach int) (*sql.DB, []*relParent) {
	t.Helper()
	requireDB(t)

	db, err := database.GetConnection()
	if err != nil {
		t.Fatalf("connection: %v", err)
	}
	ctx := context.Background()

	stmts := []string{
		`DROP TABLE IF EXISTS orm_test_children`,
		`DROP TABLE IF EXISTS orm_test_parents`,
		`CREATE TABLE orm_test_parents (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE orm_test_children (
			id BIGSERIAL PRIMARY KEY,
			parent_id BIGINT NOT NULL REFERENCES orm_test_parents(id) ON DELETE CASCADE,
			label TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("fixture setup: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP TABLE IF EXISTS orm_test_children`)
		_, _ = db.ExecContext(context.Background(), `DROP TABLE IF EXISTS orm_test_parents`)
	})

	parents := make([]*relParent, 0, parentCount)
	for i := 0; i < parentCount; i++ {
		p := &relParent{Name: fmt.Sprintf("parent-%d", i)}
		if err := CreateCtx(ctx, p); err != nil {
			t.Fatalf("create parent: %v", err)
		}
		for j := 0; j < kidsEach; j++ {
			c := &relChild{ParentID: p.ID, Label: fmt.Sprintf("kid-%d-%d", i, j)}
			if err := CreateCtx(ctx, c); err != nil {
				t.Fatalf("create child: %v", err)
			}
		}
		parents = append(parents, p)
	}
	return db, parents
}

// The whole point of the loaders: one query, no matter how many parents.
func TestLoadHasManyIssuesExactlyOneQuery(t *testing.T) {
	db, parents := setupRelationFixtures(t, 25, 3)

	counter := &countingExecutor{inner: db}
	err := LoadHasMany(context.Background(), parents, "parent_id",
		func(p *relParent) int64 { return p.ID },
		func(p *relParent, kids []*relChild) { p.Kids = kids },
		counter,
	)
	if err != nil {
		t.Fatalf("LoadHasMany: %v", err)
	}

	if got := counter.count(); got != 1 {
		t.Errorf("issued %d queries for 25 parents, want exactly 1 (N+1 regression)", got)
	}

	for _, p := range parents {
		if len(p.Kids) != 3 {
			t.Fatalf("parent %d got %d children, want 3", p.ID, len(p.Kids))
		}
		for _, kid := range p.Kids {
			if kid.ParentID != p.ID {
				t.Errorf("child %d attached to the wrong parent %d", kid.ID, p.ID)
			}
		}
	}
}

func TestLoadBelongsToIssuesExactlyOneQuery(t *testing.T) {
	db, parents := setupRelationFixtures(t, 10, 2)

	children, err := Query[relChild]().OrderBy("id", "ASC").Get()
	if err != nil {
		t.Fatalf("load children: %v", err)
	}
	if len(children) != 20 {
		t.Fatalf("got %d children, want 20", len(children))
	}

	type childWithParent struct {
		*relChild
		Parent *relParent
	}
	wrapped := make([]*childWithParent, len(children))
	for i, c := range children {
		wrapped[i] = &childWithParent{relChild: c}
	}

	counter := &countingExecutor{inner: db}
	err = LoadBelongsTo(context.Background(), wrapped,
		func(c *childWithParent) int64 { return c.ParentID },
		func(c *childWithParent, p *relParent) { c.Parent = p },
		counter,
	)
	if err != nil {
		t.Fatalf("LoadBelongsTo: %v", err)
	}

	if got := counter.count(); got != 1 {
		t.Errorf("issued %d queries for 20 children, want exactly 1", got)
	}

	byID := make(map[int64]*relParent, len(parents))
	for _, p := range parents {
		byID[p.ID] = p
	}
	for _, c := range wrapped {
		if c.Parent == nil {
			t.Fatalf("child %d has no parent attached", c.ID)
		}
		if c.Parent.ID != c.ParentID {
			t.Errorf("child %d got parent %d, want %d", c.ID, c.Parent.ID, c.ParentID)
		}
	}
}

func TestLoadHasManyHandlesEmptyAndMissing(t *testing.T) {
	db, _ := setupRelationFixtures(t, 0, 0)

	counter := &countingExecutor{inner: db}
	if err := LoadHasMany(context.Background(), []*relParent{}, "parent_id",
		func(p *relParent) int64 { return p.ID },
		func(p *relParent, kids []*relChild) { p.Kids = kids },
		counter,
	); err != nil {
		t.Fatalf("empty parents: %v", err)
	}
	if counter.count() != 0 {
		t.Errorf("issued %d queries for zero parents, want 0", counter.count())
	}

	// A parent with no children must end up with an empty slice, not an error.
	orphan := &relParent{Name: "childless"}
	if err := CreateCtx(context.Background(), orphan); err != nil {
		t.Fatalf("create: %v", err)
	}
	parents := []*relParent{orphan}
	if err := LoadHasMany(context.Background(), parents, "parent_id",
		func(p *relParent) int64 { return p.ID },
		func(p *relParent, kids []*relChild) { p.Kids = kids },
	); err != nil {
		t.Fatalf("childless parent: %v", err)
	}
	if len(parents[0].Kids) != 0 {
		t.Errorf("childless parent got %d children", len(parents[0].Kids))
	}
}

func TestLoadHasManyRejectsUnknownForeignKey(t *testing.T) {
	_, parents := setupRelationFixtures(t, 1, 1)

	err := LoadHasMany(context.Background(), parents, "not_a_column",
		func(p *relParent) int64 { return p.ID },
		func(p *relParent, kids []*relChild) { p.Kids = kids },
	)
	if err == nil {
		t.Error("expected an error for an unknown foreign key column")
	}
}
