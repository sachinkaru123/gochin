package orm

import (
	"context"
	"testing"

	"github.com/sachinkaru123/gochin/pkg/database"
)

// These isolate driver protocol overhead from query execution.
func BenchmarkDriverUnprepared(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skip("no database")
	}
	db, _ := database.GetConnection()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var n int
		_ = db.QueryRowContext(ctx, "SELECT $1::int", 1).Scan(&n)
	}
}

func BenchmarkDriverPrepared(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skip("no database")
	}
	db, _ := database.GetConnection()
	ctx := context.Background()

	stmt, err := db.PrepareContext(ctx, "SELECT $1::int")
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var n int
		_ = stmt.QueryRowContext(ctx, 1).Scan(&n)
	}
}

// A real table query, prepared vs not.
func BenchmarkTableUnprepared(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skip("no database")
	}
	db, _ := database.GetConnection()
	ctx := context.Background()
	const q = `SELECT id, name, email FROM users WHERE id = $1`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var id int64
		var name, email string
		_ = db.QueryRowContext(ctx, q, 1).Scan(&id, &name, &email)
	}
}

func BenchmarkTablePrepared(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skip("no database")
	}
	db, _ := database.GetConnection()
	ctx := context.Background()

	stmt, err := db.PrepareContext(ctx, `SELECT id, name, email FROM users WHERE id = $1`)
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var id int64
		var name, email string
		_ = stmt.QueryRowContext(ctx, 1).Scan(&id, &name, &email)
	}
}
