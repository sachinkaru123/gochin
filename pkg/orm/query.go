package orm

import (
	"context"
	"fmt"
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

	cols := q.columns
	if len(cols) == 0 {
		cols = q.schema.columns()
	}

	var b strings.Builder
	var args []any

	fmt.Fprintf(&b, "SELECT %s FROM %s", strings.Join(cols, ", "), q.table)

	if len(q.conditions) > 0 {
		b.WriteString(" WHERE ")
		clauses := make([]string, len(q.conditions))
		for i, c := range q.conditions {
			if c.op == "IN" {
				values := c.value.([]any)
				placeholders := make([]string, len(values))
				for j, v := range values {
					args = append(args, v)
					placeholders[j] = fmt.Sprintf("$%d", len(args))
				}
				clauses[i] = fmt.Sprintf("%s IN (%s)", c.column, strings.Join(placeholders, ", "))
				continue
			}
			args = append(args, c.value)
			clauses[i] = fmt.Sprintf("%s %s $%d", c.column, c.op, len(args))
		}
		b.WriteString(strings.Join(clauses, " AND "))
	}

	if len(q.order) > 0 {
		orders := make([]string, len(q.order))
		for i, o := range q.order {
			orders[i] = fmt.Sprintf("%s %s", o.column, o.direction)
		}
		fmt.Fprintf(&b, " ORDER BY %s", strings.Join(orders, ", "))
	}

	if q.limitVal != nil {
		fmt.Fprintf(&b, " LIMIT %d", *q.limitVal)
	}
	if q.offsetVal != nil {
		fmt.Fprintf(&b, " OFFSET %d", *q.offsetVal)
	}

	return b.String(), args, nil
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
