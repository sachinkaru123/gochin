package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/gochin/framework/pkg/database"
	"github.com/gochin/framework/pkg/orm"

	"github.com/gochin/framework/app/Models"
)

// This is a throwaway program to exercise the ORM end-to-end against a real
// Postgres database. Run with: go run ./cmd/test-orm
func main() {
	fmt.Println("🔍 Testing Gochin ORM")
	fmt.Println("====================================================")

	// --- Create ---
	user := &models.User{Name: "Ada", Email: "ada@example.com"}
	if err := orm.Create(user); err != nil {
		log.Fatalf("Create failed: %v", err)
	}
	fmt.Printf("✅ Created user ID=%d CreatedAt=%s\n", user.ID, user.CreatedAt)

	// --- Find ---
	found, err := orm.Find[models.User](user.ID)
	if err != nil {
		log.Fatalf("Find failed: %v", err)
	}
	fmt.Printf("✅ Found user: %+v\n", *found)

	// --- Update ---
	found.Name = "Ada Lovelace"
	if err := orm.Update(found); err != nil {
		log.Fatalf("Update failed: %v", err)
	}
	updated, err := orm.Find[models.User](user.ID)
	if err != nil {
		log.Fatalf("Find after update failed: %v", err)
	}
	fmt.Printf("✅ Updated user: Name=%s UpdatedAt=%s\n", updated.Name, updated.UpdatedAt)

	// --- Query with Where ---
	match, err := orm.Query[models.User]().Where("email", "=", "ada@example.com").First()
	if err != nil {
		log.Fatalf("Where query failed: %v", err)
	}
	fmt.Printf("✅ Query().Where() matched user ID=%d\n", match.ID)

	// --- Query with OrderBy + Limit ---
	list, err := orm.Query[models.User]().OrderBy("id", "DESC").Limit(10).Get()
	if err != nil {
		log.Fatalf("OrderBy/Limit query failed: %v", err)
	}
	fmt.Printf("✅ Query().OrderBy().Limit() returned %d row(s)\n", len(list))

	// --- Transaction rollback ---
	countBefore, err := orm.Query[models.User]().Count()
	if err != nil {
		log.Fatalf("Count failed: %v", err)
	}

	txErr := orm.WithTransaction(func(tx *sql.Tx) error {
		if err := orm.Create(&models.User{Name: "Temp", Email: "temp@example.com"}, tx); err != nil {
			return err
		}
		return errors.New("forced rollback")
	})
	if txErr == nil {
		log.Fatalf("expected transaction to fail, but it succeeded")
	}

	countAfter, err := orm.Query[models.User]().Count()
	if err != nil {
		log.Fatalf("Count failed: %v", err)
	}
	if countAfter != countBefore {
		log.Fatalf("❌ transaction rollback failed: count before=%d after=%d", countBefore, countAfter)
	}
	fmt.Printf("✅ WithTransaction rolled back correctly (count stayed at %d)\n", countAfter)

	// --- Delete ---
	if err := orm.Delete(updated); err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	_, err = orm.Find[models.User](user.ID)
	if !errors.Is(err, orm.ErrRecordNotFound) {
		log.Fatalf("expected ErrRecordNotFound after delete, got: %v", err)
	}
	fmt.Println("✅ Deleted user, Find correctly returned ErrRecordNotFound")

	database.CloseConnection()
	fmt.Println("\n🎉 All ORM checks passed!")
}
