package database

import (
	"fmt"
	"os"
	"path/filepath"

	// Use the official GORM dialect adapter from glebarez:

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	authcommon "osbourne.local/auth-common"
	"osbourne.local/profile-service/internal/domain"
)

func NewGORMDB(dbPath string) (*gorm.DB, error) {
	// SQL chatter is noise in production; surface real errors only.
	gormLogger := authcommon.NewGormLogger()

	// Ensure folder exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create DB directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("could not connect to SQLite via GORM: %w", err)
	}

	db.Exec("PRAGMA journal_mode=WAL;")

	err = db.AutoMigrate(&domain.UserProfile{})
	if err != nil {
		return nil, fmt.Errorf("error during AutoMigrate: %w", err)
	}

	return db, nil
}