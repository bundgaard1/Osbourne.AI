package database

import (
	"fmt"

	// Use the official GORM dialect adapter from glebarez:

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	authcommon "osbourne.local/auth-common"
	"osbourne.local/notification-service/internal/domain"
)

func NewGORMDB(dbPath string) (*gorm.DB, error) {
	// SQL chatter is noise in production; surface real errors only.
	gormLogger := authcommon.NewGormLogger()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
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
