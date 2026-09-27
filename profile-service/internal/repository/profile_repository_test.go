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

func TestGORMProfileRepository_Update_ClearsFields(t *testing.T) {
	// Arrange: a profile with every optional field populated. Clearing one of
	// them is the case a naive Update gets wrong - GORM skips zero values by
	// default, so writing "" would look like it succeeded while changing
	// nothing in the database.
	db := setupTestDB(t)
	repo := repository.NewGORMProfileRepository(db)
	ctx := context.Background()

	student := domain.UserProfile{
		ID:           "student-clear",
		Name:         "Ada",
		Phone:        "+31 6 1111 1111",
		Bio:          "was a bio",
		StudyProgram: "CS",
	}
	if err := repo.Create(ctx, &student); err != nil {
		t.Fatalf("Could not insert test data: %v", err)
	}

	// Act: blank the bio and rename, leaving phone and study program alone.
	loaded, err := repo.GetByID(ctx, student.ID)
	if err != nil {
		t.Fatalf("GetByID before update: %v", err)
	}
	loaded.Bio = ""
	loaded.Name = "Ada Lovelace"
	if err := repo.Update(ctx, loaded); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Assert
	got, err := repo.GetByID(ctx, student.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Bio != "" {
		t.Errorf("Bio = %q, want it cleared to \"\"", got.Bio)
	}
	if got.Name != "Ada Lovelace" {
		t.Errorf("Name = %q, want %q", got.Name, "Ada Lovelace")
	}
	// Untouched columns must survive: Update replaces the row's editable
	// fields, not every field.
	if got.Phone != student.Phone {
		t.Errorf("Phone = %q, want it unchanged at %q", got.Phone, student.Phone)
	}
	if got.StudyProgram != student.StudyProgram {
		t.Errorf("StudyProgram = %q, want it unchanged at %q", got.StudyProgram, student.StudyProgram)
	}
}

func TestGORMProfileRepository_Update_UnknownIDIsNotAnError(t *testing.T) {
	// Arrange
	db := setupTestDB(t)
	repo := repository.NewGORMProfileRepository(db)
	ctx := context.Background()

	// Act: update a row that does not exist. GORM's Updates reports no error
	// for a zero-row match, so the service layer - not this one - is what
	// guarantees a 404, by reading first.
	err := repo.Update(ctx, &domain.UserProfile{ID: "nope", Name: "ghost"})

	// Assert
	if err != nil {
		t.Errorf("Update on a missing row returned %v, want nil", err)
	}
	var count int64
	db.Model(&domain.UserProfile{}).Where("id = ?", "nope").Count(&count)
	if count != 0 {
		t.Errorf("Update inserted a phantom row for a missing id (count = %d)", count)
	}
}
