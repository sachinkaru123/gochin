package orm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// Relationship loading in Gochin is always batched.
//
// The naive approach — looping over parents and querying each one's children —
// is the N+1 problem: 25 parents cost 26 round trips, which dominates request
// latency far more than anything else in the stack. Every loader here issues
// exactly ONE query regardless of how many parents you pass.
//
// Relation fields must be tagged db:"-" so the scanner does not treat them as
// columns:
//
//	type User struct {
//	    orm.Model
//	    Name  string  `db:"name" json:"name"`
//	    Posts []*Post `db:"-" json:"posts,omitempty"`
//	}

// LoadHasMany attaches the children of every parent in one query.
//
//	orm.LoadHasMany(ctx, users, "user_id",
//	    func(u *models.User) int64 { return u.ID },
//	    func(u *models.User, posts []*models.Post) { u.Posts = posts })
func LoadHasMany[P any, C Tabler](
	ctx context.Context,
	parents []*P,
	foreignKey string,
	parentID func(*P) int64,
	assign func(*P, []*C),
	execs ...Executor,
) error {
	if len(parents) == 0 {
		return nil
	}

	schema := schemaFor[C]()
	if !schema.IsValidColumn(foreignKey) {
		var zero C
		return fmt.Errorf("orm: %T has no column %q", zero, foreignKey)
	}

	ids, index := indexParents(parents, parentID)
	if len(ids) == 0 {
		return nil
	}

	children, err := Query[C](execs...).WithContext(ctx).WhereIn(foreignKey, ids).Get()
	if err != nil {
		return err
	}

	grouped := make(map[int64][]*C, len(ids))
	for _, child := range children {
		key, ok := int64Field(reflect.ValueOf(child).Elem(), schema, foreignKey)
		if !ok {
			continue
		}
		grouped[key] = append(grouped[key], child)
	}

	for key, positions := range index {
		matches := grouped[key]
		for _, i := range positions {
			assign(parents[i], matches)
		}
	}
	return nil
}

// LoadHasOne attaches at most one child per parent, in one query.
func LoadHasOne[P any, C Tabler](
	ctx context.Context,
	parents []*P,
	foreignKey string,
	parentID func(*P) int64,
	assign func(*P, *C),
	execs ...Executor,
) error {
	return LoadHasMany(ctx, parents, foreignKey, parentID,
		func(parent *P, children []*C) {
			if len(children) > 0 {
				assign(parent, children[0])
			}
		}, execs...)
}

// LoadBelongsTo attaches each child's parent in one query.
//
//	orm.LoadBelongsTo(ctx, posts,
//	    func(p *models.Post) int64 { return p.UserID },
//	    func(p *models.Post, u *models.User) { p.Author = u })
func LoadBelongsTo[C any, P Tabler](
	ctx context.Context,
	children []*C,
	parentID func(*C) int64,
	assign func(*C, *P),
	execs ...Executor,
) error {
	if len(children) == 0 {
		return nil
	}

	schema := schemaFor[P]()
	if !schema.HasPrimaryKey {
		var zero P
		return fmt.Errorf("orm: %T has no primary key field", zero)
	}

	ids, index := indexParents(children, parentID)
	if len(ids) == 0 {
		return nil
	}

	parents, err := Query[P](execs...).WithContext(ctx).
		WhereIn(schema.PrimaryKey.Column, ids).Get()
	if err != nil {
		return err
	}

	byID := make(map[int64]*P, len(parents))
	for _, parent := range parents {
		key, ok := int64Field(reflect.ValueOf(parent).Elem(), schema, schema.PrimaryKey.Column)
		if !ok {
			continue
		}
		byID[key] = parent
	}

	for key, positions := range index {
		parent, ok := byID[key]
		if !ok {
			continue
		}
		for _, i := range positions {
			assign(children[i], parent)
		}
	}
	return nil
}

// LoadBelongsToMany attaches many-to-many relations through a pivot table, in
// one query.
//
//	orm.LoadBelongsToMany(ctx, posts, "post_tag", "post_id", "tag_id",
//	    func(p *models.Post) int64 { return p.ID },
//	    func(p *models.Post, tags []*models.Tag) { p.Tags = tags })
func LoadBelongsToMany[P any, C Tabler](
	ctx context.Context,
	parents []*P,
	pivotTable, parentForeignKey, childForeignKey string,
	parentID func(*P) int64,
	assign func(*P, []*C),
	execs ...Executor,
) error {
	if len(parents) == 0 {
		return nil
	}
	for _, ident := range []string{pivotTable, parentForeignKey, childForeignKey} {
		if err := validateIdentifier(ident); err != nil {
			return err
		}
	}

	exec, err := resolveExecutor(execs)
	if err != nil {
		return err
	}

	ids, index := indexParents(parents, parentID)
	if len(ids) == 0 {
		return nil
	}

	var zero C
	schema := schemaFor[C]()
	childTable := zero.TableName()

	// Select the child's columns plus the pivot's parent key, so one pass
	// groups every child under the right parent.
	cols := schema.columns()
	qualified := make([]string, len(cols))
	for i, c := range cols {
		qualified[i] = childTable + "." + c
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT %s.%s, %s FROM %s JOIN %s ON %s.%s = %s.%s WHERE %s.%s IN (%s)",
		pivotTable, parentForeignKey, strings.Join(qualified, ", "),
		childTable,
		pivotTable, pivotTable, childForeignKey, childTable, schema.PrimaryKey.Column,
		pivotTable, parentForeignKey, strings.Join(placeholders, ", "),
	)

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	grouped := make(map[int64][]*C, len(ids))
	for rows.Next() {
		var parentKey int64
		child := new(C)

		dest, err := scanTargets(child, schema, cols)
		if err != nil {
			return err
		}
		if err := rows.Scan(append([]any{&parentKey}, dest...)...); err != nil {
			return err
		}
		grouped[parentKey] = append(grouped[parentKey], child)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for key, positions := range index {
		matches := grouped[key]
		for _, i := range positions {
			assign(parents[i], matches)
		}
	}
	return nil
}

// indexParents collects the distinct keys to query for, and remembers which
// positions each key maps back to so duplicates are handled correctly.
func indexParents[T any](items []*T, key func(*T) int64) ([]any, map[int64][]int) {
	index := make(map[int64][]int, len(items))
	ids := make([]any, 0, len(items))

	for i, item := range items {
		if item == nil {
			continue
		}
		k := key(item)
		if k == 0 {
			continue
		}
		if _, seen := index[k]; !seen {
			ids = append(ids, k)
		}
		index[k] = append(index[k], i)
	}
	return ids, index
}

// int64Field reads a column's value as an int64, tolerating the various
// integer widths and nullable wrappers a schema may use.
func int64Field(v reflect.Value, schema *typeSchema, column string) (int64, bool) {
	f, ok := schema.ByColumn[column]
	if !ok {
		return 0, false
	}

	field := v.FieldByIndex(f.Index)
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return field.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(field.Uint()), true
	case reflect.Ptr:
		if field.IsNil() {
			return 0, false
		}
		return int64Field(field.Elem(), schema, column)
	}

	// database/sql nullable types expose the value behind a Valid flag.
	if field.Kind() == reflect.Struct {
		valid := field.FieldByName("Valid")
		inner := field.FieldByName("Int64")
		if valid.IsValid() && inner.IsValid() && valid.Kind() == reflect.Bool {
			if !valid.Bool() {
				return 0, false
			}
			return inner.Int(), true
		}
	}

	return 0, false
}
