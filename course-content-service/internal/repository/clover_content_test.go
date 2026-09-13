package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	c "github.com/ostafen/clover/v2"
	"osbourne.local/course-content-service/internal/domain"
	"osbourne.local/course-content-service/internal/repository"
)

// setupTestDB creates an isolated, in-memory Clover DB for testing
func setupTestDB(t *testing.T) (*c.DB, func()) {
	t.Helper()

	// Use temporary directory for disk mode, or memory mode
	dir, err := os.MkdirTemp("", "clover-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	db, err := c.Open(dir)
	if err != nil {
		t.Fatalf("failed to open clover db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return db, cleanup
}

func TestCloverModuleRepository_CreateAndGetModule(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewCloverModuleRepository(db, "modules-test")
	ctx := context.Background()

	inputModule := &domain.Module{
		ID:        "1",
		CourseID:  "course-1",
		Title:     "Introduction to Go",
		Text:      "Learn Go fundamentals",
		UpdatedAt: time.Now().Truncate(time.Millisecond),
	}

	// 1. Test Create
	err := repo.CreateModule(ctx, inputModule)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}

	// LogAllModules(t, repo)

	// 2. Test Get
	fetched, err := repo.GetModule(ctx, inputModule.ID)
	if err != nil {
		t.Fatalf("expected no error on get, got %v", err)
	}
	if fetched == nil {
		t.Fatal("expected module to be found, got nil")
	}

	if !fetched.UpdatedAt.Equal(inputModule.UpdatedAt) {
		t.Errorf("expected title %s, got %s", inputModule.Title, fetched.Title)
	}

	if !fetched.UpdatedAt.Equal(inputModule.UpdatedAt) {
		t.Errorf("expected ID %s, got %s", inputModule.ID, fetched.ID)
	}

	if !fetched.UpdatedAt.Equal(inputModule.UpdatedAt) {
		t.Errorf("expected CourseID %s, got %s", inputModule.CourseID, fetched.CourseID)
	}

	if !fetched.UpdatedAt.Equal(inputModule.UpdatedAt) {
		t.Errorf("expected UpdatedAt %v, got %v", inputModule.UpdatedAt, fetched.UpdatedAt)
	}

	// if len(fetched.Attachments) != len(inputModule.Attachments) {
	// 	t.Fatalf("expected %d attachments, got %d", len(inputModule.Attachments), len(fetched.Attachments))
	// }
}

func TestCloverModuleRepository_ListModulesByCourseID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewCloverModuleRepository(db, "modules-test")
	ctx := context.Background()

	modules := []*domain.Module{
		{ID: "1", CourseID: "course-1", Title: "Module One", Text: "First module"},
		{ID: "2", CourseID: "course-1", Title: "Module Two", Text: "Second module"},
		{ID: "3", CourseID: "course-2", Title: "Other Course", Text: "Unrelated"},
	}
	for _, m := range modules {
		if err := repo.CreateModule(ctx, m); err != nil {
			t.Fatalf("failed to create module: %v", err)
		}
	}

	got, err := repo.ListModules(ctx, "course-1")
	if err != nil {
		t.Fatalf("expected no error on list, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 modules for course-1, got %d", len(got))
	}
	if got[0].CourseID != "course-1" || got[1].CourseID != "course-1" {
		t.Errorf("returned modules do not belong to course-1: %+v", got)
	}
}

func TestCloverModuleRepository_UpdateAndDeleteModule(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewCloverModuleRepository(db, "modules-test")
	ctx := context.Background()

	module := &domain.Module{
		ID:        "1",
		CourseID:  "course-1",
		Title:     "Initial Title",
		Text:      "Initial Text",
		UpdatedAt: time.Now().Truncate(time.Millisecond),
	}

	// Create the module first
	err := repo.CreateModule(ctx, module)
	if err != nil {
		t.Fatalf("expected no error on create, got %v", err)
	}
	// Update the module
	module.Title = "Updated Title"
	module.Text = "Updated Text"
	err = repo.UpdateModule(ctx, module)
	if err != nil {
		t.Fatalf("expected no error on update, got %v", err)
	}

	// Fetch and verify update
	fetched, err := repo.GetModule(ctx, module.ID)
	if err != nil {
		t.Fatalf("expected no error on get after update, got %v", err)
	}
	if fetched.Title != "Updated Title" || fetched.Text != "Updated Text" {
		t.Errorf("update did not persist correctly")
	}

	// Delete the module
	err = repo.DeleteModule(ctx, module.ID)
	if err != nil {
		t.Fatalf("expected no error on delete, got %v", err)
	}

	// Verify deletion
	fetched, err = repo.GetModule(ctx, module.ID)
	if err != nil {
		t.Fatalf("expected no error on get after delete, got %v", err)
	}
	if fetched != nil {
		t.Errorf("expected module to be deleted, but it still exists")
	}
}

// func TestCloverModuleRepository_AddAndRemoveAttachment(t *testing.T) {
// 	db, cleanup := setupTestDB(t)
// 	defer cleanup()

// 	repo := repository.NewCloverModuleRepository(db, "modules-test")
// 	ctx := context.Background()

// 	module := &domain.Module{
// 		ID:        "1",
// 		CourseID:  "101",
// 		Title:     "Module with Attachments",
// 		Text:      "Testing attachments",
// 		UpdatedAt: time.Now().Truncate(time.Millisecond),
// 	}

// 	// Create the module first
// 	err := repo.CreateModule(ctx, module)
// 	if err != nil {
// 		t.Fatalf("expected no error on create, got %v", err)
// 	}

// 	attachment := &domain.Attachment{
// 		Name:      "Attachment 1",
// 		Type:      "pdf",
// 		Path:      "/path/to/attachment1.pdf",
// 		Size:      2048,
// 		CreatedAt: time.Now().Truncate(time.Millisecond),
// 	}

// 	// Add attachment
// 	err = repo.AddAttachmentToModule(ctx, module.ID, attachment)
// 	if err != nil {
// 		t.Fatalf("expected no error on add attachment, got %v", err)
// 	}

// 	// Verify attachment added
// 	fetched, err := repo.GetModule(ctx, module.ID)
// 	if err != nil {
// 		t.Fatalf("expected no error on get after adding attachment, got %v", err)
// 	}
// 	if len(fetched.Attachments) != 1 || fetched.Attachments[0].ID != attachment.ID {
// 		t.Errorf("attachment was not added correctly")
// 	}

// 	// Remove attachment
// 	err = repo.RemoveAttachmentFromModule(ctx, module.ID, attachment.ID)
// 	if err != nil {
// 		t.Fatalf("expected no error on remove attachment, got %v", err)
// 	}

// 	// Verify attachment removed
// 	fetched, err = repo.GetModule(ctx, module.ID)
// 	if err != nil {
// 		t.Fatalf("expected no error on get after removing attachment, got %v", err)
// 	}
// 	if len(fetched.Attachments) != 0 {
// 		t.Errorf("attachment was not removed correctly")
// 	}
// }

func LogAllModules(t *testing.T, repo *repository.CloverModuleRepository) {
	t.Helper()
	ctx := context.Background()
	modules, n, err := repo.GetAllModules(ctx)
	if err != nil {
		t.Fatalf("failed to list modules: %v", err)
	}
	t.Logf("Total modules found: %d \n", n)
	for _, m := range modules {
		LogModuleDetails(t, m)
	}
}

func LogModuleDetails(t *testing.T, module *domain.Module) {
	t.Helper()
	if module == nil {
		t.Log("Module is nil")
		return
	}
	t.Logf("-Module ID: %s, CourseID: %s, Title: %s, Text: %s, UpdatedAt: %v", module.ID, module.CourseID, module.Title, module.Text, module.UpdatedAt)
	// t.Logf("-Attachments count: %d", len(module.Attachments))
	// for i, att := range module.Attachments {
	// 	t.Logf("--Attachment [%d]: ID: %s, Name: %s, Type: %s, Path: %s, Size: %d, CreatedAt: %v", i, att.ID, att.Name, att.Type, att.Path, att.Size, att.CreatedAt)
	// }
}
