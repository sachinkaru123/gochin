package orm

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

var validOps = map[string]bool{
	"=": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true,
	"LIKE": true, "IN": true,
}

var validDirections = map[string]bool{"ASC": true, "DESC": true}

type condition struct {
	column string
	op     string
	value  any // single value, or []any for op == "IN"
}

type orderClause struct {
	column    string
	direction string
}

// QueryBuilder builds and executes a parameterized SELECT for model type T.
type QueryBuilder[T Tabler] struct {
	exec       Executor
	ctx        context.Context
	schema     *typeSchema
	table      string
	columns    []string
	conditions []condition
	order      []orderClause
	limitVal   *int
	offsetVal  *int
	err        error
}

// Query starts a new QueryBuilder for T, optionally scoped to an Executor
// (e.g. a *sql.Tx) instead of the default connection.
func Query[T Tabler](execs ...Executor) *QueryBuilder[T] {
	exec, err := resolveExecutor(execs)
	var zero T
	return &QueryBuilder[T]{
		exec:   exec,
		ctx:    context.Background(),
		schema: schemaFor[T](),
		table:  zero.TableName(),
		err:    err,
	}
}

// WithContext binds the query to ctx, so that a cancelled request cancels the
// running Postgres query instead of leaving it to finish unobserved.
func (q *QueryBuilder[T]) WithContext(ctx context.Context) *QueryBuilder[T] {
	if ctx != nil {
		q.ctx = ctx
	}
	return q
}

// Select restricts which columns are fetched. Defaults to all mapped columns.
func (q *QueryBuilder[T]) Select(columns ...string) *QueryBuilder[T] {
	if q.err != nil {
		return q
	}
	for _, c := range columns {
		if !q.schema.IsValidColumn(c) {
			q.err = fmt.Errorf("orm: unknown column %q", c)
			return q
		}
	}
	q.columns = columns
	return q
}

// Where adds a parameterized "column op value" condition, ANDed with any
// existing conditions. op must be one of =, !=, <, <=, >, >=, LIKE.
func (q *QueryBuilder[T]) Where(column, op string, value any) *QueryBuilder[T] {
	if q.err != nil {
		return q
	}
	if !q.schema.IsValidColumn(column) {
		q.err = fmt.Errorf("orm: unknown column %q", column)
		return q
	}
	if !validOps[op] || op == "IN" {
		q.err = fmt.Errorf("orm: unsupported operator %q", op)
		return q
	}
	q.conditions = append(q.conditions, condition{column: column, op: op, value: value})
	return q
}

// WhereIn adds a "column IN (...)" condition, ANDed with any existing ones.
func (q *QueryBuilder[T]) WhereIn(column string, values []any) *QueryBuilder[T] {
	if q.err != nil {
		return q
	}
	if !q.schema.IsValidColumn(column) {
		q.err = fmt.Errorf("orm: unknown column %q", column)
		return q
	}
	if len(values) == 0 {
		q.err = fmt.Errorf("orm: WhereIn requires at least one value")
		return q
	}
	q.conditions = append(q.conditions, condition{column: column, op: "IN", value: values})
	return q
}

// OrderBy appends an ORDER BY clause. direction must be ASC or DESC.
func (q *QueryBuilder[T]) OrderBy(column, direction string) *QueryBuilder[T] {
	if q.err != nil {
		return q
	}
	direction = strings.ToUpper(direction)
	if !q.schema.IsValidColumn(column) {
		q.err = fmt.Errorf("orm: unknown column %q", column)
		return q
	}
	if !validDirections[direction] {
		q.err = fmt.Errorf("orm: invalid order direction %q", direction)
		return q
	}
	q.order = append(q.order, orderClause{column: column, direction: direction})
	return q
}

// Limit caps the number of rows returned.
func (q *QueryBuilder[T]) Limit(n int) *QueryBuilder[T] {
	q.limitVal = &n
	return q
}

// Offset skips the first n matching rows.
func (q *QueryBuilder[T]) Offset(n int) *QueryBuilder[T] {
	q.offsetVal = &n
	return q
}

// buildSelect renders the SELECT statement and its argument list.
func (q *QueryBuilder[T]) buildSelect() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}

	// Written with a Builder rather than Fprintf/Sprintf: the formatted
	// variants allocated on every clause and dominated query construction.
	var b strings.Builder
	b.Grow(estimateQuerySize(q))

	b.WriteString("SELECT ")
	if len(q.columns) == 0 {
		b.WriteString(q.schema.SelectList) // precomputed at schema build
	} else {
		for i, c := range q.columns {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(c)
		}
	}
	b.WriteString(" FROM ")
	b.WriteString(q.table)

	var args []any

	if len(q.conditions) > 0 {
		b.WriteString(" WHERE ")
		for i, c := range q.conditions {
			if i > 0 {
				b.WriteString(" AND ")
			}

			if c.op == "IN" {
				values := c.value.([]any)
				b.WriteString(c.column)
				b.WriteString(" IN (")
				for j, v := range values {
					if j > 0 {
						b.WriteString(", ")
					}
					args = append(args, v)
					b.WriteString(placeholder(len(args)))
				}
				b.WriteString(")")
				continue
			}

			args = append(args, c.value)
			b.WriteString(c.column)
			b.WriteByte(' ')
			b.WriteString(c.op)
			b.WriteByte(' ')
			b.WriteString(placeholder(len(args)))
		}
	}

	if len(q.order) > 0 {
		b.WriteString(" ORDER BY ")
		for i, o := range q.order {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(o.column)
			b.WriteByte(' ')
			b.WriteString(o.direction)
		}
	}

	if q.limitVal != nil {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.Itoa(*q.limitVal))
	}
	if q.offsetVal != nil {
		b.WriteString(" OFFSET ")
		b.WriteString(strconv.Itoa(*q.offsetVal))
	}

	return b.String(), args, nil
}

// placeholders holds pre-rendered "$1".."$64", which covers essentially every
// real query and keeps strconv off the hot path.
var placeholders = func() [65]string {
	var p [65]string
	for i := 1; i <= 64; i++ {
		p[i] = "$" + strconv.Itoa(i)
	}
	return p
}()

func placeholder(n int) string {
	if n >= 1 && n <= 64 {
		return placeholders[n]
	}
	return "$" + strconv.Itoa(n)
}

// estimateQuerySize pre-sizes the builder so it does not regrow mid-build.
func estimateQuerySize[T Tabler](q *QueryBuilder[T]) int {
	size := len(q.schema.SelectList) + len(q.table) + 16
	size += len(q.conditions) * 24
	size += len(q.order) * 16
	return size
}

// Get executes the query and returns every matching row.
func (q *QueryBuilder[T]) Get() ([]*T, error) {
	query, args, err := q.buildSelect()
	if err != nil {
		return nil, err
	}

	rows, err := q.exec.QueryContext(q.ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return scanRows[T](rows, q.schema)
}

// First executes the query with an implicit LIMIT 1 and returns the single
// match, or ErrRecordNotFound if there isn't one.
func (q *QueryBuilder[T]) First() (*T, error) {
	one := 1
	q.limitVal = &one

	results, err := q.Get()
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrRecordNotFound
	}
	return results[0], nil
}

// Count returns the number of rows matching the query's conditions.
func (q *QueryBuilder[T]) Count() (int64, error) {
	if q.err != nil {
		return 0, q.err
	}

	// ORDER BY/LIMIT/OFFSET are meaningless for a count, and an ORDER BY on a
	// non-aggregated column makes Postgres reject the query outright.
	savedColumns, savedOrder := q.columns, q.order
	savedLimit, savedOffset := q.limitVal, q.offsetVal
	q.columns = []string{"COUNT(*)"}
	q.order, q.limitVal, q.offsetVal = nil, nil, nil

	query, args, err := q.buildSelect()

	q.columns, q.order = savedColumns, savedOrder
	q.limitVal, q.offsetVal = savedLimit, savedOffset
	if err != nil {
		return 0, err
	}

	var count int64
	if err := q.exec.QueryRowContext(q.ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Paginate runs the query for one page of results plus the total row count.
//
// page and perPage are clamped, so a client asking for per_page=1000000
// cannot turn one request into a full table scan.
func (q *QueryBuilder[T]) Paginate(page, perPage int) (*Page[T], error) {
	if q.err != nil {
		return nil, q.err
	}

	page, perPage = ClampPagination(page, perPage)

	total, err := q.Count()
	if err != nil {
		return nil, err
	}

	q.limitVal = &perPage
	offset := (page - 1) * perPage
	q.offsetVal = &offset

	data, err := q.Get()
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	// Serialize an empty page as [] rather than null: clients iterate this.
	if data == nil {
		data = []*T{}
	}

	return &Page[T]{
		Data:       data,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}
