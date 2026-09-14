package models

import (
	"github.com/gochin/framework/pkg/orm"
)

// Tag represents a row in the "tags" table.
type Tag struct {
	orm.Model

	Name string `db:"name" json:"name"`

	Posts []*Post `db:"-" json:"posts,omitempty"`
}

func (Tag) TableName() string { return "tags" }

var _ orm.Tabler = Tag{}
