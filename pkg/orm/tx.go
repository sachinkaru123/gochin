package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gochin/framework/pkg/database"
)

// Executor is satisfied by both *sql.DB and *sql.Tx, letting every CRUD
// function and the query builder run either against the default connection
// pool or inside an explicit transaction.
//
// Only the context-aware methods are required: without them a client that
// disconnects leaves its query running in Postgres, holding a pooled
// connection until it completes.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DefaultExecutor returns the framework's shared connection pool, for the
// occasional raw statement the query builder does not cover.
func DefaultExecutor() (Executor, error) {
	return database.GetConnection()
}

// resolveExecutor picks the caller-supplied Executor if one was given,
// otherwise falls back to the framework's default database connection.
func resolveExecutor(execs []Executor) (Executor, error) {
	if len(execs) > 0 && execs[0] != nil {
		return execs[0], nil
	}
	return database.GetConnection()
}

// WithTransaction runs fn inside a database transaction against the default
// connection, committing on a nil return and rolling back otherwise.
func WithTransaction(fn func(tx *sql.Tx) error) error {
	return WithTransactionCtx(context.Background(), func(_ context.Context, tx *sql.Tx) error {
		return fn(tx)
	})
}

// WithTransactionCtx runs fn inside a transaction bound to ctx.
func WithTransactionCtx(ctx context.Context, fn func(context.Context, *sql.Tx) error) (err error) {
	db, connErr := database.GetConnection()
	if connErr != nil {
		return connErr
	}

	tx, beginErr := db.BeginTx(ctx, nil)
	if beginErr != nil {
		return beginErr
	}

	// Without this a panic inside fn would leave the transaction open until
	// the connection is reaped, holding its locks the whole time.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("orm: rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}
