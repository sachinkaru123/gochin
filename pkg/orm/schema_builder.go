package orm

import (
	"database/sql"
	"fmt"
	"strings"
)

// RefAction is what Postgres does to a referencing row when the referenced
// row changes.
type RefAction string

const (
	Cascade  RefAction = "CASCADE"
	Restrict RefAction = "RESTRICT"
	SetNull  RefAction = "SET NULL"
	NoAction RefAction = "NO ACTION"
)

// Column is a fluent column definition inside a Blueprint.
type Column struct {
	name       string
	typ        string
	nullable   bool
	unique     bool
	primaryKey bool
	defaultVal string
	wantIndex  bool
	noIndex    bool

	refTable  string
	refColumn string
	onDelete  RefAction
	onUpdate  RefAction
}

// Nullable allows NULL in this column. Columns are NOT NULL by default.
func (c *Column) Nullable() *Column {
	c.nullable = true
	return c
}

// Unique adds a UNIQUE constraint.
func (c *Column) Unique() *Column {
	c.unique = true
	return c
}

// Default sets a raw SQL default expression, e.g. "now()" or "0".
func (c *Column) Default(expr string) *Column {
	c.defaultVal = expr
	return c
}

// Index creates a secondary index on this column.
func (c *Column) Index() *Column {
	c.wantIndex = true
	return c
}

// NoIndex opts a foreign key out of its automatic index.
func (c *Column) NoIndex() *Column {
	c.noIndex = true
	return c
}

// References points this column at another table's column, which is what
// makes Postgres enforce the relationship.
func (c *Column) References(table, column string) *Column {
	c.refTable = table
	c.refColumn = column
	return c
}

// OnDelete sets the referential action for deletes of the parent row.
func (c *Column) OnDelete(action RefAction) *Column {
	c.onDelete = action
	return c
}

// OnUpdate sets the referential action for updates of the parent key.
func (c *Column) OnUpdate(action RefAction) *Column {
	c.onUpdate = action
	return c
}

func (c *Column) definition() string {
	var b strings.Builder
	b.WriteString(c.name)
	b.WriteString(" ")
	b.WriteString(c.typ)

	if c.primaryKey {
		b.WriteString(" PRIMARY KEY")
		return b.String()
	}
	if !c.nullable {
		b.WriteString(" NOT NULL")
	}
	if c.unique {
		b.WriteString(" UNIQUE")
	}
	if c.defaultVal != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(c.defaultVal)
	}
	if c.refTable != "" {
		fmt.Fprintf(&b, " REFERENCES %s(%s)", c.refTable, c.refColumn)
		if c.onDelete != "" {
			fmt.Fprintf(&b, " ON DELETE %s", c.onDelete)
		}
		if c.onUpdate != "" {
			fmt.Fprintf(&b, " ON UPDATE %s", c.onUpdate)
		}
	}
	return b.String()
}

// needsIndex reports whether this column should get its own index.
//
// Foreign keys are indexed by default: Postgres indexes the referenced
// primary key, never the referencing column, so without one every cascade
// delete and every join on it degrades into a sequential scan.
func (c *Column) needsIndex() bool {
	if c.noIndex || c.primaryKey || c.unique {
		return false
	}
	return c.wantIndex || c.refTable != ""
}

type indexDef struct {
	name    string
	columns []string
	unique  bool
}

// Blueprint collects the columns, constraints and indexes of one table.
type Blueprint struct {
	table   string
	columns []*Column
	indexes []indexDef
	raw     []string
}

func (b *Blueprint) add(name, typ string) *Column {
	c := &Column{name: name, typ: typ}
	b.columns = append(b.columns, c)
	return c
}

// ID adds the conventional auto-incrementing primary key.
func (b *Blueprint) ID() *Column { return b.add("id", "BIGSERIAL").primary() }

func (c *Column) primary() *Column {
	c.primaryKey = true
	return c
}

func (b *Blueprint) String(name string) *Column  { return b.add(name, "TEXT") }
func (b *Blueprint) Text(name string) *Column    { return b.add(name, "TEXT") }
func (b *Blueprint) Integer(name string) *Column { return b.add(name, "INTEGER") }
func (b *Blueprint) BigInteger(name string) *Column {
	return b.add(name, "BIGINT")
}
func (b *Blueprint) Boolean(name string) *Column   { return b.add(name, "BOOLEAN") }
func (b *Blueprint) Timestamp(name string) *Column { return b.add(name, "TIMESTAMPTZ") }
func (b *Blueprint) Date(name string) *Column      { return b.add(name, "DATE") }
func (b *Blueprint) JSONB(name string) *Column     { return b.add(name, "JSONB") }
func (b *Blueprint) UUID(name string) *Column      { return b.add(name, "UUID") }

// Varchar adds a length-limited string column.
func (b *Blueprint) Varchar(name string, length int) *Column {
	return b.add(name, fmt.Sprintf("VARCHAR(%d)", length))
}

// Decimal adds an exact numeric column, for money and other values that must
// not be stored as floating point.
func (b *Blueprint) Decimal(name string, precision, scale int) *Column {
	return b.add(name, fmt.Sprintf("NUMERIC(%d,%d)", precision, scale))
}

// Timestamps adds the created_at/updated_at pair orm.Model expects.
func (b *Blueprint) Timestamps() {
	b.Timestamp("created_at").Default("now()")
	b.Timestamp("updated_at").Default("now()")
}

// ForeignID adds a foreign key column. Pair it with References:
//
//	t.ForeignID("user_id").References("users", "id").OnDelete(orm.Cascade)
func (b *Blueprint) ForeignID(name string) *Column {
	return b.add(name, "BIGINT")
}

// Index adds a composite index across columns.
func (b *Blueprint) Index(columns ...string) {
	b.indexes = append(b.indexes, indexDef{
		name:    indexName(b.table, columns),
		columns: columns,
	})
}

// UniqueIndex adds a composite unique index, e.g. to make a pivot table's
// pair unique.
func (b *Blueprint) UniqueIndex(columns ...string) {
	b.indexes = append(b.indexes, indexDef{
		name:    indexName(b.table, columns),
		columns: columns,
		unique:  true,
	})
}

// Raw appends an arbitrary SQL statement, the escape hatch for anything the
// builder does not cover.
func (b *Blueprint) Raw(sql string) {
	b.raw = append(b.raw, sql)
}

func indexName(table string, columns []string) string {
	return "idx_" + table + "_" + strings.Join(columns, "_")
}

// createStatements renders CREATE TABLE plus every index it implies.
func (b *Blueprint) createStatements() []string {
	defs := make([]string, 0, len(b.columns))
	for _, c := range b.columns {
		defs = append(defs, "\t"+c.definition())
	}

	stmts := []string{fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (\n%s\n)",
		b.table, strings.Join(defs, ",\n"),
	)}

	return append(stmts, b.indexStatements()...)
}

// alterStatements renders ADD COLUMN for each column plus its indexes.
func (b *Blueprint) alterStatements() []string {
	stmts := make([]string, 0, len(b.columns))
	for _, c := range b.columns {
		stmts = append(stmts, fmt.Sprintf(
			"ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s", b.table, c.definition(),
		))
	}
	return append(stmts, b.indexStatements()...)
}

func (b *Blueprint) indexStatements() []string {
	var stmts []string

	for _, c := range b.columns {
		if c.needsIndex() {
			stmts = append(stmts, fmt.Sprintf(
				"CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
				indexName(b.table, []string{c.name}), b.table, c.name,
			))
		}
	}

	for _, idx := range b.indexes {
		unique := ""
		if idx.unique {
			unique = "UNIQUE "
		}
		stmts = append(stmts, fmt.Sprintf(
			"CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
			unique, idx.name, b.table, strings.Join(idx.columns, ", "),
		))
	}

	return append(stmts, b.raw...)
}

// CreateTable builds and executes a CREATE TABLE inside a migration.
func CreateTable(tx *sql.Tx, table string, fn func(*Blueprint)) error {
	b := &Blueprint{table: table}
	fn(b)

	for _, stmt := range b.createStatements() {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("orm: %s: %w", firstLine(stmt), err)
		}
	}
	return nil
}

// AlterTable adds columns and indexes to an existing table.
func AlterTable(tx *sql.Tx, table string, fn func(*Blueprint)) error {
	b := &Blueprint{table: table}
	fn(b)

	for _, stmt := range b.alterStatements() {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("orm: %s: %w", firstLine(stmt), err)
		}
	}
	return nil
}

// DropTable removes a table if it exists.
func DropTable(tx *sql.Tx, table string) error {
	_, err := tx.Exec("DROP TABLE IF EXISTS " + table)
	return err
}

// DropColumn removes a column if it exists.
func DropColumn(tx *sql.Tx, table, column string) error {
	_, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", table, column))
	return err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
