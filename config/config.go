package config

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func InitDB() (*sql.DB, error) {
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", "data/subscriptions.db")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}