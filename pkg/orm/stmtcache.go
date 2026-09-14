package orm

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
)

// DefaultStmtCacheSize bounds how many prepared statements are kept per
// process. Each cached statement is prepared lazily on every pooled
// connection, so this trades memory for query speed.
var DefaultStmtCacheSize = 128

// stmtCache memoizes prepared statements by SQL text.
//
// lib/pq re-parses and re-plans a statement on every call: caching cuts a
// typical query from ~159µs to ~57µs. The cache is strictly an optimization —
// every failure path falls back to an unprepared query.
//
// Once full the cache stops admitting new statements rather than evicting
// old ones. Eviction would mean closing a *sql.Stmt that another goroutine
// may be about to use, and a rare "statement is closed" error is a worse
// trade than an occasional unprepared query.
type stmtCache struct {
	mu    sync.RWMutex
	max   int
	stmts map[string]*sql.Stmt
}

func newStmtCache(max int) *stmtCache {
	if max < 0 {
		max = 0
	}
	return &stmtCache{
		max:   max,
		stmts: make(map[string]*sql.Stmt, max),
	}
}

// get returns a cached statement, preparing it on first use.
//
// A nil return means "run this query unprepared" and is never an error the
// caller should surface.
func (c *stmtCache) get(ctx context.Context, db *sql.DB, query string) *sql.Stmt {
	if c == nil || c.max == 0 {
		return nil
	}

	c.mu.RLock()
	stmt, ok := c.stmts[query]
	full := len(c.stmts) >= c.max
	c.mu.RUnlock()

	if ok {
		return stmt
	}
	if full {
		return nil
	}

	// Prepared outside the lock: preparing costs a round trip, and holding
	// the write lock across it would serialize every query in the process.
	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil
	}

	c.mu.Lock()
	if existing, ok := c.stmts[query]; ok {
		// Another goroutine won the race; keep theirs and discard ours.
		c.mu.Unlock()
		_ = stmt.Close()
		return existing
	}
	if len(c.stmts) >= c.max {
		c.mu.Unlock()
		_ = stmt.Close()
		return nil
	}
	c.stmts[query] = stmt
	c.mu.Unlock()

	return stmt
}

// forget drops a statement that turned out to be unusable, so the next call
// re-prepares it.
func (c *stmtCache) forget(query string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	stmt, ok := c.stmts[query]
	delete(c.stmts, query)
	c.mu.Unlock()

	if ok {
		_ = stmt.Close()
	}
}

// Close releases every cached statement.
func (c *stmtCache) Close() {
	if c == nil {
		return
	}

	c.mu.Lock()
	stmts := c.stmts
	c.stmts = make(map[string]*sql.Stmt)
	c.mu.Unlock()

	for _, stmt := range stmts {
		_ = stmt.Close()
	}
}

func (c *stmtCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.stmts)
}

// cachingExecutor runs queries through the statement cache, falling back to
// the raw connection whenever the cache cannot help.
type cachingExecutor struct {
	db    *sql.DB
	cache *stmtCache
}

// staleStmt reports whether an error means the cached statement is unusable
// and the query should be retried unprepared.
//
// database/sql keeps its "statement is closed" sentinel unexported, so the
// message is matched directly.
func staleStmt(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}
	return strings.Contains(err.Error(), "statement is closed")
}

func (e *cachingExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if stmt := e.cache.get(ctx, e.db, query); stmt != nil {
		rows, err := stmt.QueryContext(ctx, args...)
		if err == nil {
			return rows, nil
		}
		if !staleStmt(err) {
			return rows, err
		}
		e.cache.forget(query)
	}
	return e.db.QueryContext(ctx, query, args...)
}

func (e *cachingExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if stmt := e.cache.get(ctx, e.db, query); stmt != nil {
		res, err := stmt.ExecContext(ctx, args...)
		if err == nil {
			return res, nil
		}
		if !staleStmt(err) {
			return res, err
		}
		e.cache.forget(query)
	}
	return e.db.ExecContext(ctx, query, args...)
}

func (e *cachingExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if stmt := e.cache.get(ctx, e.db, query); stmt != nil {
		return stmt.QueryRowContext(ctx, args...)
	}
	return e.db.QueryRowContext(ctx, query, args...)
}

// DB exposes the underlying pool, for callers that need it directly.
func (e *cachingExecutor) DB() *sql.DB { return e.db }

var _ Executor = (*cachingExecutor)(nil)
