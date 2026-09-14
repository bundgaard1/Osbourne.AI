package database

import (
	"fmt"

	// Use the official GORM dialect adapter from glebarez:

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"osbourne.local/notification-service/internal/domain"
)

func NewGORMDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("could not connect to SQLite via GORM: %w", err)
	}

	// Enable WAL via a direct PRAGMA instead of a query string
	db.Exec("PRAGMA journal_mode=WAL;")

	err = db.AutoMigrate(&domain.Notification{})
	if err != nil {
		return nil, fmt.Errorf("error during AutoMigrate: %w", err)
	}

	return db, nil
}
