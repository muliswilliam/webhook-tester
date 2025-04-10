package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"log"
	"os"
)

func RunMigrations() error {
	db, err := sql.Open("sqlite3", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Printf("failed to connect to db: %v", err)
		return err
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}(db)

	if err = goose.Up(db, os.Getenv("GOOSE_MIGRATION_DIR")); err != nil {
		log.Printf("failed to run migrations: %v", err)
		return err
	}

	log.Printf("✅ Migration run successfully")
	return nil
}
