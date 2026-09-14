package models

import (
	"github.com/gochin/framework/pkg/orm"
)

// Post represents a row in the "posts" table.
type Post struct {
	orm.Model

	Title     string `db:"title" json:"title"`
	Body      string `db:"body" json:"body"`
	Published bool   `db:"published" json:"published"`
	UserID    int64  `db:"user_id" json:"user_id"`

	// Relations are not columns, so they must be tagged db:"-" or the
	// scanner will look for a matching column and fail.
	Author *User  `db:"-" json:"author,omitempty"`
	Tags   []*Tag `db:"-" json:"tags,omitempty"`
}

func (Post) TableName() string { return "posts" }

var _ orm.Tabler = Post{}
