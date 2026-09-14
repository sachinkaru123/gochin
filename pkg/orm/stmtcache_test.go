package orm

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/sachinkaru123/gochin/pkg/database"
)

func cachedExecutor(t *testing.T, size int) (*cachingExecutor, *sql.DB) {
	t.Helper()
	requireDB(t)

	db, err := database.GetConnection()
	if err != nil {
		t.Fatalf("connection: %v", err)
	}

	e := &cachingExecutor{db: db, cache: newStmtCache(size)}
	t.Cleanup(e.cache.Close)
	return e, db
}

// A cached statement must return exactly what an uncached one does.
func TestStmtCacheReturnsSameResults(t *testing.T) {
	e, db := cachedExecutor(t, 8)
	ctx := context.Background()
	const q = `SELECT $1::int + $2::int`

	var direct int
	if err := db.QueryRowContext(ctx, q, 2, 3).Scan(&direct); err != nil {
		t.Fatalf("direct query: %v", err)
	}

	// First call prepares, second hits the cache; both must agree.
	for i := 0; i < 2; i++ {
		var cached int
		if err := e.QueryRowContext(ctx, q, 2, 3).Scan(&cached); err != nil {
			t.Fatalf("cached query %d: %v", i, err)
		}
		if cached != direct {
			t.Errorf("cached result %d = %d, want %d", i, cached, direct)
		}
	}

	if e.cache.len() != 1 {
		t.Errorf("cache holds %d statements, want 1", e.cache.len())
	}
}

func TestStmtCacheQueryAndExec(t *testing.T) {
	e, _ := cachedExecutor(t, 8)
	ctx := context.Background()

	rows, err := e.QueryContext(ctx, `SELECT generate_series(1, 3)`)
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	n := 0
	for rows.Next() {
		n++
	}
	rows.Close()
	if n != 3 {
		t.Errorf("got %d rows, want 3", n)
	}

	if _, err := e.ExecContext(ctx, `SELECT $1::int`, 1); err != nil {
		t.Errorf("ExecContext: %v", err)
	}
}

// Once full the cache must stop admitting statements rather than evicting
// one that another goroutine may still be holding.
func TestStmtCacheRespectsBound(t *testing.T) {
	e, _ := cachedExecutor(t, 2)
	ctx := context.Background()

	queries := []string{
		`SELECT 1::int`,
		`SELECT 2::int`,
		`SELECT 3::int`,
		`SELECT 4::int`,
	}

	for _, q := range queries {
		var n int
		if err := e.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
	}

	if got := e.cache.len(); got > 2 {
		t.Errorf("cache holds %d statements, want at most 2", got)
	}
}

// Queries must still succeed when caching is switched off entirely.
func TestStmtCacheDisabled(t *testing.T) {
	e, _ := cachedExecutor(t, 0)
	ctx := context.Background()

	var n int
	if err := e.QueryRowContext(ctx, `SELECT 7::int`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 7 {
		t.Errorf("got %d, want 7", n)
	}
	if e.cache.len() != 0 {
		t.Error("a disabled cache retained statements")
	}
}

// A forgotten statement must be re-prepared rather than failing.
func TestStmtCacheRecoversFromForget(t *testing.T) {
	e, _ := cachedExecutor(t, 8)
	ctx := context.Background()
	const q = `SELECT 11::int`

	var first int
	if err := e.QueryRowContext(ctx, q).Scan(&first); err != nil {
		t.Fatal(err)
	}

	e.cache.forget(q)

	var second int
	if err := e.QueryRowContext(ctx, q).Scan(&second); err != nil {
		t.Fatalf("query after forget: %v", err)
	}
	if second != first {
		t.Errorf("got %d after forget, want %d", second, first)
	}
}

// Many goroutines racing on the same uncached query must end up with one
// statement and no corruption.
func TestStmtCacheConcurrentPrepare(t *testing.T) {
	e, _ := cachedExecutor(t, 16)
	ctx := context.Background()
	const q = `SELECT $1::int`

	var wg sync.WaitGroup
	errs := make(chan error, 40)

	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			var got int
			if err := e.QueryRowContext(ctx, q, n).Scan(&got); err != nil {
				errs <- err
				return
			}
			if got != n {
				errs <- errWrongValue(n, got)
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
	if got := e.cache.len(); got != 1 {
		t.Errorf("cache holds %d statements for one query, want 1", got)
	}
}

// A transaction must not be routed through the pool's statement cache.
func TestTransactionBypassesCache(t *testing.T) {
	requireDB(t)

	exec, err := resolveExecutor(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := exec.(*cachingExecutor); !ok {
		t.Fatalf("default executor is %T, want *cachingExecutor", exec)
	}

	db, _ := database.GetConnection()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	txExec, err := resolveExecutor([]Executor{tx})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := txExec.(*cachingExecutor); ok {
		t.Error("a caller-supplied transaction was wrapped in the statement cache")
	}
}

func errWrongValue(want, got int) error {
	return &valueMismatch{want: want, got: got}
}

type valueMismatch struct{ want, got int }

func (e *valueMismatch) Error() string {
	return "query returned " + itoaSmall(e.got) + ", want " + itoaSmall(e.want)
}

func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
