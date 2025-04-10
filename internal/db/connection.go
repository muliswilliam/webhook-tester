package db

import (
	"database/sql"
	"log"
	"os"
)

var DB *sql.DB

func Connect() error {
	var err error
	DB, err = sql.Open("sqlite3", os.Getenv("DATABASE_URL"))

	if err != nil {
		log.Printf("error connecting to database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Printf("error pinging database: %v", err)
	}

	return err
}
