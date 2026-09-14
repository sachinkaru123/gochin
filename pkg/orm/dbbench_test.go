package orm

import (
	"context"
	"testing"

	"github.com/sachinkaru123/gochin/pkg/database"
)

type realUser struct {
	Model
	Name         string `db:"name"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
}

func (realUser) TableName() string { return "users" }

// BenchmarkDBRoundTrip is the reference number every other benchmark should
// be judged against.
func BenchmarkDBRoundTrip(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skipf("no database: %v", err)
	}
	db, _ := database.GetConnection()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var n int
		_ = db.QueryRowContext(ctx, "SELECT 1").Scan(&n)
	}
}

func BenchmarkORMQueryPage(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skipf("no database: %v", err)
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Query[realUser]().WithContext(ctx).OrderBy("id", "DESC").Limit(25).Get()
	}
}

func BenchmarkORMFind(b *testing.B) {
	if err := database.Ping(); err != nil {
		b.Skipf("no database: %v", err)
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FindCtx[realUser](ctx, 1)
	}
}
