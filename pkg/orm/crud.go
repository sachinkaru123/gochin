package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Find fetches the row with the given primary key value into a new *T.
func Find[T Tabler](id any, execs ...Executor) (*T, error) {
	return FindCtx[T](context.Background(), id, execs...)
}

// FindCtx fetches the row with the given primary key, cancelling with ctx.
func FindCtx[T Tabler](ctx context.Context, id any, execs ...Executor) (*T, error) {
	var zero T
	schema := schemaFor[T]()
	if !schema.HasPrimaryKey {
		return nil, fmt.Errorf("orm: %T has no primary key field", zero)
	}

	return Query[T](execs...).WithContext(ctx).
		Where(schema.PrimaryKey.Column, "=", id).First()
}

// All fetches every row for T, subject to the package row cap.
func All[T Tabler](execs ...Executor) ([]*T, error) {
	return AllCtx[T](context.Background(), execs...)
}

// AllCtx fetches every row for T, cancelling with ctx.
func AllCtx[T Tabler](ctx context.Context, execs ...Executor) ([]*T, error) {
	return Query[T](execs...).WithContext(ctx).Get()
}

// Create inserts m, populating its primary key (and CreatedAt/UpdatedAt, if
// present) from the database.
func Create[T Tabler](m *T, execs ...Executor) error {
	return CreateCtx(context.Background(), m, execs...)
}

// CreateCtx inserts m, cancelling with ctx.
func CreateCtx[T Tabler](ctx context.Context, m *T, execs ...Executor) error {
	exec, err := resolveExecutor(execs)
	if err != nil {
		return err
	}

	schema := schemaFor[T]()
	v := reflect.ValueOf(m).Elem()
	now := time.Now().UTC()

	setTimeIfZero(v, schema, "created_at", now)
	setTimeIfZero(v, schema, "updated_at", now)

	var (
		table        = (*m).TableName()
		cols         []string
		placeholders []string
		args         []any
	)

	for _, f := range schema.Fields {
		if f.IsPrimaryKey {
			continue
		}
		cols = append(cols, f.Column)
		args = append(args, v.FieldByIndex(f.Index).Interface())
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}

	// lib/pq has no LastInsertId, so the generated key comes back via RETURNING.
	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		table, strings.Join(cols, ", "), strings.Join(placeholders, ", "), schema.PrimaryKey.Column,
	)

	dest := v.FieldByIndex(schema.PrimaryKey.Index).Addr().Interface()
	return exec.QueryRowContext(ctx, query, args...).Scan(dest)
}

// Update persists every non-primary-key column of m, refreshing UpdatedAt if
// present. Returns ErrRecordNotFound if no row matched the primary key.
func Update[T Tabler](m *T, execs ...Executor) error {
	return UpdateCtx(context.Background(), m, execs...)
}

// UpdateCtx persists m, cancelling with ctx.
func UpdateCtx[T Tabler](ctx context.Context, m *T, execs ...Executor) error {
	exec, err := resolveExecutor(execs)
	if err != nil {
		return err
	}

	schema := schemaFor[T]()
	if !schema.HasPrimaryKey {
		return fmt.Errorf("orm: %T has no primary key field", *m)
	}

	v := reflect.ValueOf(m).Elem()
	setTime(v, schema, "updated_at", time.Now().UTC())

	var (
		table = (*m).TableName()
		sets  []string
		args  []any
	)

	for _, f := range schema.Fields {
		if f.IsPrimaryKey || f.Column == "created_at" {
			continue
		}
		args = append(args, v.FieldByIndex(f.Index).Interface())
		sets = append(sets, fmt.Sprintf("%s = $%d", f.Column, len(args)))
	}

	pkField := v.FieldByIndex(schema.PrimaryKey.Index)
	if pkField.IsZero() {
		return ErrMissingPrimaryKey
	}
	args = append(args, pkField.Interface())

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		table, strings.Join(sets, ", "), schema.PrimaryKey.Column, len(args),
	)

	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// Delete removes the row matching m's primary key.
func Delete[T Tabler](m *T, execs ...Executor) error {
	return DeleteCtx(context.Background(), m, execs...)
}

// DeleteCtx removes the row matching m's primary key, cancelling with ctx.
func DeleteCtx[T Tabler](ctx context.Context, m *T, execs ...Executor) error {
	exec, err := resolveExecutor(execs)
	if err != nil {
		return err
	}

	schema := schemaFor[T]()
	if !schema.HasPrimaryKey {
		return fmt.Errorf("orm: %T has no primary key field", *m)
	}

	v := reflect.ValueOf(m).Elem()
	pkField := v.FieldByIndex(schema.PrimaryKey.Index)
	if pkField.IsZero() {
		return ErrMissingPrimaryKey
	}

	table := (*m).TableName()
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, schema.PrimaryKey.Column)

	result, err := exec.ExecContext(ctx, query, pkField.Interface())
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// setTimeIfZero sets the named time.Time column on v to value if the field
// exists and is currently the zero time.
func setTimeIfZero(v reflect.Value, schema *typeSchema, column string, value time.Time) {
	f, ok := schema.ByColumn[column]
	if !ok {
		return
	}
	field := v.FieldByIndex(f.Index)
	t, ok := field.Interface().(time.Time)
	if !ok {
		return
	}
	if t.IsZero() {
		field.Set(reflect.ValueOf(value))
	}
}

// setTime unconditionally sets the named time.Time column on v to value, if
// the field exists.
func setTime(v reflect.Value, schema *typeSchema, column string, value time.Time) {
	f, ok := schema.ByColumn[column]
	if !ok {
		return
	}
	field := v.FieldByIndex(f.Index)
	if _, ok := field.Interface().(time.Time); !ok {
		return
	}
	field.Set(reflect.ValueOf(value))
}
