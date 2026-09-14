package orm

import "time"

// Model is the base type every ORM-backed struct should embed. It provides
// the primary key and timestamp columns most tables share.
type Model struct {
	ID        int64     `db:"id,primary_key" json:"id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Tabler is implemented by every model to declare its backing table name.
// Implementations must use a value receiver (func (User) TableName() string)
// so that the model type itself, not a pointer to it, satisfies this
// interface - that's what allows the generic CRUD functions to be written
// as Find[T Tabler] instead of requiring a pointer-type-parameter trick.
type Tabler interface {
	TableName() string
}
