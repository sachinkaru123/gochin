package migrations

import (
	"database/sql"

	"github.com/gochin/framework/pkg/orm"
)

func init() {
	orm.Register(orm.Migration{
		Version: "000002",
		Name:    "create_posts_table",
		Up: func(tx *sql.Tx) error {
			// The foreign key is what makes Postgres itself enforce the
			// relationship: a post can never reference a missing user, and
			// deleting a user removes their posts in the same transaction.
			return orm.CreateTable(tx, "posts", func(t *orm.Blueprint) {
				t.ID()
				t.String("title")
				t.Text("body").Nullable()
				t.Boolean("published").Default("false")
				t.ForeignID("user_id").
					References("users", "id").
					OnDelete(orm.Cascade)
				t.Timestamps()
			})
		},
		Down: func(tx *sql.Tx) error {
			return orm.DropTable(tx, "posts")
		},
	})
}
