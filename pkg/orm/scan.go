package orm

import (
	"database/sql"
	"fmt"
	"reflect"
)

// scanRows reads every row from rows into a freshly allocated []*T, mapping
// columns to struct fields via the cached schema for T.
func scanRows[T any](rows *sql.Rows, schema *typeSchema) ([]*T, error) {
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []*T
	for rows.Next() {
		// Abandon rather than truncate: a silently short result set would
		// produce wrong answers downstream.
		if HardRowCap > 0 && len(results) >= HardRowCap {
			return nil, ErrTooManyRows
		}
		item := new(T)
		dest, err := scanTargets(item, schema, cols)
		if err != nil {
			return nil, err
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

// scanTargets builds the []any slice of field addresses that rows.Scan
// should write into, one per column in cols, in the order given.
func scanTargets[T any](item *T, schema *typeSchema, cols []string) ([]any, error) {
	v := reflect.ValueOf(item).Elem()

	dest := make([]any, len(cols))
	for i, col := range cols {
		fi, ok := schema.ByColumn[col]
		if !ok {
			return nil, fmt.Errorf("orm: column %q has no matching field on %T", col, item)
		}
		dest[i] = v.FieldByIndex(fi.Index).Addr().Interface()
	}

	return dest, nil
}
