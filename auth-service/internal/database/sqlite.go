package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"osbourne.local/auth-service/internal/domain"
)

func NewGORMDB(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create DB directory: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("could not connect to SQLite via GORM: %w", err)
	}

	db.Exec("PRAGMA journal_mode=WAL;")

	err = db.AutoMigrate(&domain.UserAccount{})
	if err != nil {
		return nil, fmt.Errorf("error during AutoMigrate: %w", err)
	}

	return db, nil
}