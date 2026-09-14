package migrations

import (
	"database/sql"

	"github.com/gochin/framework/pkg/orm"
)

func init() {
	orm.Register(orm.Migration{
		Version: "000004",
		Name:    "add_auth_to_users",
		Up: func(tx *sql.Tx) error {
			// The table already has rows, so the column needs a default.
			// '' can never verify against an argon2id hash, so pre-existing
			// accounts are login-disabled until a password is set: it fails
			// closed rather than open.
			if err := orm.AlterTable(tx, "users", func(t *orm.Blueprint) {
				t.String("password_hash").Default("''")
			}); err != nil {
				return err
			}

			// Until now "Alice@x.com" and "alice@x.com" were two separate
			// accounts. With passwords in play that becomes a registration
			// confusion and account-takeover vector, so fold case now and
			// enforce it with a functional unique index.
			if _, err := tx.Exec(`UPDATE users SET email = lower(btrim(email))`); err != nil {
				return err
			}
			_, err := tx.Exec(
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email))`)
			return err
		},
		Down: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_users_email_lower`); err != nil {
				return err
			}
			return orm.DropColumn(tx, "users", "password_hash")
		},
	})
}
