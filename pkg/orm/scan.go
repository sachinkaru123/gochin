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

	// One destination slice for the whole result set: rows.Scan copies
	// through the pointers and does not retain the slice, so refilling it per
	// row saves an allocation on every row.
	dest := make([]any, len(cols))

	var results []*T
	for rows.Next() {
		// Abandon rather than truncate: a silently short result set would
		// produce wrong answers downstream.
		if HardRowCap > 0 && len(results) >= HardRowCap {
			return nil, ErrTooManyRows
		}
		item := new(T)
		if err := fillScanTargets(dest, item, schema, cols); err != nil {
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
	dest := make([]any, len(cols))
	if err := fillScanTargets(dest, item, schema, cols); err != nil {
		return nil, err
	}
	return dest, nil
}

// fillScanTargets writes field addresses into an existing slice, so a caller
// scanning many rows can reuse one allocation.
func fillScanTargets[T any](dest []any, item *T, schema *typeSchema, cols []string) error {
	v := reflect.ValueOf(item).Elem()

	for i, col := range cols {
		fi, ok := schema.ByColumn[col]
		if !ok {
			return fmt.Errorf("orm: column %q has no matching field on %T", col, item)
		}
		dest[i] = v.FieldByIndex(fi.Index).Addr().Interface()
	}

	return nil
}
