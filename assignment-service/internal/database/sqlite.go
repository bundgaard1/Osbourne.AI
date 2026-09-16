package database

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"osbourne.local/common"
	"osbourne.local/assignment-service/internal/domain"
	"osbourne.local/assignment-service/internal/seed"
)

func NewGORMDB(dbPath string) (*gorm.DB, error) {
	// SQL chatter is noise in production; surface real errors only.
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: common.NewGormLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("Could not initialize GORM: %w", err)
	}

	// Enable WAL via a direct PRAGMA instead of a query string
	db.Exec("PRAGMA journal_mode=WAL;")

	for _, model := range []any{
		&domain.Assignment{},
		&domain.Submission{},
	} {
		if err := db.AutoMigrate(model); err != nil {
			return nil, fmt.Errorf("Could not migrate database: %w", err)
		}
	}

	return db, nil
}

func SeedGORMData(db *gorm.DB) error {
	var count int64
	if err := db.Model(&domain.Assignment{}).Count(&count).Error; err != nil {
		return fmt.Errorf("Could not count assignments: %w", err)
	}

	if count > 0 {
		return nil
	}

	assignments := []domain.Assignment{
		{ID: "1", CourseID: "1", Title: "Assignment 1", Description: "This is the first assignment.", DueDate: time.Now().AddDate(0, 0, 7)},
		{ID: "2", CourseID: "2", Title: "Assignment 2", Description: "This is the second assignment.", DueDate: time.Now().AddDate(0, 0, 7)},
		{ID: "3", CourseID: "2", Title: "Assignment 3", Description: "This is the third assignment.", DueDate: time.Now().AddDate(0, 0, 14)},
	}

	if err := db.Create(&assignments).Error; err != nil {
		return fmt.Errorf("Could not seed assignments: %w", err)
	}

	// Submissions come from the seed package so their flat UUID file paths
	// always match the physical files written by seed.SeedFiles.
	submissions := seed.Submissions()
	for i := range submissions {
		submissions[i].SubmittedAt = time.Now()
	}

	if err := db.Create(&submissions).Error; err != nil {
		return fmt.Errorf("Could not seed submissions: %w", err)
	}

	return nil
}
