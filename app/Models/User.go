package models

import (
	"github.com/gochin/framework/pkg/orm"
)

// User represents a row in the "users" table.
type User struct {
	orm.Model

	Name  string `db:"name" json:"name"`
	Email string `db:"email" json:"email"`

	// json:"-" is what keeps the hash out of every endpoint that already
	// serializes a user. Never remove it.
	PasswordHash string `db:"password_hash" json:"-"`

	// Relation, not a column — hence db:"-". Populated on demand by
	// orm.LoadHasMany, never automatically.
	Posts []*Post `db:"-" json:"posts,omitempty"`
}

// TableName returns the database table name for User.
func (User) TableName() string {
	return "users"
}

var _ orm.Tabler = User{}
