package migrations

import (
	"database/sql"

	"github.com/gochin/framework/pkg/orm"
)

func init() {
	orm.Register(orm.Migration{
		Version: "000005",
		Name:    "create_auth_tokens_table",
		Up: func(tx *sql.Tx) error {
			return orm.CreateTable(tx, "auth_tokens", func(t *orm.Blueprint) {
				t.ID()
				// Deleting a user revokes every token it owns, in the same
				// transaction, without any application code running.
				t.ForeignID("user_id").References("users", "id").OnDelete(orm.Cascade)
				t.String("name").Default("'api'")
				// Only the digest is stored, never the token itself, so a
				// database dump cannot be replayed against the API. The
				// unique index is also the authentication lookup path.
				t.String("token_digest").Unique()
				t.String("abilities").Default("'*'")
				t.Timestamp("expires_at").Nullable()
				t.Timestamp("revoked_at").Nullable()
				t.Timestamp("last_used_at").Nullable()
				t.Timestamps()
			})
		},
		Down: func(tx *sql.Tx) error {
			return orm.DropTable(tx, "auth_tokens")
		},
	})
}
