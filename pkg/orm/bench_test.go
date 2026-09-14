package orm

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type benchUser struct {
	Model
	Name   string    `db:"name"`
	Email  string    `db:"email"`
	Age    int       `db:"age"`
	Active bool      `db:"active"`
	Seen   time.Time `db:"seen_at"`
}

func (benchUser) TableName() string { return "bench_users" }

// BenchmarkSchemaLookup measures the cached reflection metadata lookup, which
// happens on every single ORM call.
func BenchmarkSchemaLookup(b *testing.B) {
	schemaFor[benchUser]() // warm the cache

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = schemaFor[benchUser]()
	}
}

// BenchmarkBuildSelect measures query string construction.
func BenchmarkBuildSelect(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := &QueryBuilder[benchUser]{
			ctx:    context.Background(),
			schema: schemaFor[benchUser](),
			table:  "bench_users",
		}
		q.Where("active", "=", true).
			Where("age", ">", 18).
			OrderBy("id", "DESC").
			Limit(25)
		_, _, _ = q.buildSelect()
	}
}

// BenchmarkScanTargets measures the per-row reflection that turns a struct
// into scan destinations — this runs once per returned row.
func BenchmarkScanTargets(b *testing.B) {
	schema := schemaFor[benchUser]()
	cols := schema.columns()
	item := new(benchUser)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = scanTargets(item, schema, cols)
	}
}

// BenchmarkScanTargets25Rows models scanning a 25-row page the way scanRows
// does it: one destination slice reused across every row.
func BenchmarkScanTargets25Rows(b *testing.B) {
	schema := schemaFor[benchUser]()
	cols := schema.columns()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dest := make([]any, len(cols))
		for row := 0; row < 25; row++ {
			item := new(benchUser)
			_ = fillScanTargets(dest, item, schema, cols)
		}
	}
}

// BenchmarkScanTargets25RowsPerRowAlloc is the old behaviour, kept as the
// comparison point for the slice-reuse change.
func BenchmarkScanTargets25RowsPerRowAlloc(b *testing.B) {
	schema := schemaFor[benchUser]()
	cols := schema.columns()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for row := 0; row < 25; row++ {
			item := new(benchUser)
			_, _ = scanTargets(item, schema, cols)
		}
	}
}

// BenchmarkColumns measures the column-name slice built for every SELECT.
func BenchmarkColumns(b *testing.B) {
	schema := schemaFor[benchUser]()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = schema.columns()
	}
}

// BenchmarkBuildSchema measures the uncached path, for reference.
func BenchmarkBuildSchema(b *testing.B) {
	var zero benchUser
	t := reflect.TypeOf(zero)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildSchema(t)
	}
}

// BenchmarkSchemaLookupParallel checks RWMutex contention on the cache that
// every ORM call reads.
func BenchmarkSchemaLookupParallel(b *testing.B) {
	schemaFor[benchUser]()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = schemaFor[benchUser]()
		}
	})
}
