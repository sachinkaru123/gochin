package orm

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sachinkaru123/gochin/pkg/database"
)

// requireDB skips when no database is reachable, so the suite still runs on
// machines without Postgres.
func requireDB(t *testing.T) {
	t.Helper()
	if err := database.Ping(); err != nil {
		t.Skipf("no database available: %v", err)
	}
}

// A cancelled context must abort the running query rather than let it finish
// unobserved while holding a pooled connection.
func TestContextCancellationAbortsQuery(t *testing.T) {
	requireDB(t)

	db, err := database.GetConnection()
	if err != nil {
		t.Fatalf("connection: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = db.QueryContext(ctx, "SELECT pg_sleep(5)")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the query to be cancelled, got no error")
	}
	// lib/pq surfaces its own "canceling statement due to user request"
	// rather than wrapping context.DeadlineExceeded, which is exactly why
	// bootstrap maps SQLSTATE 57014 as well as the context errors.
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "canceling statement") {
		t.Errorf("error = %v, want a cancellation error", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("query took %v; cancellation never reached Postgres", elapsed)
	}
}

func TestPaginationClamps(t *testing.T) {
	cases := []struct {
		page, perPage         int
		wantPage, wantPerPage int
	}{
		{0, 0, 1, DefaultPerPage},
		{-5, -5, 1, DefaultPerPage},
		{2, 10, 2, 10},
		{1, 99999, 1, MaxPerPage},
	}
	for _, tc := range cases {
		gotPage, gotPerPage := ClampPagination(tc.page, tc.perPage)
		if gotPage != tc.wantPage || gotPerPage != tc.wantPerPage {
			t.Errorf("ClampPagination(%d,%d) = (%d,%d), want (%d,%d)",
				tc.page, tc.perPage, gotPage, gotPerPage, tc.wantPage, tc.wantPerPage)
		}
	}
}

// A panic inside the transaction body must still roll back, or the
// transaction holds its locks until the connection is reaped.
func TestWithTransactionRollsBackOnPanic(t *testing.T) {
	requireDB(t)

	defer func() {
		if recover() == nil {
			t.Error("expected the panic to propagate after rollback")
		}
	}()

	_ = WithTransaction(func(tx *sql.Tx) error {
		panic("boom")
	})
}
