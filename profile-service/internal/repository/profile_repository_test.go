package repository_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"osbourne.local/profile-service/internal/domain"
	"osbourne.local/profile-service/internal/repository"
)

// setupTestDB creates a completely clean in-memory SQLite database for each test
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// "file::memory:?cache=shared" or simply ":memory:" ensures the DB only lives in RAM during the test
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Hide SQL logs during test runs
	})
	if err != nil {
		t.Fatalf("Could not create in-memory database: %v", err)
	}

	// Run auto-migration on the in-memory DB
	err = db.AutoMigrate(&domain.UserProfile{})
	if err != nil {
		t.Fatalf("Error during AutoMigrate in test: %v", err)
	}

	return db
}

func TestGORMProfileRepository_GetByID(t *testing.T) {
	// 1. Arrange: Prepare test data in the in-memory database
	db := setupTestDB(t)
	repo := repository.NewGORMProfileRepository(db)
	ctx := context.Background()

	testStudent := domain.UserProfile{
		ID:   "student-123",
		Name: "student",
	}

	if err := db.Create(&testStudent).Error; err != nil {
		t.Fatalf("Could not insert test data: %v", err)
	}

	// 2. Act: Call the method on the repository
	result, err := repo.GetByID(ctx, "student-123")

	// 3. Assert: Verify the result is as expected
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	if result.ID != testStudent.ID {
		t.Errorf("Expected ID %s, but got %s", testStudent.ID, result.ID)
	}

	if result.Name != testStudent.Name {
		t.Errorf("Expected Name %s, but got %s", testStudent.Name, result.Name)
	}

}

func TestGORMProfileRepository_GetByID_NotFound(t *testing.T) {
	// Arrange
	db := setupTestDB(t)
	repo := repository.NewGORMProfileRepository(db)
	ctx := context.Background()

	// Act: Search for an ID that does not exist
	_, err := repo.GetByID(ctx, "non-existing-id")

	// Assert: Expect an error
	if err == nil {
		t.Error("Expected an error for an invalid ID, but got nil")
	}
}
