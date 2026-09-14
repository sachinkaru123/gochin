package migrations

import (
	"database/sql"

	"github.com/gochin/framework/pkg/orm"
)

func init() {
	orm.Register(orm.Migration{
		Version: "000003",
		Name:    "create_tags_tables",
		Up: func(tx *sql.Tx) error {
			if err := orm.CreateTable(tx, "tags", func(t *orm.Blueprint) {
				t.ID()
				t.String("name").Unique()
				t.Timestamps()
			}); err != nil {
				return err
			}

			// Pivot table for the many-to-many between posts and tags. The
			// unique index is what stops the same tag being attached twice.
			return orm.CreateTable(tx, "post_tag", func(t *orm.Blueprint) {
				t.ID()
				t.ForeignID("post_id").References("posts", "id").OnDelete(orm.Cascade)
				t.ForeignID("tag_id").References("tags", "id").OnDelete(orm.Cascade)
				t.UniqueIndex("post_id", "tag_id")
			})
		},
		Down: func(tx *sql.Tx) error {
			if err := orm.DropTable(tx, "post_tag"); err != nil {
				return err
			}
			return orm.DropTable(tx, "tags")
		},
	})
}
