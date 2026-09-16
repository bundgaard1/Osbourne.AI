package database

import (
	"errors"
	"fmt"

	c "github.com/ostafen/clover/v2"
	"github.com/ostafen/clover/v2/document"
	"osbourne.local/course-content-service/internal/domain"
)

func NewCloverDB(dbDir string) (*c.DB, error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func SeedCloverData(db *c.DB) error {
	err := db.CreateCollection("modules")
	if err != nil {
		if errors.Is(err, c.ErrCollectionExist) {
			return nil
		}
		return fmt.Errorf("Could not create collection: %w", err)
	}

	module := domain.Module{
		ID:       "1",
		CourseID: "1",
		Title:    "Module 1",
		Text:     "This is the first module.",
	}

	doc := document.NewDocumentOf(module)

	if doc == nil {
		return fmt.Errorf("Could not create document")
	}

	if _, err := db.InsertOne("modules", doc); err != nil {
		return fmt.Errorf("Could not insert document: %w", err)
	}

	return nil
}
